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
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachine.
func (r *HetznerBareMetalMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=mutation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HetznerBareMetalMachine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *HetznerBareMetalMachine) Default() {
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HetznerBareMetalMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachine) ValidateCreate() error {
	var allErrs field.ErrorList

	if r.Spec.SSHSpec.PortAfterInstallImage == 0 {
		r.Spec.SSHSpec.PortAfterInstallImage = 22
	}
	if r.Spec.SSHSpec.PortAfterCloudInit == 0 {
		r.Spec.SSHSpec.PortAfterCloudInit = r.Spec.SSHSpec.PortAfterInstallImage
	}

	if (r.Spec.InstallImage.Image.Name == "" || r.Spec.InstallImage.Image.URL == "") &&
		r.Spec.InstallImage.Image.Path == "" {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "installImage", "image"), r.Spec.InstallImage.Image,
				"have to specify either image name and url or path"),
		)
	}

	if r.Spec.InstallImage.Image.URL != "" {
		if _, err := GetImageSuffix(r.Spec.InstallImage.Image.URL); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "installImage", "image", "url"), r.Spec.InstallImage.Image.URL,
					"unknown image type in URL"),
			)
		}
	}
	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachine) ValidateUpdate(old runtime.Object) error {
	var allErrs field.ErrorList

	oldHetznerBareMetalMachine := old.(*HetznerBareMetalMachine)
	if !reflect.DeepEqual(r.Spec.InstallImage, oldHetznerBareMetalMachine.Spec.InstallImage) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "installImage"), r.Spec.InstallImage, "installImage immutable"),
		)
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "sshSpec"), r.Spec.SSHSpec, "sshSpec immutable"),
		)
	}
	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachine) ValidateDelete() error {
	return nil
}
