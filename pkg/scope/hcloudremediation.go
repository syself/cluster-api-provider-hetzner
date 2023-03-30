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

package scope

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HCloudRemediationScopeParams defines the input parameters used to create a new Scope.
type HCloudRemediationScopeParams struct {
	Logger            logr.Logger
	Client            client.Client
	HCloudClient      hcloudclient.Client
	Machine           *clusterv1.Machine
	HCloudMachine     *infrav1.HCloudMachine
	HetznerCluster    *infrav1.HetznerCluster
	HCloudRemediation *infrav1.HCloudRemediation
}

// NewHCloudRemediationScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewHCloudRemediationScope(ctx context.Context, params HCloudRemediationScopeParams) (*HCloudRemediationScope, error) {
	if params.HCloudRemediation == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudRemediation")
	}
	if params.Client == nil {
		return nil, errors.New("failed to generate new scope from nil client")
	}
	if params.HCloudClient == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudClient")
	}
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.HCloudMachine == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudMachine")
	}

	emptyLogger := logr.Logger{}
	if params.Logger == emptyLogger {
		return nil, errors.New("failed to generate new scope from nil Logger")
	}

	patchHelper, err := patch.NewHelper(params.HCloudRemediation, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	machinePatchHelper, err := patch.NewHelper(params.Machine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init machine patch helper")
	}

	return &HCloudRemediationScope{
		Logger:             params.Logger,
		Client:             params.Client,
		HCloudClient:       params.HCloudClient,
		patchHelper:        patchHelper,
		machinePatchHelper: machinePatchHelper,
		Machine:            params.Machine,
		HCloudMachine:      params.HCloudMachine,
		HCloudRemediation:  params.HCloudRemediation,
	}, nil
}

// HCloudRemediationScope defines the basic context for an actuator to operate upon.
type HCloudRemediationScope struct {
	logr.Logger
	Client             client.Client
	patchHelper        *patch.Helper
	machinePatchHelper *patch.Helper
	HCloudClient       hcloudclient.Client
	Machine            *clusterv1.Machine
	HCloudMachine      *infrav1.HCloudMachine
	HCloudRemediation  *infrav1.HCloudRemediation
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *HCloudRemediationScope) Close(ctx context.Context, opts ...patch.Option) error {
	return m.patchHelper.Patch(ctx, m.HCloudRemediation, opts...)
}

// Name returns the HCloudMachine name.
func (m *HCloudRemediationScope) Name() string {
	return m.HCloudRemediation.Name
}

// Namespace returns the namespace name.
func (m *HCloudRemediationScope) Namespace() string {
	return m.HCloudRemediation.Namespace
}

// ServerIDFromProviderID returns the namespace name.
func (m *HCloudRemediationScope) ServerIDFromProviderID() (int, error) {
	return hcloudutil.ServerIDFromProviderID(m.HCloudMachine.Spec.ProviderID)
}

// PatchObject persists the remediation spec and status.
func (m *HCloudRemediationScope) PatchObject(ctx context.Context, opts ...patch.Option) error {
	return m.patchHelper.Patch(ctx, m.HCloudRemediation, opts...)
}

// PatchMachine persists the machine spec and status.
func (m *HCloudRemediationScope) PatchMachine(ctx context.Context, opts ...patch.Option) error {
	return m.machinePatchHelper.Patch(ctx, m.Machine, opts...)
}
