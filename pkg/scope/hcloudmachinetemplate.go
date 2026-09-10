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
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2/textlogger"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// HCloudMachineTemplateScopeParams defines the input parameters used to create a new scope.
type HCloudMachineTemplateScopeParams struct {
	Logger                *logr.Logger
	HCloudClient          hcloudclient.Client
	HCloudMachineTemplate *infrav2.HCloudMachineTemplate
	EventRecorder         record.EventRecorder
}

// NewHCloudMachineTemplateScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewHCloudMachineTemplateScope(params HCloudMachineTemplateScopeParams) (*HCloudMachineTemplateScope, error) {
	if params.HCloudClient == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudClient")
	}
	if params.EventRecorder == nil {
		return nil, errors.New("cannot create hcloud machine template scope without EventRecorder")
	}

	if params.Logger == nil {
		logger := textlogger.NewLogger(textlogger.NewConfig())
		params.Logger = &logger
	}

	return &HCloudMachineTemplateScope{
		Logger:                params.Logger,
		HCloudMachineTemplate: params.HCloudMachineTemplate,
		HCloudClient:          params.HCloudClient,
		EventRecorder:         params.EventRecorder,
	}, nil
}

// HCloudMachineTemplateScope defines the basic context for an actuator to operate upon.
type HCloudMachineTemplateScope struct {
	*logr.Logger
	HCloudClient hcloudclient.Client

	HCloudMachineTemplate *infrav2.HCloudMachineTemplate
	EventRecorder         record.EventRecorder
}

// Name returns the HCloudMachineTemplate name.
func (s *HCloudMachineTemplateScope) Name() string {
	return s.HCloudMachineTemplate.Name
}

// Namespace returns the namespace name.
func (s *HCloudMachineTemplateScope) Namespace() string {
	return s.HCloudMachineTemplate.Namespace
}

// SetHCloudMachineTemplateSummaryCondition computes and sets the HCloudMachineTemplate Ready condition.
func SetHCloudMachineTemplateSummaryCondition(hcloudMachineTemplate *infrav2.HCloudMachineTemplate) error {
	readyCondition, err := conditions.NewSummaryCondition(
		hcloudMachineTemplate,
		clusterv1.ReadyCondition,
		infrav2.HCloudMachineTemplateSummaryOpts()...,
	)
	if err != nil {
		return err
	}

	conditions.Set(hcloudMachineTemplate, *readyCondition)
	return nil
}

// MachineTemplatePatchOpts returns the list of patch.Option for HCloudMachineTemplate,
// declaring both the conditions and the deprecated v1beta1 conditions owned by this controller so
// the patch helper handles three-way merge correctly across concurrent updates.
func MachineTemplatePatchOpts() []patch.Option {
	return []patch.Option{
		// owned deprecated v1beta1 conditions.
		patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyV1Beta1Condition,
			infrav2.HCloudTokenAvailableV1Beta1Condition,
			infrav2.HetznerAPIReachableV1Beta1Condition,
		}},
		// owned conditions.
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav2.HCloudMachineTemplateAvailableCondition,
			infrav2.HCloudTokenAvailableCondition,
			infrav2.HCloudRateLimitExceededCondition,
		}},
	}
}
