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

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
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

// Reconcile implements reconcilement of HCloudMachinesTemplates.
func (s *Service) Reconcile(ctx context.Context) (reconcile.Result, error) {
	machineTemplate := s.scope.HCloudMachineTemplate

	if machineTemplate.Status.Capacity == nil {
		serverTypes, err := s.scope.HCloudClient.ListServerTypes(ctx)
		if err != nil {
			hcloudutil.HandleRateLimitExceededV1Beta1(machineTemplate, err, "ListServerTypes")
			err = fmt.Errorf("failed to list server types: %w", err)
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:    infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.InternalErrorV1Beta2Reason,
				Message: err.Error(),
			})
			return reconcile.Result{}, err
		}

		capacity, found, err := getCapacity(serverTypes, string(machineTemplate.Spec.Template.Spec.Type))
		if err != nil {
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:    infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.InternalErrorV1Beta2Reason,
				Message: err.Error(),
			})
			return reconcile.Result{}, fmt.Errorf("failed to get capacity: %w", err)
		}
		if !found {
			// wrong server type, not an internal error. don't retry with backoff, a restart
			// picks it up again if hcloud starts offering it.
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:    infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineTemplateServerTypeNotFoundV1Beta2Reason,
				Message: fmt.Sprintf("failed to find server type for %s", machineTemplate.Spec.Template.Spec.Type),
			})
			return reconcile.Result{}, nil
		}

		machineTemplate.Status.Capacity = capacity
	}

	v1beta2conditions.Set(machineTemplate, metav1.Condition{
		Type:   infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudMachineTemplateAvailableV1Beta2Reason,
	})
	return reconcile.Result{}, nil
}

// getCapacity finds wantType among serverTypes and returns its CPU cores and memory as a
// ResourceList. found is false if hcloud does not offer that server type.
func getCapacity(serverTypes []*hcloud.ServerType, wantType string) (capacity corev1.ResourceList, found bool, err error) {
	for _, serverType := range serverTypes {
		if serverType.Name != wantType {
			continue
		}

		capacity = make(corev1.ResourceList)
		cpu, err := GetCPUQuantityFromInt(serverType.Cores)
		if err != nil {
			return nil, false, fmt.Errorf("failed to parse quantity. CPU cores %v. Server type %+v: %w", serverType.Cores, serverType, err)
		}
		capacity[corev1.ResourceCPU] = cpu
		memory, err := GetMemoryQuantityFromFloat32(serverType.Memory)
		if err != nil {
			return nil, false, fmt.Errorf("failed to parse quantity. Memory %v. Server type %+v: %w", serverType.Memory, serverType, err)
		}
		capacity[corev1.ResourceMemory] = memory
		return capacity, true, nil
	}

	return nil, false, nil
}
