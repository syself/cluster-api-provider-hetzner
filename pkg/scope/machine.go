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
	"errors"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// MachineScopeParams defines the input parameters used to create a new Scope.
type MachineScopeParams struct {
	Client           client.Client
	APIReader        client.Reader
	Logger           logr.Logger
	HetznerSecret    *corev1.Secret
	HCloudClient     hcloudclient.Client
	Cluster          *clusterv1.Cluster
	HetznerCluster   *infrav2.HetznerCluster
	Machine          *clusterv1.Machine
	HCloudMachine    *infrav2.HCloudMachine
	SSHClientFactory sshclient.Factory
}

var (
	// ErrBootstrapDataNotReady return an error if no bootstrap data is ready.
	ErrBootstrapDataNotReady = errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	// ErrFailureDomainNotFound returns an error if no region is found.
	ErrFailureDomainNotFound = errors.New("error no failure domain available")
	// ErrEmptyProviderID indicates an empty providerID.
	ErrEmptyProviderID = fmt.Errorf("providerID is empty")
	// ErrInvalidProviderID indicates an invalid providerID.
	ErrInvalidProviderID = fmt.Errorf("providerID is invalid")
	// ErrInvalidServerID indicates an invalid serverID.
	ErrInvalidServerID = fmt.Errorf("serverID is invalid")
)

// NewMachineScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.HCloudMachine == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudMachine")
	}
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.HetznerCluster == nil {
		return nil, errors.New("failed to generate new scope from nil HetznerCluster")
	}
	if params.HCloudClient == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudClient")
	}
	if params.APIReader == nil {
		return nil, errors.New("failed to generate new scope from nil APIReader")
	}

	emptyLogger := logr.Logger{}
	if params.Logger == emptyLogger {
		return nil, errors.New("failed to generate new scope from nil Logger")
	}

	patchHelper, err := patch.NewHelper(params.HCloudMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &MachineScope{
		Logger:           params.Logger,
		Client:           params.Client,
		APIReader:        params.APIReader,
		patchHelper:      patchHelper,
		hetznerSecret:    params.HetznerSecret,
		HCloudClient:     params.HCloudClient,
		Cluster:          params.Cluster,
		HetznerCluster:   params.HetznerCluster,
		Machine:          params.Machine,
		HCloudMachine:    params.HCloudMachine,
		SSHClientFactory: params.SSHClientFactory,
	}, nil
}

// MachineScope defines the basic context for an actuator to operate upon.
//
// It holds the HCloudMachine being reconciled and the patch helper used to persist changes to it.
type MachineScope struct {
	logr.Logger
	Client        client.Client
	APIReader     client.Reader
	patchHelper   *patch.Helper
	hetznerSecret *corev1.Secret

	HCloudClient hcloudclient.Client

	Cluster        *clusterv1.Cluster
	HetznerCluster *infrav2.HetznerCluster

	Machine          *clusterv1.Machine
	HCloudMachine    *infrav2.HCloudMachine
	SSHClientFactory sshclient.Factory
}

// Close closes the current scope persisting the machine configuration and status.
func (m *MachineScope) Close(ctx context.Context) error {
	// set summary for deprecated v1beta1 conditions.
	deprecatedv1beta1conditions.SetSummary(m.HCloudMachine)

	// set summary for conditions.
	readyCondition, err := conditions.NewSummaryCondition(
		m.HCloudMachine,
		clusterv1.ReadyCondition,
		infrav2.HCloudMachineSummaryOpts()...,
	)
	if err != nil {
		// Note, this could only happen if we hit edge cases in computing the summary, which should not happen due to the fact
		// that we are passing a non empty list of ForConditionTypes.
		m.Error(err, "Failed to set Ready condition")
		unknownReadyCondition := metav1.Condition{
			Type:   clusterv1.ReadyCondition,
			Status: metav1.ConditionUnknown,
			Reason: clusterv1.InternalErrorReason,
		}

		conditions.Set(m.HCloudMachine, unknownReadyCondition)

		patchErr := m.patchHelper.Patch(ctx, m.HCloudMachine, machinePatchOpts()...)
		return errors.Join(err, patchErr)
	}

	conditions.Set(m.HCloudMachine, *readyCondition)

	return m.patchHelper.Patch(ctx, m.HCloudMachine, machinePatchOpts()...)
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

// HetznerSecret returns the hetzner secret.
func (m *MachineScope) HetznerSecret() *corev1.Secret {
	return m.hetznerSecret
}

// PatchObject persists the machine spec and status.
func (m *MachineScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.HCloudMachine, machinePatchOpts()...)
}

// machinePatchOpts returns the list of patch.Option for HCloudMachine.
func machinePatchOpts() []patch.Option {
	return []patch.Option{
		// owned deprecated v1beta1 conditions.
		patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyV1Beta1Condition,
			infrav2.BootstrapReadyV1Beta1Condition,
			infrav2.HCloudTokenAvailableV1Beta1Condition,
			infrav2.HetznerAPIReachableV1Beta1Condition,
			infrav2.SSHPrivateKeyAvailableV1Beta1Condition,
			infrav2.ServerCreateSucceededV1Beta1Condition,
			infrav2.ServerProvisionedV1Beta1Condition,
			infrav2.ServerAvailableV1Beta1Condition,
		}},
		// owned conditions.
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav2.HCloudTokenAvailableCondition,
			infrav2.HCloudRateLimitExceededCondition,
			infrav2.HCloudMachineSSHPrivateKeyAvailableCondition,
			infrav2.HCloudMachineServerCreatedCondition,
			infrav2.HCloudMachineServerProvisionedCondition,
			infrav2.HCloudMachineServerAvailableCondition,
		}},
	}
}

