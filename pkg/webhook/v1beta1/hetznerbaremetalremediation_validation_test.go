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

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func TestValidateBareMetalRemediationStrategy(t *testing.T) {
	strategyPath := field.NewPath("spec", "strategy")

	tests := []struct {
		name     string
		strategy *infrav1.BareMetalRemediationStrategy
		want     *field.Error
	}{
		{
			name:     "nil strategy is valid",
			strategy: nil,
		},
		{
			name:     "unset onExhaustion without retireConditions is valid",
			strategy: &infrav1.BareMetalRemediationStrategy{},
		},
		{
			name:     "Retire without retireConditions is valid",
			strategy: &infrav1.BareMetalRemediationStrategy{OnExhaustion: infrav1.OnExhaustionRetire},
		},
		{
			name: "RetireIfUnhealthyCondition with retireConditions is valid",
			strategy: &infrav1.BareMetalRemediationStrategy{
				OnExhaustion:     infrav1.OnExhaustionRetireIfUnhealthyCondition,
				RetireConditions: []corev1.NodeConditionType{"DisksFailure"},
			},
		},
		{
			name: "RetireIfUnhealthyCondition without retireConditions is rejected",
			strategy: &infrav1.BareMetalRemediationStrategy{
				OnExhaustion: infrav1.OnExhaustionRetireIfUnhealthyCondition,
			},
			want: field.Required(strategyPath.Child("retireConditions"),
				"must list at least one node condition when onExhaustion is RetireIfUnhealthyCondition"),
		},
		{
			name: "Retire with retireConditions is rejected",
			strategy: &infrav1.BareMetalRemediationStrategy{
				OnExhaustion:     infrav1.OnExhaustionRetire,
				RetireConditions: []corev1.NodeConditionType{"DisksFailure"},
			},
			want: field.Invalid(strategyPath.Child("retireConditions"),
				[]corev1.NodeConditionType{"DisksFailure"},
				"is only allowed when onExhaustion is RetireIfUnhealthyCondition"),
		},
		{
			name:     "Reuse without retireConditions is valid",
			strategy: &infrav1.BareMetalRemediationStrategy{OnExhaustion: infrav1.OnExhaustionReuse},
		},
		{
			name: "Reuse with retireConditions is rejected",
			strategy: &infrav1.BareMetalRemediationStrategy{
				OnExhaustion:     infrav1.OnExhaustionReuse,
				RetireConditions: []corev1.NodeConditionType{"DisksFailure"},
			},
			want: field.Invalid(strategyPath.Child("retireConditions"),
				[]corev1.NodeConditionType{"DisksFailure"},
				"is only allowed when onExhaustion is RetireIfUnhealthyCondition"),
		},
		{
			name: "unset onExhaustion with retireConditions is rejected",
			strategy: &infrav1.BareMetalRemediationStrategy{
				RetireConditions: []corev1.NodeConditionType{"DisksFailure"},
			},
			want: field.Invalid(strategyPath.Child("retireConditions"),
				[]corev1.NodeConditionType{"DisksFailure"},
				"is only allowed when onExhaustion is RetireIfUnhealthyCondition"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateBareMetalRemediationStrategy(tt.strategy, strategyPath)

			if tt.want == nil {
				assert.Empty(t, got)
				return
			}

			if assert.Len(t, got, 1) {
				assert.Equal(t, tt.want.Type, got[0].Type)
				assert.Equal(t, tt.want.Field, got[0].Field)
				assert.Equal(t, tt.want.Detail, got[0].Detail)
			}
		})
	}
}
