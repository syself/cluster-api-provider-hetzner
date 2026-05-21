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

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
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
func NewHCloudRemediationScope(params HCloudRemediationScopeParams) (*HCloudRemediationScope, error) {
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

	patchHelper, err := v1beta1patch.NewHelper(params.HCloudRemediation, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &HCloudRemediationScope{
		Logger:            params.Logger,
		Client:            params.Client,
		HCloudClient:      params.HCloudClient,
		patchHelper:       patchHelper,
		Machine:           params.Machine,
		HCloudMachine:     params.HCloudMachine,
		HCloudRemediation: params.HCloudRemediation,
	}, nil
}

// HCloudRemediationScope defines the basic context for an actuator to operate upon.
type HCloudRemediationScope struct {
	logr.Logger
	Client            client.Client
	patchHelper       *v1beta1patch.Helper
	HCloudClient      hcloudclient.Client
	Machine           *clusterv1.Machine
	HCloudMachine     *infrav1.HCloudMachine
	HCloudRemediation *infrav1.HCloudRemediation
}

// Close closes the current scope persisting the remediation configuration and status.
func (m *HCloudRemediationScope) Close(ctx context.Context, opts ...v1beta1patch.Option) error {
	// set summary for v1beta1 conditions.
	v1beta1conditions.SetSummary(m.HCloudRemediation)

	allOpts := append(opts, HCloudRemediationPatchOpts()...)

	// set summary for v1beta2 conditions.

	readyCondition, err := v1beta2conditions.NewSummaryCondition(
		m.HCloudRemediation,
		clusterv1beta1.ReadyV1Beta2Condition,
		infrav1.HCloudRemediationV1Beta2SummaryOpts()...,
	)
	if err != nil {
		// Note, this could only happen if we hit edge cases in computing the summary, which should not happen due to the fact
		// that we are passing a non empty list of ForConditionTypes.
		m.Error(err, "Failed to set v1beta2 Ready condition")
		unknownReadyCondition := metav1.Condition{
			Type:   clusterv1beta1.ReadyV1Beta2Condition,
			Status: metav1.ConditionUnknown,
			Reason: infrav1.InternalErrorV1Beta2Reason,
		}

		// set the ready condition with unknown status.
		v1beta2conditions.Set(m.HCloudRemediation, unknownReadyCondition)

		patchErr := m.patchHelper.Patch(ctx, m.HCloudRemediation, allOpts...)
		return errors.Join(err, patchErr)
	}

	v1beta2conditions.Set(m.HCloudRemediation, *readyCondition)

	return m.patchHelper.Patch(ctx, m.HCloudRemediation, allOpts...)
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
func (m *HCloudRemediationScope) ServerIDFromProviderID() (int64, error) {
	return hcloudutil.ServerIDFromProviderID(m.HCloudMachine.Spec.ProviderID)
}

// PatchObject persists the remediation spec and status.
func (m *HCloudRemediationScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.HCloudRemediation, HCloudRemediationPatchOpts()...)
}

// HCloudRemediationPatchOpts returns the list of v1beta1patch.Option for HCloudRemediation.
// Exported so early-exit paths in the controller (that bypass the scope) can share the
// same owned-conditions list.
func HCloudRemediationPatchOpts() []v1beta1patch.Option {
	return []v1beta1patch.Option{
		// owned v1beta1 conditions.
		v1beta1patch.WithOwnedConditions{Conditions: []clusterv1beta1.ConditionType{
			clusterv1beta1.ReadyCondition,
			infrav1.HCloudTokenAvailableCondition,
			infrav1.HetznerAPIReachableCondition,
			infrav1.RemediationSkippedCondition,
		}},
		// owned v1beta2 conditions.
		v1beta1patch.WithOwnedV1Beta2Conditions{Conditions: []string{
			clusterv1beta1.ReadyV1Beta2Condition,
			infrav1.HCloudTokenAvailableV1Beta2Condition,
			infrav1.HCloudRateLimitExceededV1Beta2Condition,
			infrav1.HCloudRemediationSkippedV1Beta2Condition,
		}},
	}
}
