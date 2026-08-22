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
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
)

// HCloudRemediationScopeParams defines the input parameters used to create a new Scope.
type HCloudRemediationScopeParams struct {
	Logger            logr.Logger
	Client            client.Client
	HCloudClient      hcloudclient.Client
	Machine           *clusterv1.Machine
	HCloudMachine     *infrav2.HCloudMachine
	HetznerCluster    *infrav2.HetznerCluster
	HCloudRemediation *infrav2.HCloudRemediation
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

	patchHelper, err := patch.NewHelper(params.HCloudRemediation, params.Client)
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
	patchHelper       *patch.Helper
	HCloudClient      hcloudclient.Client
	Machine           *clusterv1.Machine
	HCloudMachine     *infrav2.HCloudMachine
	HCloudRemediation *infrav2.HCloudRemediation
}

// Close closes the current scope persisting the remediation configuration and status.
func (m *HCloudRemediationScope) Close(ctx context.Context, opts ...patch.Option) error {
	// set summary for deprecated v1beta1 conditions.
	deprecatedv1beta1conditions.SetSummary(m.HCloudRemediation)

	allOpts := append(opts, HCloudRemediationPatchOpts()...)

	// set summary for conditions.
	readyCondition, err := conditions.NewSummaryCondition(
		m.HCloudRemediation,
		clusterv1.ReadyCondition,
		infrav2.HCloudRemediationSummaryOpts()...,
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

		// set the ready condition with unknown status.
		conditions.Set(m.HCloudRemediation, unknownReadyCondition)

		patchErr := m.patchHelper.Patch(ctx, m.HCloudRemediation, allOpts...)
		return errors.Join(err, patchErr)
	}

	conditions.Set(m.HCloudRemediation, *readyCondition)

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

// HasRetriesLeft returns true if the retry limit is greater than retry count.
func (m *HCloudRemediationScope) HasRetriesLeft() bool {
	retryLimit := ptr.Deref(m.HCloudRemediation.Spec.Strategy.RetryLimit, 0)
	return retryLimit > 0 && retryLimit > ptr.Deref(m.HCloudRemediation.Status.RetryCount, 0)
}

// ServerIDFromProviderID returns the namespace name.
func (m *HCloudRemediationScope) ServerIDFromProviderID() (int64, error) {
	return hcloudutil.ServerIDFromProviderID(m.HCloudMachine.Spec.ProviderID)
}

// PatchObject persists the remediation spec and status.
func (m *HCloudRemediationScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.HCloudRemediation, HCloudRemediationPatchOpts()...)
}

// HCloudRemediationPatchOpts returns the list of patch.Option for HCloudRemediation.
// Exported so early-exit paths in the controller (that bypass the scope) can share the
// same owned-conditions list.
func HCloudRemediationPatchOpts() []patch.Option {
	return []patch.Option{
		// owned deprecated v1beta1 conditions.
		patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyV1Beta1Condition,
			infrav2.HCloudTokenAvailableV1Beta1Condition,
			infrav2.HetznerAPIReachableV1Beta1Condition,
			infrav2.RemediationSkippedV1Beta1Condition,
		}},
		// owned conditions.
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav2.HCloudTokenAvailableCondition,
			infrav2.HCloudRateLimitExceededCondition,
			infrav2.HCloudRemediationSkippedCondition,
		}},
	}
}
