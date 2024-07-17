/*
Copyright 2021 The Kubernetes Authors.

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
	"reflect"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateHCloudMachineSpec(oldSpec, newSpec HCloudMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	// Type is immutable
	if !reflect.DeepEqual(oldSpec.Type, newSpec.Type) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "type"), newSpec.Type, "field is immutable"),
		)
	}

	// ImageName is immutable
	if !reflect.DeepEqual(oldSpec.ImageName, newSpec.ImageName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "imageName"), newSpec.ImageName, "field is immutable"),
		)
	}

	// SSHKeys is immutable
	if !reflect.DeepEqual(oldSpec.SSHKeys, newSpec.SSHKeys) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "sshKeys"), newSpec.SSHKeys, "field is immutable"),
		)
	}

	// Placement group name is immutable
	if !reflect.DeepEqual(oldSpec.PlacementGroupName, newSpec.PlacementGroupName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "placementGroupName"), newSpec.PlacementGroupName, "field is immutable"),
		)
	}

	return allErrs
}
