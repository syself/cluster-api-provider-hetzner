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

// Package machinetemplate implements functions to manage the lifecycle of HCloud machine templates.
package machinetemplate

import (
	"context"
	"fmt"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
)

// Service defines struct with HCloudMachineTemplate scope to reconcile HCloud machine templates.
type Service struct {
	scope *scope.HCloudMachineTemplateScope
}

// NewService outs a new service with HCloudMachineTemplate scope.
func NewService(scope *scope.HCloudMachineTemplateScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HCloud machines.
func (s *Service) Reconcile(ctx context.Context) error {
	capacity, err := s.getCapacity(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get capacity")
	}

	s.scope.HCloudMachineTemplate.Status.Capacity = capacity
	return nil
}

func (s *Service) getCapacity(ctx context.Context) (corev1.ResourceList, error) {
	capacity := make(corev1.ResourceList)
	// List all server types
	serverTypes, err := s.scope.HCloudClient.ListServerTypes(ctx)
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HCloudMachineTemplate, infrav1.RateLimitExceeded)
			record.Event(s.scope.HCloudMachineTemplate,
				"RateLimitExceeded",
				"exceeded rate limit with calling hcloud function ListServerTypes",
			)
		}
		return nil, errors.Wrap(err, "failed to list server types")
	}

	// Find the correct server type and check number of CPU cores and GB of memory
	var foundServerType bool
	for _, serverType := range serverTypes {
		if serverType.Name != string(s.scope.HCloudMachineTemplate.Spec.Template.Spec.Type) {
			continue
		}

		foundServerType = true
		cpu, err := GetCPUQuantityFromInt(serverType.Cores)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quantity. CPU cores: %v. Server type: %+v", serverType.Cores, serverType)
		}
		capacity[corev1.ResourceCPU] = cpu
		memory, err := GetMemoryQuantityFromFloat32(serverType.Memory)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse quantity. Memory: %v. Server type: %+v", serverType.Memory, serverType)
		}
		capacity[corev1.ResourceMemory] = memory
	}
	if !foundServerType {
		return nil, fmt.Errorf("failed to find server type for %s", s.scope.HCloudMachineTemplate.Spec.Template.Spec.Type)
	}

	return capacity, nil
}
