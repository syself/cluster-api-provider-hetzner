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
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2/textlogger"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// HCloudMachineTemplateScopeParams defines the input parameters used to create a new scope.
type HCloudMachineTemplateScopeParams struct {
	Client                client.Client
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

	helper, err := patch.NewHelper(params.HCloudMachineTemplate, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &HCloudMachineTemplateScope{
		Logger:                params.Logger,
		Client:                params.Client,
		HCloudMachineTemplate: params.HCloudMachineTemplate,
		HCloudClient:          params.HCloudClient,
		patchHelper:           helper,
	}, nil
}

// HCloudMachineTemplateScope defines the basic context for an actuator to operate upon.
type HCloudMachineTemplateScope struct {
	*logr.Logger
	Client       client.Client
	patchHelper  *patch.Helper
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

// Close closes the current scope persisting the cluster configuration and status.
func (s *HCloudMachineTemplateScope) Close(ctx context.Context) error {
	conditions.SetSummary(s.HCloudMachineTemplate)
	return s.patchHelper.Patch(ctx, s.HCloudMachineTemplate)
}

// PatchObject persists the machine spec and status.
func (s *HCloudMachineTemplateScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.HCloudMachineTemplate)
}
