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
	"k8s.io/utils/ptr"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

func validateHetznerBareMetalMachineSpecCreate(spec infrav2.HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList

	// installImage and customProvisioner are the two mutually exclusive provisioning flows.
	switch {
	case !spec.InstallImage.IsDefined() && !spec.CustomProvisioner.IsDefined():
		allErrs = append(allErrs,
			field.Required(field.NewPath("spec"), "either installImage or customProvisioner must be set"),
		)
	case spec.InstallImage.IsDefined() && spec.CustomProvisioner.IsDefined():
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "customProvisioner"),
				"installImage and customProvisioner are mutually exclusive"),
		)
	case spec.InstallImage.IsDefined():
		allErrs = append(allErrs, validateInstallImage(spec.InstallImage)...)
	case spec.CustomProvisioner.IsDefined():
		allErrs = append(allErrs, validateCustomProvisioner(spec.CustomProvisioner)...)
	}

	// validate host selector
	hostSelector := ptr.Deref(spec.HostSelector, infrav2.HostSelector{})
	for labelKey, labelVal := range hostSelector.MatchLabels {
		if _, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal}); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "hostSelector", "matchLabels"), hostSelector.MatchLabels,
				fmt.Sprintf("invalid match label: %s", err.Error()),
			))
		}
	}
	for _, req := range hostSelector.MatchExpressions {
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		if _, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "hostSelector", "matchExpressions"), hostSelector.MatchExpressions,
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

	// url and command are required and non-empty by the CRD schema, so this only checks their format:
	// url must be a valid URI, and command a valid basename.
	if _, err := url.ParseRequestURI(customProvisioner.URL); err != nil {
		allErrs = append(allErrs, field.Invalid(base.Child("url"), customProvisioner.URL, err.Error()))
	}

	if err := utils.ValidateCustomProvisionerCommandName(customProvisioner.Command); err != nil {
		allErrs = append(allErrs, field.Invalid(base.Child("command"), customProvisioner.Command, err.Error()))
	}

	return allErrs
}

func validateHetznerBareMetalMachineSpecUpdate(oldSpec, newSpec infrav2.HetznerBareMetalMachineSpec) field.ErrorList {
	var allErrs field.ErrorList
	oldSpec = normalizeHetznerBareMetalMachineSpec(oldSpec)
	newSpec = normalizeHetznerBareMetalMachineSpec(newSpec)

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

	if oldSpec.ProviderID != "" {
		// once the ProviderID was set, the value must not change.
		if oldSpec.ProviderID != newSpec.ProviderID {
			allErrs = append(allErrs,
				field.Forbidden(field.NewPath("spec", "providerID"), "providerID is immutable"),
			)
		}
	}

	return allErrs
}

// normalizeHetznerBareMetalMachineSpec returns a copy of spec in which every unset field gets the value
// that an unset field means. A stored object and an update request can differ in which unset fields
// they send. Without this, such an update looks like a change of an immutable field.
func normalizeHetznerBareMetalMachineSpec(spec infrav2.HetznerBareMetalMachineSpec) infrav2.HetznerBareMetalMachineSpec {
	if spec.InstallImage.Swraid == nil {
		spec.InstallImage.Swraid = ptr.To(infrav2.DefaultSwraid)
	}
	if spec.InstallImage.SwraidLevel == nil {
		spec.InstallImage.SwraidLevel = ptr.To(infrav2.DefaultSwraidLevel)
	}
	if spec.CustomProvisioner.Swraid == nil {
		spec.CustomProvisioner.Swraid = ptr.To(infrav2.DefaultSwraid)
	}
	if spec.HostSelector == nil {
		spec.HostSelector = &infrav2.HostSelector{}
	}
	if spec.SSHSpec.NoSSHAfterInstallImage == nil {
		spec.SSHSpec.NoSSHAfterInstallImage = ptr.To(false)
	}
	if spec.SSHSpec.PortAfterInstallImage == nil {
		spec.SSHSpec.PortAfterInstallImage = ptr.To(infrav2.DefaultPortAfterInstallImage)
	}
	if spec.SSHSpec.PortAfterCloudInit == nil { //nolint:staticcheck // the deprecated field is still part of the immutable sshSpec
		spec.SSHSpec.PortAfterCloudInit = ptr.To[int32](0) //nolint:staticcheck // the deprecated field is still part of the immutable sshSpec
	}
	if spec.SkipCheckDisk == nil {
		spec.SkipCheckDisk = ptr.To(false)
	}
	return spec
}
