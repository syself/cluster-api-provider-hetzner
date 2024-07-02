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
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateHetznerBareMetalMachineSpecCreate(spec HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList

	if spec.SSHSpec.PortAfterCloudInit == 0 {
		spec.SSHSpec.PortAfterCloudInit = spec.SSHSpec.PortAfterInstallImage
	}

	if (spec.InstallImage.Image.Name == "" || spec.InstallImage.Image.URL == "") &&
		spec.InstallImage.Image.Path == "" {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "installImage", "image"), spec.InstallImage.Image,
				"have to specify either image name and url or path"),
		)
	}

	if spec.InstallImage.Image.URL != "" {
		if _, err := GetImageSuffix(spec.InstallImage.Image.URL); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "installImage", "image", "url"), spec.InstallImage.Image.URL,
					"unknown image type in URL"),
			)
		}
	}

	// validate host selector
	for labelKey, labelVal := range spec.HostSelector.MatchLabels {
		if _, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal}); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "hostSelector", "matchLabels"), spec.HostSelector.MatchLabels,
				fmt.Sprintf("invalid match label: %s", err.Error()),
			))
		}
	}
	for _, req := range spec.HostSelector.MatchExpressions {
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		if _, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "hostSelector", "matchExpressions"), spec.HostSelector.MatchExpressions,
				fmt.Sprintf("invalid match expression: %s", err.Error()),
			))
		}
	}

	return allErrs
}

func validateHetznerBareMetalMachineSpecUpdate(oldSpec, newSpec HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	if !reflect.DeepEqual(newSpec.InstallImage, oldSpec.InstallImage) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "installImage"), newSpec.InstallImage, "installImage immutable"),
		)
	}
	if !reflect.DeepEqual(newSpec.SSHSpec, oldSpec.SSHSpec) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "sshSpec"), newSpec.SSHSpec, "sshSpec immutable"),
		)
	}
	if !reflect.DeepEqual(newSpec.HostSelector, oldSpec.HostSelector) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "hostSelector"), newSpec.HostSelector, "hostSelector immutable"),
		)
	}

	return allErrs
}
