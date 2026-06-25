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
	"net/url"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

func validateHetznerBareMetalMachineSpecCreate(spec infrav1.HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	installImage := spec.InstallImage
	image := installImage.Image

	if installImage.UsesImageURLCommand() {
		if image.URL == "" {
			allErrs = append(allErrs,
				field.Required(field.NewPath("spec", "installImage", "image", "url"),
					"url is required when imageURLCommand is set"),
			)
		} else if _, err := url.ParseRequestURI(image.URL); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "installImage", "image", "url"), image.URL, err.Error()),
			)
		}

		if image.Name != "" {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "installImage", "image", "name"), image.Name,
					"name must be empty when imageURLCommand is set"),
			)
		}

		if image.Path != "" {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "installImage", "image", "path"), image.Path,
					"path must be empty when imageURLCommand is set"),
			)
		}

		if err := utils.ValidateImageURLCommandName(installImage.ImageURLCommand); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "installImage", "imageURLCommand"), installImage.ImageURLCommand,
					err.Error()),
			)
		}

	} else {
		if (image.Name == "" || image.URL == "") && image.Path == "" {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "installImage", "image"), image,
					"have to specify either image name and url or path"),
			)
		}

		if image.URL != "" {
			if _, err := infrav1.GetImageSuffix(image.URL); err != nil {
				allErrs = append(allErrs,
					field.Invalid(field.NewPath("spec", "installImage", "image", "url"), image.URL,
						"unknown image type in URL"),
				)
			}
		}

		if len(installImage.Partitions) == 0 {
			allErrs = append(allErrs,
				field.Required(field.NewPath("spec", "installImage", "partitions"),
					"partitions must be set when imageURLCommand is not set"),
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

func validateHetznerBareMetalMachineSpecUpdate(oldSpec, newSpec infrav1.HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	if !reflect.DeepEqual(newSpec.InstallImage, oldSpec.InstallImage) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "installImage"), "installImage is immutable"),
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
