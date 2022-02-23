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
	"hash/crc32"
	"sort"

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
)

// MachineScopeParams defines the input parameters used to create a new Scope.
type MachineScopeParams struct {
	ClusterScopeParams
	Machine       *clusterv1.Machine
	HCloudMachine *infrav1.HCloudMachine
}

// ErrBootstrapDataNotReady return an error if no bootstrap data is ready.
var ErrBootstrapDataNotReady = errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")

// ErrFailureDomainNotFound returns an error if no region is found.
var ErrFailureDomainNotFound = errors.New("error no failure domain available")

// NewMachineScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachineScope(ctx context.Context, params MachineScopeParams) (*MachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.HCloudMachine == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudMachine")
	}

	cs, err := NewClusterScope(ctx, params.ClusterScopeParams)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	cs.patchHelper, err = patch.NewHelper(params.HCloudMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &MachineScope{
		ClusterScope:  *cs,
		Machine:       params.Machine,
		HCloudMachine: params.HCloudMachine,
	}, nil
}

// MachineScope defines the basic context for an actuator to operate upon.
type MachineScope struct {
	ClusterScope
	Machine       *clusterv1.Machine
	HCloudMachine *infrav1.HCloudMachine
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachineScope) Close(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.HCloudMachine)
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// Name returns the HCloudMachine name.
func (m *MachineScope) Name() string {
	return m.HCloudMachine.Name
}

// Namespace returns the namespace name.
func (m *MachineScope) Namespace() string {
	return m.HCloudMachine.Namespace
}

// PatchObject persists the machine spec and status.
func (m *MachineScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.HCloudMachine)
}

// IsBootstrapDataReady checks the readiness of a capi machine's bootstrap data.
func (m *MachineScope) IsBootstrapDataReady(ctx context.Context) bool {
	return m.Machine.Spec.Bootstrap.DataSecretName != nil
}

// GetFailureDomain returns the machine's failure domain or a default one based on a hash.
func (m *MachineScope) GetFailureDomain() (string, error) {
	if m.Machine.Spec.FailureDomain != nil {
		return *m.Machine.Spec.FailureDomain, nil
	}

	failureDomainNames := make([]string, 0, len(m.Cluster.Status.FailureDomains))
	for fdName, fd := range m.Cluster.Status.FailureDomains {
		// filter out zones if we are a control plane and the cluster object
		// wants to avoid contorl planes in that zone
		if m.IsControlPlane() && !fd.ControlPlane {
			continue
		}
		failureDomainNames = append(failureDomainNames, fdName)
	}

	if len(failureDomainNames) == 0 {
		return "", ErrFailureDomainNotFound
	}
	if len(failureDomainNames) == 1 {
		return failureDomainNames[0], nil
	}

	sort.Strings(failureDomainNames)

	// assign the node a zone based on a hash
	pos := int(crc32.ChecksumIEEE([]byte(m.HCloudMachine.Name))) % len(failureDomainNames)

	return failureDomainNames[pos], nil
}

// GetRawBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetRawBootstrapData(ctx context.Context) ([]byte, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, ErrBootstrapDataNotReady
	}

	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	secretManager := secretutil.NewSecretManager(*m.Logger, m.Client, m.APIReader)
	secret, err := secretManager.AcquireSecret(ctx, key, m.HCloudMachine, false, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire secret")
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}
