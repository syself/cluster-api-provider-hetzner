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
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

func validateHetznerBareMetalMachineSpecCreate(spec infrav2.HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList

	// installImage and customProvisioner are the two mutually exclusive provisioning flows.
	switch {
	case spec.InstallImage == nil && spec.CustomProvisioner == nil:
		allErrs = append(allErrs,
			field.Required(field.NewPath("spec"), "either installImage or customProvisioner must be set"),
		)
	case spec.InstallImage != nil && spec.CustomProvisioner != nil:
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "customProvisioner"),
				"installImage and customProvisioner are mutually exclusive"),
		)
	case spec.InstallImage != nil:
		allErrs = append(allErrs, validateInstallImage(*spec.InstallImage)...)
	case spec.CustomProvisioner != nil:
		allErrs = append(allErrs, validateCustomProvisioner(*spec.CustomProvisioner)...)
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

func validateInstallImage(installImage infrav2.InstallImage) field.ErrorList {
	var allErrs field.ErrorList
	base := field.NewPath("spec", "installImage")
	image := installImage.Image

	if (image.Name == "" || image.URL == "") && image.Path == "" {
		allErrs = append(allErrs,
			field.Invalid(base.Child("image"), image, "have to specify either image name and url or path"),
		)
	}

	if image.URL != "" {
		if _, err := infrav2.GetImageSuffix(image.URL); err != nil {
			allErrs = append(allErrs,
				field.Invalid(base.Child("image", "url"), image.URL, "unknown image type in URL"),
			)
		}
	}

	return allErrs
}

func validateCustomProvisioner(customProvisioner infrav2.CustomProvisioner) field.ErrorList {
	var allErrs field.ErrorList
	base := field.NewPath("spec", "customProvisioner")

	if customProvisioner.URL == "" {
		allErrs = append(allErrs, field.Required(base.Child("url"), "url is required"))
	} else if _, err := url.ParseRequestURI(customProvisioner.URL); err != nil {
		allErrs = append(allErrs, field.Invalid(base.Child("url"), customProvisioner.URL, err.Error()))
	}

	if customProvisioner.Command == "" {
		allErrs = append(allErrs, field.Required(base.Child("command"), "command is required"))
	} else if err := utils.ValidateImageURLCommandName(customProvisioner.Command); err != nil {
		allErrs = append(allErrs, field.Invalid(base.Child("command"), customProvisioner.Command, err.Error()))
	}

	return allErrs
}

func validateHetznerBareMetalMachineSpecUpdate(oldSpec, newSpec infrav2.HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	if !reflect.DeepEqual(newSpec.InstallImage, oldSpec.InstallImage) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "installImage"), "installImage is immutable"),
		)
	}
	if !reflect.DeepEqual(newSpec.CustomProvisioner, oldSpec.CustomProvisioner) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "customProvisioner"), "customProvisioner is immutable"),
		)
	}
	if !reflect.DeepEqual(newSpec.SSHSpec, oldSpec.SSHSpec) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "sshSpec"), "sshSpec is immutable"),
		)
	}
	if !reflect.DeepEqual(newSpec.HostSelector, oldSpec.HostSelector) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "hostSelector"), "hostSelector is immutable"),
		)
	}

	if oldSpec.ProviderID != nil && *oldSpec.ProviderID != "" {
		// once the ProviderID was set, the value must not change.
		if newSpec.ProviderID == nil || *oldSpec.ProviderID != *newSpec.ProviderID {
			allErrs = append(allErrs,
				field.Forbidden(field.NewPath("spec", "providerID"), "providerID is immutable"),
			)
		}
	}

	return allErrs
}
