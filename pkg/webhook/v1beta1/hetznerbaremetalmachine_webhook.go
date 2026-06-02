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

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// HetznerBareMetalMachineWebhook implements admission webhooks for HetznerBareMetalMachine.
type HetznerBareMetalMachineWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachine.
func (webhook *HetznerBareMetalMachineWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.HetznerBareMetalMachine{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=mutation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Defaulter[*infrav1.HetznerBareMetalMachine] = &HetznerBareMetalMachineWebhook{}

// Default implements admission.Defaulter so a webhook will be registered for HetznerBareMetalMachine.
func (*HetznerBareMetalMachineWebhook) Default(context.Context, *infrav1.HetznerBareMetalMachine) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*infrav1.HetznerBareMetalMachine] = &HetznerBareMetalMachineWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HetznerBareMetalMachine.
func (*HetznerBareMetalMachineWebhook) ValidateCreate(_ context.Context, r *infrav1.HetznerBareMetalMachine) (admission.Warnings, error) {
	allErrs := validateHetznerBareMetalMachineSpecCreate(r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HetznerBareMetalMachine.
func (*HetznerBareMetalMachineWebhook) ValidateUpdate(_ context.Context, oldHetznerBareMetalMachine *infrav1.HetznerBareMetalMachine, r *infrav1.HetznerBareMetalMachine) (admission.Warnings, error) {
	allErrs := validateHetznerBareMetalMachineSpecUpdate(oldHetznerBareMetalMachine.Spec, r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HetznerBareMetalMachine.
func (*HetznerBareMetalMachineWebhook) ValidateDelete(context.Context, *infrav1.HetznerBareMetalMachine) (admission.Warnings, error) {
	return nil, nil
}
