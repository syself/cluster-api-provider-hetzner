/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package scope defines cluster and machine scope as well as a repository for the Hetzner API.
package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// BareMetalHostScopeParams defines the input parameters used to create a new scope.
type BareMetalHostScopeParams struct {
	Client                  client.Client
	Logger                  logr.Logger
	HetznerBareMetalHost    *infrav2.HetznerBareMetalHost
	HetznerBareMetalMachine *infrav1.HetznerBareMetalMachine
	Machine                 *clusterv1.Machine
	HetznerCluster          *infrav1.HetznerCluster
	Cluster                 *clusterv1.Cluster
	RobotClient             robotclient.Client
	SSHClientFactory        sshclient.Factory
	OSSSHSecret             *corev1.Secret
	RescueSSHSecret         *corev1.Secret
	SecretManager           *secretutil.SecretManager
	PreProvisionCommand     string

	// WorkloadClusterClientFactory overrides the default real factory. Intended for tests only.
	WorkloadClusterClientFactory WorkloadClusterClientFactory
}

// NewBareMetalHostScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewBareMetalHostScope(params BareMetalHostScopeParams) (*BareMetalHostScope, error) {
	if params.Client == nil {
		return nil, errors.New("cannot create baremetal host scope without client")
	}
	if params.HetznerBareMetalHost == nil {
		return nil, errors.New("cannot create baremetal host scope without host object")
	}
	if params.HetznerCluster == nil {
		return nil, errors.New("cannot create baremetal host scope without Hetzner cluster")
	}
	if params.Cluster == nil {
		return nil, errors.New("cannot create baremetal host scope without cluster")
	}
	if params.RobotClient == nil {
		return nil, errors.New("cannot create baremetal host scope without robot client")
	}
	if params.SSHClientFactory == nil {
		return nil, errors.New("cannot create baremetal host scope without ssh client factory")
	}
	if params.SecretManager == nil {
		return nil, errors.New("cannot create baremetal host scope without secret manager")
	}

	var emptyLogger logr.Logger
	if params.Logger == emptyLogger {
		return nil, fmt.Errorf("failed to generate new scope from nil Logger")
	}

	return &BareMetalHostScope{
		Logger:                  params.Logger,
		Client:                  params.Client,
		RobotClient:             params.RobotClient,
		SSHClientFactory:        params.SSHClientFactory,
		HetznerCluster:          params.HetznerCluster,
		Cluster:                 params.Cluster,
		HetznerBareMetalHost:    params.HetznerBareMetalHost,
		HetznerBareMetalMachine: params.HetznerBareMetalMachine,
		Machine:                 params.Machine,
		OSSSHSecret:             params.OSSSHSecret,
		RescueSSHSecret:         params.RescueSSHSecret,
		SecretManager:           params.SecretManager,
		PreProvisionCommand:     params.PreProvisionCommand,
		WorkloadClusterClientFactory: func() WorkloadClusterClientFactory {
			if params.WorkloadClusterClientFactory != nil {
				return params.WorkloadClusterClientFactory
			}
			return &realWorkloadClusterClientFactory{
				logger:         params.Logger,
				client:         params.Client,
				cluster:        params.Cluster,
				hetznerCluster: params.HetznerCluster,
			}
		}(),
	}, nil
}

// BareMetalHostScope defines the basic context for an actuator to operate upon.
type BareMetalHostScope struct {
	logr.Logger
	Client               client.Client
	SecretManager        *secretutil.SecretManager
	RobotClient          robotclient.Client
	SSHClientFactory     sshclient.Factory
	HetznerBareMetalHost *infrav2.HetznerBareMetalHost
	// HetznerBareMetalMachine is the machine that consumes the host. It is nil when the host has no
	// consumer or the machine no longer exists.
	HetznerBareMetalMachine *infrav1.HetznerBareMetalMachine
	// Machine is the CAPI machine that owns the HetznerBareMetalMachine. It is nil when there is no
	// consuming machine or the owner machine is not set yet.
	Machine                      *clusterv1.Machine
	HetznerCluster               *infrav1.HetznerCluster
	Cluster                      *clusterv1.Cluster
	OSSSHSecret                  *corev1.Secret
	RescueSSHSecret              *corev1.Secret
	PreProvisionCommand          string
	WorkloadClusterClientFactory WorkloadClusterClientFactory
}

// Name returns the HetznerCluster name.
func (s *BareMetalHostScope) Name() string {
	return s.HetznerBareMetalHost.Name
}

