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

package v1beta2

import (
	"net/url"
	"reflect"

	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

func validateHCloudMachineSpecUpdate(oldSpec, newSpec infrav2.HCloudMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	// Type is immutable
	if !reflect.DeepEqual(oldSpec.Type, newSpec.Type) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "type"), "field is immutable"),
		)
	}

	// ImageName is immutable
	if !reflect.DeepEqual(oldSpec.ImageName, newSpec.ImageName) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "imageName"), "field is immutable"),
		)
	}

	// CustomProvisioner is immutable
	if !reflect.DeepEqual(oldSpec.CustomProvisioner, newSpec.CustomProvisioner) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "customProvisioner"), "field is immutable"),
		)
	}

	// SSHKeys is immutable
	if !reflect.DeepEqual(oldSpec.SSHKeys, newSpec.SSHKeys) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "sshKeys"), "field is immutable"),
		)
	}

	// Placement group name is immutable
	if !reflect.DeepEqual(oldSpec.PlacementGroupName, newSpec.PlacementGroupName) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "placementGroupName"), "field is immutable"),
		)
	}

	allErrs = append(allErrs, validateHCloudMachineSpec(newSpec)...)

	return allErrs
}

func validateHCloudMachineSpec(spec infrav2.HCloudMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	if spec.ImageName != "" && spec.CustomProvisioner != nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "imageName"), spec.ImageName, "imageName and customProvisioner are mutually exclusive"))
	}

	if spec.ImageName == "" && spec.CustomProvisioner == nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "imageName"), spec.ImageName, "imageName and customProvisioner empty. One of these attributes must be set"))
	}

	if spec.CustomProvisioner != nil {
		allErrs = append(allErrs, validateHCloudCustomProvisioner(*spec.CustomProvisioner)...)
	}

	return allErrs
}

func validateHCloudCustomProvisioner(customProvisioner infrav2.HCloudCustomProvisioner) field.ErrorList {
	var allErrs field.ErrorList
	base := field.NewPath("spec", "customProvisioner")

	// url and command are required and non-empty by the CRD schema, so this only checks their format.
	if _, err := url.ParseRequestURI(customProvisioner.URL); err != nil {
		allErrs = append(allErrs, field.Invalid(base.Child("url"), customProvisioner.URL, err.Error()))
	}

	// Intentionally validate only the name here. Checking whether the file exists on the
	// controller pod would make kubectl apply depend on the current controller filesystem state.
	if err := utils.ValidateCustomProvisionerCommandName(customProvisioner.Command); err != nil {
		allErrs = append(allErrs, field.Invalid(base.Child("command"), customProvisioner.Command, err.Error()))
	}

	return allErrs
}
