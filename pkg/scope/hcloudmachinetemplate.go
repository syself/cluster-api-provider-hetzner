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

// Package scope defines cluster and machine scope as well as a repository for the Hetzner API.
package scope

import (
	"errors"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2/textlogger"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// HCloudMachineTemplateScopeParams defines the input parameters used to create a new scope.
type HCloudMachineTemplateScopeParams struct {
	Logger                *logr.Logger
	HCloudClient          hcloudclient.Client
	HCloudMachineTemplate *infrav1.HCloudMachineTemplate
}

// NewHCloudMachineTemplateScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewHCloudMachineTemplateScope(params HCloudMachineTemplateScopeParams) (*HCloudMachineTemplateScope, error) {
	if params.HCloudClient == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudClient")
	}

	if params.Logger == nil {
		logger := textlogger.NewLogger(textlogger.NewConfig())
		params.Logger = &logger
	}

	return &HCloudMachineTemplateScope{
		Logger:                params.Logger,
		HCloudMachineTemplate: params.HCloudMachineTemplate,
		HCloudClient:          params.HCloudClient,
	}, nil
}

// HCloudMachineTemplateScope defines the basic context for an actuator to operate upon.
type HCloudMachineTemplateScope struct {
	*logr.Logger
	HCloudClient hcloudclient.Client

	HCloudMachineTemplate *infrav1.HCloudMachineTemplate
}

// Name returns the HCloudMachineTemplate name.
func (s *HCloudMachineTemplateScope) Name() string {
	return s.HCloudMachineTemplate.Name
}

// Namespace returns the namespace name.
func (s *HCloudMachineTemplateScope) Namespace() string {
	return s.HCloudMachineTemplate.Namespace
}

// SetHCloudMachineTemplateV1Beta2SummaryCondition computes the HCloudMachineTemplate v1beta2 Ready condition.
func SetHCloudMachineTemplateV1Beta2SummaryCondition(hcloudMachineTemplate *infrav1.HCloudMachineTemplate) error {
	return v1beta2conditions.SetSummaryCondition(hcloudMachineTemplate, hcloudMachineTemplate, clusterv1beta1.ReadyV1Beta2Condition,
		infrav1.HCloudMachineTemplateV1Beta2SummaryOpts()...,
	)
}

// MachineTemplatePatchOpts returns the list of patch.Option for HCloudMachineTemplate,
// declaring both the v1beta1 and v1beta2 conditions owned by this controller so the
// patch helper handles three-way merge correctly across concurrent updates.
func MachineTemplatePatchOpts() []v1beta1patch.Option {
	return []v1beta1patch.Option{
		// owned v1beta1 conditions.
		v1beta1patch.WithOwnedConditions{Conditions: []clusterv1beta1.ConditionType{
			clusterv1beta1.ReadyCondition,
			infrav1.HCloudTokenAvailableCondition,
			infrav1.HetznerAPIReachableCondition,
		}},
		// owned v1beta2 conditions.
		v1beta1patch.WithOwnedV1Beta2Conditions{Conditions: []string{
			clusterv1beta1.ReadyV1Beta2Condition,
			infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
			infrav1.HCloudTokenAvailableV1Beta2Condition,
			infrav1.HCloudRateLimitExceededV1Beta2Condition,
		}},
	}
}