// Namespace returns the namespace name.
func (s *BareMetalHostScope) Namespace() string {
	return s.HetznerBareMetalHost.Namespace
}

// GetRawBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (s *BareMetalHostScope) GetRawBootstrapData(ctx context.Context) ([]byte, error) {
	if s.Machine == nil || s.Machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("no bootstrap data secret name on the machine")
	}

	key := types.NamespacedName{Namespace: s.HetznerBareMetalHost.Namespace, Name: *s.Machine.Spec.Bootstrap.DataSecretName}
	secret, err := s.SecretManager.AcquireSecret(ctx, key, s.HetznerBareMetalHost, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire secret: %w", err)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}

// Hostname returns the desired host name.
func (s *BareMetalHostScope) Hostname() (hostname string) {
	if s.hasConstantHostname() {
		hostname = fmt.Sprintf("%s%s-%v", infrav2.BareMetalHostNamePrefix, s.Cluster.Name, s.HetznerBareMetalHost.Spec.ServerID)
	} else {
		hostname = infrav2.BareMetalHostNamePrefix + s.HetznerBareMetalHost.Spec.ConsumerRef.Name
	}

	return hostname
}

func (s *BareMetalHostScope) hasConstantHostname() bool {
	return s.Cluster.GetAnnotations()[infrav2.ConstantBareMetalHostnameAnnotation] == "true" ||
		s.HetznerBareMetalMachine != nil && s.HetznerBareMetalMachine.GetAnnotations()[infrav2.ConstantBareMetalHostnameAnnotation] == "true"
}

// SSHAfterInstallImageEnabled returns the effective SSH-after-installimage setting for the host.
// When the consuming machine no longer exists, there is no SSH spec to connect with, so SSH access
// is treated as disabled.
func (s *BareMetalHostScope) SSHAfterInstallImageEnabled() bool {
	if s.HetznerBareMetalMachine == nil {
		return false
	}
	return !s.HetznerBareMetalMachine.Spec.SSHSpec.NoSSHAfterInstallImage
}

// SetHetznerBareMetalHostReadySummary computes and sets the Ready summary condition on the
// HetznerBareMetalHost. It is the single source of truth for computing the summary and is called
// from both the controller defer block and any early-exit paths that bypass it.
//
// If the summary cannot be computed, Ready is set to Unknown with InternalError reason so the
// summary is never silently omitted.
func SetHetznerBareMetalHostReadySummary(bmHost *infrav2.HetznerBareMetalHost) {
	readyCondition, err := conditions.NewSummaryCondition(
		bmHost, clusterv1.ReadyCondition,
		infrav2.HetznerBareMetalHostSummaryOpts()...,
	)
	if err != nil {
		conditions.Set(bmHost, metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  clusterv1.InternalErrorReason,
			Message: err.Error(),
		})
		return
	}
	conditions.Set(bmHost, *readyCondition)
}

// BareMetalHostPatchOpts returns the patch options declaring both the deprecated v1beta1 and the
// current owned conditions for HetznerBareMetalHost so the patch helper does not strip them on
// three-way merge.
func BareMetalHostPatchOpts() []patch.Option {
	return []patch.Option{
		// owned deprecated v1beta1 conditions. Kept identical to the pre-migration list so the
		// v1beta1 conditions behave exactly as before.
		patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyV1Beta1Condition,
			infrav2.CredentialsAvailableV1Beta1Condition,
			infrav2.RobotCredentialsAvailableV1Beta1Condition,
			infrav2.RootDeviceHintsValidatedV1Beta1Condition,
			infrav2.ProvisionSucceededV1Beta1Condition,
			infrav2.HetznerAPIReachableV1Beta1Condition,
		}},
		// owned conditions.
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav2.HetznerBareMetalHostSSHKeysAvailableCondition,
			infrav2.HetznerBareMetalHostRobotCredentialsAvailableCondition,
			infrav2.HetznerBareMetalHostRootDeviceHintsValidatedCondition,
			infrav2.HetznerBareMetalHostProvisionSucceededCondition,
			infrav2.HetznerBareMetalHostNodeBootIDRetrievedCondition,
			infrav2.HetznerBareMetalHostRebootSucceededCondition,
			infrav2.HetznerBareMetalHostDeletingCondition,
			infrav2.HetznerBareMetalHostRobotRateLimitExceededCondition,
			infrav2.HetznerBareMetalHostActionCompletedCondition,
		}},
	}
}
