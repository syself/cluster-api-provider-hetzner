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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// BareMetalMachineScopeParams defines the input parameters used to create a new Scope.
type BareMetalMachineScopeParams struct {
	Logger           logr.Logger
	Client           client.Client
	Cluster          *clusterv1.Cluster
	Machine          *clusterv1.Machine
	BareMetalMachine *infrav1.HetznerBareMetalMachine
	HetznerCluster   *infrav1.HetznerCluster
	HCloudClient     hcloudclient.Client
	EventRecorder    record.EventRecorder
}

// NewBareMetalMachineScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewBareMetalMachineScope(params BareMetalMachineScopeParams) (*BareMetalMachineScope, error) {
	if params.Client == nil {
		return nil, fmt.Errorf("cannot create baremetal machine scope without client")
	}
	if params.EventRecorder == nil {
		return nil, fmt.Errorf("cannot create baremetal machine scope without EventRecorder")
	}
	if params.Cluster == nil {
		return nil, fmt.Errorf("failed to generate new scope from nil Cluster")
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
		Cluster:          params.Cluster,
		Machine:          params.Machine,
		BareMetalMachine: params.BareMetalMachine,
		HetznerCluster:   params.HetznerCluster,
		HCloudClient:     params.HCloudClient,
		EventRecorder:    params.EventRecorder,
	}, nil
}

// BareMetalMachineScope defines the basic context for an actuator to operate upon.
type BareMetalMachineScope struct {
	logr.Logger
	Client           client.Client
	patchHelper      *patch.Helper
	Cluster          *clusterv1.Cluster
	Machine          *clusterv1.Machine
	BareMetalMachine *infrav1.HetznerBareMetalMachine
	HetznerCluster   *infrav1.HetznerCluster

	HCloudClient  hcloudclient.Client
	EventRecorder record.EventRecorder
}

// Close closes the current scope persisting the machine configuration and status.
func (m *BareMetalMachineScope) Close(ctx context.Context) error {
	deprecatedv1beta1conditions.SetSummary(m.BareMetalMachine)
	SetHetznerBareMetalMachineReadySummary(m.BareMetalMachine)

	return m.patchHelper.Patch(ctx, m.BareMetalMachine, bareMetalMachinePatchOpts()...)
}

// SetHetznerBareMetalMachineReadySummary computes and sets the Ready summary condition on the
// HetznerBareMetalMachine.
//
// If the summary cannot be computed, Ready is set to Unknown with the InternalError reason and the
// error message.
func SetHetznerBareMetalMachineReadySummary(hbmm *infrav1.HetznerBareMetalMachine) {
	readyCondition, err := conditions.NewSummaryCondition(
		hbmm, clusterv1.ReadyCondition,
		infrav1.HetznerBareMetalMachineSummaryOpts()...,
	)
	if err != nil {
		conditions.Set(hbmm, metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  clusterv1.InternalErrorReason,
			Message: err.Error(),
		})
		return
	}
	conditions.Set(hbmm, *readyCondition)
}

func bareMetalMachinePatchOpts() []patch.Option {
	return []patch.Option{
		// owned deprecated v1beta1 conditions.
		patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyV1Beta1Condition,
			infrav1.BootstrapReadyV1Beta1Condition,
			infrav1.HCloudTokenAvailableV1Beta1Condition,
			infrav1.HetznerAPIReachableV1Beta1Condition,
			infrav1.HostAssociateSucceededV1Beta1Condition,
			infrav1.HostReadyV1Beta1Condition,
			infrav1.ServerAvailableV1Beta1Condition,
		}},
		// owned conditions.
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav1.HCloudTokenAvailableCondition,
			infrav1.HCloudRateLimitExceededCondition,
			infrav1.HetznerBareMetalMachineHostAssociatedCondition,
			infrav1.HetznerBareMetalMachineDeletingCondition,
			infrav1.HetznerBareMetalMachineHostReadyCondition,
			infrav1.HetznerBareMetalMachineServerAvailableCondition,
		}},
	}
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
func (m *BareMetalMachineScope) PatchObject(ctx context.Context, opts ...patch.Option) error {
	allOpts := append(bareMetalMachinePatchOpts(), opts...)
	return m.patchHelper.Patch(ctx, m.BareMetalMachine, allOpts...)
}

// IsControlPlane returns true if the machine is a control plane.
func (m *BareMetalMachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// IsBootstrapReady checks the readiness of a capi machine's bootstrap data.
func (m *BareMetalMachineScope) IsBootstrapReady() bool {
	return m.Machine.Spec.Bootstrap.DataSecretName != nil
}

// SetRemediateMachineAnnotationToDeleteMachine sets "cluster.x-k8s.io/remediate-machine" annotation
// on the corresponding CAPI machine. This will trigger CAPI to start remediation. Our remediation
// contoller will use HasFatalError() to differentiate between a remediate (with reboot) and delete
// (no reboot gets tried). Finally the capi machine and the infra machine will be deleted.
//
// Background: the hbmm/hbmh controller has no permission to delete a capi machine. That's why this
// extra step (via remediate-machine annotation) is needed.
func (m *BareMetalMachineScope) SetRemediateMachineAnnotationToDeleteMachine(ctx context.Context, message string) error {
	capiMachine := m.Machine

	// Create a patch base
	patch := client.MergeFrom(capiMachine.DeepCopy())

	// Modify only annotations on the in-memory copy
	if capiMachine.Annotations == nil {
		capiMachine.Annotations = map[string]string{}
	}
	capiMachine.Annotations[clusterv1.RemediateMachineAnnotation] = ""

	// Apply patch – only the diff (annotations) is sent to the API server
	if err := m.Client.Patch(ctx, capiMachine, patch); err != nil {
		return err
	}

	m.EventRecorder.Eventf(
		m.BareMetalMachine,
		corev1.EventTypeWarning,
		"MachineWillBeDeleted",
		"Machine will be deleted: %s",
		message,
	)
	return nil
}
