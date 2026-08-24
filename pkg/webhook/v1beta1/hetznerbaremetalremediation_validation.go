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
	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// validateBareMetalRemediationStrategy rejects a strategy whose OnExhaustion mode and RetireConditions list
// disagree: RetireIfUnhealthyCondition needs a non-empty list, and every other mode (including
// unset) must leave the list empty. A nil strategy returns no error.
func validateBareMetalRemediationStrategy(strategy *infrav1.BareMetalRemediationStrategy, fldPath *field.Path) field.ErrorList {
	if strategy == nil {
		return nil
	}
	if strategy.OnExhaustion == infrav1.OnExhaustionRetireIfUnhealthyCondition {
		if len(strategy.RetireConditions) == 0 {
			return field.ErrorList{field.Required(fldPath.Child("retireConditions"),
				"must list at least one node condition when onExhaustion is RetireIfUnhealthyCondition")}
		}
		return nil
	}
	if len(strategy.RetireConditions) > 0 {
		return field.ErrorList{field.Invalid(fldPath.Child("retireConditions"), strategy.RetireConditions,
			"is only allowed when onExhaustion is RetireIfUnhealthyCondition")}
	}
	return nil
}
