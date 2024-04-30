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
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// BareMetalHostScopeParams defines the input parameters used to create a new scope.
type BareMetalHostScopeParams struct {
	Client               client.Client
	Logger               logr.Logger
	HetznerBareMetalHost *infrav1.HetznerBareMetalHost
	HetznerCluster       *infrav1.HetznerCluster
	Cluster              *clusterv1.Cluster
	RobotClient          robotclient.Client
	SSHClientFactory     sshclient.Factory
	OSSSHSecret          *corev1.Secret
	RescueSSHSecret      *corev1.Secret
	SecretManager        *secretutil.SecretManager
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
		Logger:               params.Logger,
		Client:               params.Client,
		RobotClient:          params.RobotClient,
		SSHClientFactory:     params.SSHClientFactory,
		HetznerCluster:       params.HetznerCluster,
		Cluster:              params.Cluster,
		HetznerBareMetalHost: params.HetznerBareMetalHost,
		OSSSHSecret:          params.OSSSHSecret,
		RescueSSHSecret:      params.RescueSSHSecret,
		SecretManager:        params.SecretManager,
	}, nil
}

// BareMetalHostScope defines the basic context for an actuator to operate upon.
type BareMetalHostScope struct {
	logr.Logger
	Client               client.Client
	SecretManager        *secretutil.SecretManager
	RobotClient          robotclient.Client
	SSHClientFactory     sshclient.Factory
	HetznerBareMetalHost *infrav1.HetznerBareMetalHost
	HetznerCluster       *infrav1.HetznerCluster
	Cluster              *clusterv1.Cluster
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

// GetRawBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (s *BareMetalHostScope) GetRawBootstrapData(ctx context.Context) ([]byte, error) {
	if s.HetznerBareMetalHost.Spec.Status.UserData == nil {
		return nil, errors.New("no user data in host spec")
	}

	key := types.NamespacedName{Namespace: s.HetznerBareMetalHost.Spec.Status.UserData.Namespace, Name: s.HetznerBareMetalHost.Spec.Status.UserData.Name}
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
		hostname = fmt.Sprintf("%s%s-%v", infrav1.BareMetalHostNamePrefix, s.Cluster.Name, s.HetznerBareMetalHost.Spec.ServerID)
	} else {
		hostname = infrav1.BareMetalHostNamePrefix + s.HetznerBareMetalHost.Spec.ConsumerRef.Name
	}

	return hostname
}

func (s *BareMetalHostScope) hasConstantHostname() bool {
	if s.Cluster.Annotations != nil {
		return s.Cluster.Annotations[infrav1.ConstantBareMetalHostnameAnnotation] == "true"
	}
	return false
}
