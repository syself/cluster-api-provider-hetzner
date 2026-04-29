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
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type hetznerBareMetalMachineWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachine.
func (r *HetznerBareMetalMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hetznerBareMetalMachineWebhook)
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=mutation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Defaulter[*HetznerBareMetalMachine] = &hetznerBareMetalMachineWebhook{}

// Default implements admission.Defaulter[*HetznerBareMetalMachine] so a webhook will be registered for the type.
func (*hetznerBareMetalMachineWebhook) Default(context.Context, *HetznerBareMetalMachine) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*HetznerBareMetalMachine] = &hetznerBareMetalMachineWebhook{}

// ValidateCreate implements admission.Validator[*HetznerBareMetalMachine] so a webhook will be registered for the type.
func (*hetznerBareMetalMachineWebhook) ValidateCreate(_ context.Context, r *HetznerBareMetalMachine) (admission.Warnings, error) {
	allErrs := validateHetznerBareMetalMachineSpecCreate(r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements admission.Validator[*HetznerBareMetalMachine] so a webhook will be registered for the type.
func (*hetznerBareMetalMachineWebhook) ValidateUpdate(_ context.Context, oldHetznerBareMetalMachine *HetznerBareMetalMachine, r *HetznerBareMetalMachine) (admission.Warnings, error) {
	allErrs := validateHetznerBareMetalMachineSpecUpdate(oldHetznerBareMetalMachine.Spec, r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements admission.Validator[*HetznerBareMetalMachine] so a webhook will be registered for the type.
func (*hetznerBareMetalMachineWebhook) ValidateDelete(context.Context, *HetznerBareMetalMachine) (admission.Warnings, error) {
	return nil, nil
}
