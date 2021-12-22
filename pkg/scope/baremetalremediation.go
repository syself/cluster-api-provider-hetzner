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

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
)

// BareMetalRemediationScopeParams defines the input parameters used to create a new Scope.
type BareMetalRemediationScopeParams struct {
	BareMetalMachineScopeParams
	BareMetalRemediation *infrav1.HetznerBareMetalRemediation
}

// NewBareMetalRemediationScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewBareMetalRemediationScope(ctx context.Context, params BareMetalRemediationScopeParams) (*BareMetalRemediationScope, error) {
	if params.BareMetalRemediation == nil {
		return nil, errors.New("failed to generate new scope from nil BareMetalRemediation")
	}

	cs, err := NewBareMetalMachineScope(ctx, params.BareMetalMachineScopeParams)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	cs.patchHelper, err = patch.NewHelper(params.BareMetalRemediation, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &BareMetalRemediationScope{
		BareMetalMachineScope: *cs,
		BareMetalRemediation:  params.BareMetalRemediation,
	}, nil
}

// BareMetalRemediationScope defines the basic context for an actuator to operate upon.
type BareMetalRemediationScope struct {
	BareMetalMachineScope
	BareMetalRemediation *infrav1.HetznerBareMetalRemediation
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *BareMetalRemediationScope) Close(ctx context.Context, opts ...patch.Option) error {
	return m.patchHelper.Patch(ctx, m.BareMetalRemediation, opts...)
}

// Name returns the BareMetalMachine name.
func (m *BareMetalRemediationScope) Name() string {
	return m.BareMetalRemediation.Name
}

// Namespace returns the namespace name.
func (m *BareMetalRemediationScope) Namespace() string {
	return m.BareMetalRemediation.Namespace
}

// PatchObject persists the machine spec and status.
func (m *BareMetalRemediationScope) PatchObject(ctx context.Context, opts ...patch.Option) error {
	return m.patchHelper.Patch(ctx, m.BareMetalRemediation, opts...)
}
