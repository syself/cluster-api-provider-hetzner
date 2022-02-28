/*
Copyright 2018 The Kubernetes Authors.
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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

const (
	// HostAnnotation is the key for an annotation that should go on a HetznerBareMetalMachine to
	// reference what HetznerBareMetalHost it corresponds to.
	HostAnnotation = "infrastructure.cluster.x-k8s.io/HetznerBareMetalHost"
)

// BareMetalMachineScopeParams defines the input parameters used to create a new Scope.
type BareMetalMachineScopeParams struct {
	ClusterScopeParams
	Machine          *clusterv1.Machine
	BareMetalMachine *infrav1.HetznerBareMetalMachine
}

// NewBareMetalMachineScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewBareMetalMachineScope(ctx context.Context, params BareMetalMachineScopeParams) (*BareMetalMachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.BareMetalMachine == nil {
		return nil, errors.New("failed to generate new scope from nil BareMetalMachine")
	}

	cs, err := NewClusterScope(ctx, params.ClusterScopeParams)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	cs.patchHelper, err = patch.NewHelper(params.BareMetalMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &BareMetalMachineScope{
		ClusterScope:     *cs,
		Machine:          params.Machine,
		BareMetalMachine: params.BareMetalMachine,
	}, nil
}

// BareMetalMachineScope defines the basic context for an actuator to operate upon.
type BareMetalMachineScope struct {
	ClusterScope
	Machine          *clusterv1.Machine
	BareMetalMachine *infrav1.HetznerBareMetalMachine
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *BareMetalMachineScope) Close(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.BareMetalMachine)
}

// Name returns the BareMetalMachine name.
func (m *BareMetalMachineScope) Name() string {
	return m.BareMetalMachine.Name
}

// Namespace returns the namespace name.
func (m *BareMetalMachineScope) Namespace() string {
	return m.BareMetalMachine.Namespace
}

// PatchObject persists the machine spec and status.
func (m *BareMetalMachineScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.BareMetalMachine)
}

// SetError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (m *BareMetalMachineScope) SetError(message string, reason capierrors.MachineStatusError) {
	m.BareMetalMachine.Status.FailureMessage = &message
	m.BareMetalMachine.Status.FailureReason = &reason
}

// IsProvisioned checks if the bareMetalMachine is provisioned
func (m *BareMetalMachineScope) IsProvisioned() bool {
	if m.BareMetalMachine.Spec.ProviderID != nil && m.BareMetalMachine.Status.Ready {
		return true
	}
	return false
}

// IsBootstrapReady checks the readiness of a capi machine's bootstrap data.
func (m *BareMetalMachineScope) IsBootstrapReady(ctx context.Context) bool {
	return m.Machine.Status.BootstrapReady
}

// HasAnnotation makes sure the machine has an annotation that references a host
func (m *BareMetalMachineScope) HasAnnotation() bool {
	annotations := m.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[HostAnnotation]
	return ok
}

// clearError removes the ErrorMessage from the machine's Status if set. Returns
// nil if ErrorMessage was already nil. Returns a RequeueAfterError if the
// machine was updated.
func (m *BareMetalMachineScope) ClearError() {
	if m.BareMetalMachine.Status.FailureMessage != nil || m.BareMetalMachine.Status.FailureReason != nil {
		m.BareMetalMachine.Status.FailureMessage = nil
		m.BareMetalMachine.Status.FailureReason = nil
	}
}
