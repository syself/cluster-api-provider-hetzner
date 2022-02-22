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

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioner"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BareMetalHostScopeParams defines the input parameters used to create a new scope.
type BareMetalHostScopeParams struct {
	Client               client.Client
	HetznerBareMetalHost *infrav1.HetznerBareMetalHost
	HetznerCluster       *infrav1.HetznerCluster
	Provisioner          provisioner.Provisioner
	RobotClient          robotclient.Client
	SSHClient            sshclient.Client
	SSHSecret            *corev1.Secret
}

// NewBareMetalHostScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewBareMetalHostScope(ctx context.Context, params BareMetalHostScopeParams) (*BareMetalHostScope, error) {
	if params.Provisioner == nil {
		return nil, errors.New("cannot create baremetal host scope without provisioner")
	}
	if params.RobotClient == nil {
		return nil, errors.New("cannot create baremetal host scope without robot client")
	}
	if params.SSHSecret == nil {
		return nil, errors.New("cannot create baremetal host scope without ssh secret")
	}

	helper, err := patch.NewHelper(params.HetznerCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &BareMetalHostScope{
		Client:               params.Client,
		Provisioner:          params.Provisioner,
		RobotClient:          params.RobotClient,
		patchHelper:          helper,
		HetznerCluster:       params.HetznerCluster,
		HetznerBareMetalHost: params.HetznerBareMetalHost,
		SSHSecret:            params.SSHSecret,
	}, nil
}

// BareMetalHostScope defines the basic context for an actuator to operate upon.
type BareMetalHostScope struct {
	Client               client.Client
	Provisioner          provisioner.Provisioner
	RobotClient          robotclient.Client
	patchHelper          *patch.Helper
	HetznerBareMetalHost *infrav1.HetznerBareMetalHost
	HetznerCluster       *infrav1.HetznerCluster
	SSHSecret            *corev1.Secret
}

// Name returns the HetznerCluster name.
func (s *BareMetalHostScope) Name() string {
	return s.HetznerBareMetalHost.Name
}

// Namespace returns the namespace name.
func (s *BareMetalHostScope) Namespace() string {
	return s.HetznerBareMetalHost.Namespace
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *BareMetalHostScope) Close(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.HetznerBareMetalHost)
}

// PatchObject persists the machine spec and status.
func (s *BareMetalHostScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.HetznerBareMetalHost)
}

// SetOperationalStatus sets the operational status of the HetznerBareMetalHost.
func (s *BareMetalHostScope) SetOperationalStatus(sts infrav1.OperationalStatus) {
	s.HetznerBareMetalHost.Spec.Status.OperationalStatus = sts
}

// SetErrorCount sets the operational status of the HetznerBareMetalHost.
func (s *BareMetalHostScope) SetErrorCount(count int) {
	s.HetznerBareMetalHost.Spec.Status.ErrorCount = count
}
