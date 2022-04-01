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

// Package scope defines cluster and machine scope as well as a repository for the Hetzner API
package scope

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BareMetalHostScopeParams defines the input parameters used to create a new scope.
type BareMetalHostScopeParams struct {
	Client               client.Client
	Logger               *logr.Logger
	HetznerBareMetalHost *infrav1.HetznerBareMetalHost
	HetznerCluster       *infrav1.HetznerCluster
	RobotClient          robotclient.Client
	SSHClientFactory     sshclient.Factory
	OSSSHSecret          *corev1.Secret
	RescueSSHSecret      *corev1.Secret
	SecretManager        *secretutil.SecretManager
}

// NewBareMetalHostScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewBareMetalHostScope(ctx context.Context, params BareMetalHostScopeParams) (*BareMetalHostScope, error) {
	if params.Client == nil {
		return nil, errors.New("cannot create baremetal host scope without client")
	}
	if params.HetznerBareMetalHost == nil {
		return nil, errors.New("cannot create baremetal host scope without host object")
	}
	if params.HetznerCluster == nil {
		return nil, errors.New("cannot create baremetal host scope without Hetzner cluster")
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

	if params.Logger == nil {
		logger := klogr.New()
		params.Logger = &logger
	}

	return &BareMetalHostScope{
		Logger:               params.Logger,
		Client:               params.Client,
		RobotClient:          params.RobotClient,
		SSHClientFactory:     params.SSHClientFactory,
		HetznerCluster:       params.HetznerCluster,
		HetznerBareMetalHost: params.HetznerBareMetalHost,
		OSSSHSecret:          params.OSSSHSecret,
		RescueSSHSecret:      params.RescueSSHSecret,
		SecretManager:        params.SecretManager,
	}, nil
}

// BareMetalHostScope defines the basic context for an actuator to operate upon.
type BareMetalHostScope struct {
	*logr.Logger
	Client               client.Client
	SecretManager        *secretutil.SecretManager
	RobotClient          robotclient.Client
	SSHClientFactory     sshclient.Factory
	HetznerBareMetalHost *infrav1.HetznerBareMetalHost
	HetznerCluster       *infrav1.HetznerCluster
	OSSSHSecret          *corev1.Secret
	RescueSSHSecret      *corev1.Secret
}

// Name returns the HetznerCluster name.
func (s *BareMetalHostScope) Name() string {
	return s.HetznerBareMetalHost.Name
}

// Namespace returns the namespace name.
func (s *BareMetalHostScope) Namespace() string {
	return s.HetznerBareMetalHost.Namespace
}

// SetErrorCount sets the operational status of the HetznerBareMetalHost.
func (s *BareMetalHostScope) SetErrorCount(count int) {
	s.HetznerBareMetalHost.Spec.Status.ErrorCount = count
}

// GetRawBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (s *BareMetalHostScope) GetRawBootstrapData(ctx context.Context) ([]byte, error) {
	if s.HetznerBareMetalHost.Spec.Status.UserData == nil {
		return nil, errors.New("no user data in host spec")
	}

	key := types.NamespacedName{Namespace: s.HetznerBareMetalHost.Spec.Status.UserData.Namespace, Name: s.HetznerBareMetalHost.Spec.Status.UserData.Name}
	secret, err := s.SecretManager.AcquireSecret(ctx, key, s.HetznerBareMetalHost, false, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire secret")
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}
