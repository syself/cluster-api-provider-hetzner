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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachine.
func (bmMachine *HetznerBareMetalMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(bmMachine).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=mutation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HetznerBareMetalMachine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (bmMachine *HetznerBareMetalMachine) Default() {}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HetznerBareMetalMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (bmMachine *HetznerBareMetalMachine) ValidateCreate() (admission.Warnings, error) {
	allErrs := validateHetznerBareMetalMachineSpecCreate(bmMachine.Spec)

	return nil, aggregateObjErrors(bmMachine.GroupVersionKind().GroupKind(), bmMachine.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (bmMachine *HetznerBareMetalMachine) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldHetznerBareMetalMachine := old.(*HetznerBareMetalMachine)

	allErrs := validateHetznerBareMetalMachineSpecUpdate(oldHetznerBareMetalMachine.Spec, bmMachine.Spec)

	return nil, aggregateObjErrors(bmMachine.GroupVersionKind().GroupKind(), bmMachine.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (bmMachine *HetznerBareMetalMachine) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
