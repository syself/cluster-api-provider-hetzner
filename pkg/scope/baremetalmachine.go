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
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// BareMetalMachineScopeParams defines the input parameters used to create a new Scope.
type BareMetalMachineScopeParams struct {
	Logger           logr.Logger
	Client           client.Client
	Machine          *clusterv1.Machine
	BareMetalMachine *infrav1.HetznerBareMetalMachine
	HetznerCluster   *infrav1.HetznerCluster
	HCloudClient     hcloudclient.Client
}

// NewBareMetalMachineScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewBareMetalMachineScope(params BareMetalMachineScopeParams) (*BareMetalMachineScope, error) {
	if params.Client == nil {
		return nil, fmt.Errorf("cannot create baremetal machine scope without client")
	}
	if params.Machine == nil {
		return nil, fmt.Errorf("failed to generate new scope from nil Machine")
	}
	if params.BareMetalMachine == nil {
		return nil, fmt.Errorf("failed to generate new scope from nil BareMetalMachine")
	}
	if params.HetznerCluster == nil {
		return nil, fmt.Errorf("failed to generate new scope from nil HetznerCluster")
	}
	if params.HCloudClient == nil {
		return nil, fmt.Errorf("failed to generate new scope from nil HCloudClient")
	}

	var emptyLogger logr.Logger
	if params.Logger == emptyLogger {
		return nil, fmt.Errorf("failed to generate new scope from nil Logger")
	}

	patchHelper, err := patch.NewHelper(params.BareMetalMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &BareMetalMachineScope{
		Logger:           params.Logger,
		Client:           params.Client,
		patchHelper:      patchHelper,
		Machine:          params.Machine,
		BareMetalMachine: params.BareMetalMachine,
		HetznerCluster:   params.HetznerCluster,
		HCloudClient:     params.HCloudClient,
	}, nil
}

// BareMetalMachineScope defines the basic context for an actuator to operate upon.
type BareMetalMachineScope struct {
	logr.Logger
	Client           client.Client
	patchHelper      *patch.Helper
	Machine          *clusterv1.Machine
	BareMetalMachine *infrav1.HetznerBareMetalMachine
	HetznerCluster   *infrav1.HetznerCluster

	HCloudClient hcloudclient.Client
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *BareMetalMachineScope) Close(ctx context.Context) error {
	conditions.SetSummary(m.BareMetalMachine)
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

// IsControlPlane returns true if the machine is a control plane.
func (m *BareMetalMachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// IsBootstrapReady checks the readiness of a capi machine's bootstrap data.
func (m *BareMetalMachineScope) IsBootstrapReady() bool {
	return m.Machine.Spec.Bootstrap.DataSecretName != nil
}

// SetErrorAndDeleteCapiMachine deletes the corresponding CAPI machine. CAPI will remediate that
// machine. Additionally, an event of type Warning will be created, and the condition will be set on
// the BareMetalMachine NoRemediateMachineAnnotationCondition.
func (m *BareMetalMachineScope) SetErrorAndDeleteCapiMachine(ctx context.Context, message string) error {
	if err := m.Client.Delete(ctx, m.Machine); err != nil {
		return fmt.Errorf("SetErrorAndDeleteCapiMachine Delete failed: %w", err)
	}

	record.Warnf(m.BareMetalMachine, "HetznerBareMetalMachineWillBeRemediated",
		"HetznerBareMetalMachine will be remediated: %s", message)

	conditions.MarkFalse(m.BareMetalMachine, infrav1.NoPermanentErrorCondition,
		infrav1.PermanentErrorConditionIsSet, clusterv1.ConditionSeverityInfo, "%s", message)
	return nil
}

func splitHostKey(key string) (namespace, name string) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		panic("unexpected host key")
	}
	return parts[0], parts[1]
}

// GetAssociatedHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (m *BareMetalMachineScope) GetAssociatedHost(ctx context.Context) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	annotations := m.BareMetalMachine.ObjectMeta.GetAnnotations()
	// if no annotations exist on machine, no host can be associated
	if annotations == nil {
		return nil, nil, nil
	}

	// check if host annotation is set and return if not
	hostKey, ok := annotations[infrav1.HostAnnotation]
	if !ok {
		return nil, nil, nil
	}

	// find associated host object and return it
	hostNamespace, hostName := splitHostKey(hostKey)

	host := infrav1.HetznerBareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}

	if err := m.Client.Get(ctx, key, &host); err != nil {
		return nil, nil, fmt.Errorf("failed to get host object: %w", err)
	}

	helper, err := patch.NewHelper(&host, m.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create patch helper: %w", err)
	}

	return &host, helper, nil
}