// SetErrorAndRemediate sets "cluster.x-k8s.io/remediate-machine" annotation on the corresponding
// CAPI machine. CAPI will remediate that machine. Additionally, an event of type Warning will be
// created, and the DeleteMachineSucceededCondition will be set to False on the hcloud-machine. It
// gets used, when a not-recoverable error happens. Example: hcloud server was deleted by hand in
// the hcloud UI.
func (m *MachineScope) SetErrorAndRemediate(ctx context.Context, message string) error {
	return SetRemediateMachineAnnotationToDeleteMachine(ctx, m.Client, m.Machine, m.HCloudMachine, message)
}

// SetRemediateMachineAnnotationToDeleteMachine sets "cluster.x-k8s.io/remediate-machine" annotation
// on the corresponding CAPI machine. This will trigger CAPI to start remediation. Our remediation
// contoller will inspect BootState to differentiate between a remediate (with reboot) and delete
// (no reboot gets tried). Finally the capi machine and the infra machine will be deleted.
//
// Background: the hcloudmachine controller has no permission to delete a capi machine. That's why
// this extra step (via remediate-machine annotation) is needed.
func SetRemediateMachineAnnotationToDeleteMachine(ctx context.Context, crClient client.Client, capiMachine *clusterv1.Machine, hcloudMachine *infrav2.HCloudMachine, message string) error {
	// Create a patch base
	patch := client.MergeFrom(capiMachine.DeepCopy())

	// Modify only annotations on the in-memory copy
	if capiMachine.Annotations == nil {
		capiMachine.Annotations = map[string]string{}
	}
	capiMachine.Annotations[clusterv1.RemediateMachineAnnotation] = ""

	// Apply patch – only the diff (annotations) is sent to the API server
	if err := crClient.Patch(ctx, capiMachine, patch); err != nil {
		return fmt.Errorf("patch failed in SetErrorAndRemediate: %w", err)
	}

	record.Warnf(hcloudMachine,
		"HCloudMachineWillBeRemediated",
		"HCloudMachine will be remediated: %s", message)

	hcloudMachine.SetBootState(infrav2.HCloudBootStateProvisioningFailed)

	return nil
}

// SetRegion sets the region field on the machine.
func (m *MachineScope) SetRegion(region string) {
	m.HCloudMachine.Status.Region = infrav2.Region(region)
}

// SetProviderID sets the providerID field on the machine.
func (m *MachineScope) SetProviderID(serverID int64) {
	providerID := fmt.Sprintf("hcloud://%d", serverID)
	m.HCloudMachine.Spec.ProviderID = &providerID
}

// ServerIDFromProviderID converts the ProviderID (hcloud://NNNN) to the ServerID.
func (m *MachineScope) ServerIDFromProviderID() (int64, error) {
	if m.HCloudMachine.Spec.ProviderID == nil || *m.HCloudMachine.Spec.ProviderID == "" {
		return 0, ErrEmptyProviderID
	}
	prefix := "hcloud://"
	if !strings.HasPrefix(*m.HCloudMachine.Spec.ProviderID, prefix) {
		return 0, ErrInvalidProviderID
	}

	serverID, err := strconv.ParseInt(strings.TrimPrefix(*m.HCloudMachine.Spec.ProviderID, prefix), 10, 64)
	if err != nil {
		return 0, ErrInvalidServerID
	}
	return serverID, nil
}

// SetProvisioned records that the machine's infrastructure is provisioned. Provisioned is a one-time
// signal per the CAPI infra-machine contract: once true it stays true, so a false argument is a no-op.
func (m *MachineScope) SetProvisioned(provisioned bool) {
	if provisioned {
		m.HCloudMachine.Status.Initialization.Provisioned = ptr.To(true)
	}
}

// HasServerAvailableCondition reports whether the ServerAvailable condition is currently True.
//
// The delete flow relies on this as a one-shot gate: a running server is shut down while
// ServerAvailable is still True, the shutdown step then sets it False, and the next reconcile deletes
// the server instead of shutting it down again. For this to work, Delete() must not set
// ServerAvailable to False before this gate runs, otherwise the graceful shutdown would be skipped.
func (m *MachineScope) HasServerAvailableCondition() bool {
	return conditions.IsTrue(m.HCloudMachine, infrav2.HCloudMachineServerAvailableCondition)
}

// IsBootstrapDataReady checks the readiness of a capi machine's bootstrap data.
func (m *MachineScope) IsBootstrapDataReady() bool {
	return m.Machine.Spec.Bootstrap.DataSecretName != nil
}

// GetFailureDomain returns the machine's failure domain or a default one based on a hash.
func (m *MachineScope) GetFailureDomain() (string, error) {
	if m.Machine.Spec.FailureDomain != "" {
		return m.Machine.Spec.FailureDomain, nil
	}

	failureDomainNames := make([]string, 0, len(m.Cluster.Status.FailureDomains))
	for _, fd := range m.Cluster.Status.FailureDomains {
		// filter out zones if we are a control plane and the cluster object
		// wants to avoid contorl planes in that zone
		if m.IsControlPlane() && (fd.ControlPlane == nil || !*fd.ControlPlane) {
			continue
		}
		failureDomainNames = append(failureDomainNames, fd.Name)
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
	secretManager := secretutil.NewSecretManager(m.Logger, m.Client, m.APIReader)
	secret, err := secretManager.AcquireSecret(ctx, key, m.HCloudMachine, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire secret: %w", err)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}
