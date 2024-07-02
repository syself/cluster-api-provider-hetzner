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
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachineTemplate.
func (r *HetznerBareMetalMachineTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&HetznerBareMetalMachineTemplate{}).
		WithValidator(r).
		Complete()
}

// HetznerBareMetalMachineTemplateWebhook implements a custom validation webhook for HetznerBareMetalMachineTemplate.
// +kubebuilder:object:generate=false
type HetznerBareMetalMachineTemplateWebhook struct{}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachinetemplate,mutating=false,sideEffects=None,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachinetemplates,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachinetemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &HetznerBareMetalMachineTemplateWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachineTemplateWebhook) ValidateCreate(_ context.Context, raw runtime.Object) (admission.Warnings, error) {
	hbmmt, ok := raw.(*HetznerBareMetalMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerBareMetalMachineTemplate but got a %T", raw))
	}

	if hbmmt.Spec.Template.Spec.SSHSpec.PortAfterCloudInit == 0 {
		hbmmt.Spec.Template.Spec.SSHSpec.PortAfterCloudInit = hbmmt.Spec.Template.Spec.SSHSpec.PortAfterInstallImage
	}

	allErrs := validateHetznerBareMetalMachineSpecCreate(hbmmt.Spec.Template.Spec)

	return nil, aggregateObjErrors(hbmmt.GroupVersionKind().GroupKind(), hbmmt.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldRaw runtime.Object, newRaw runtime.Object) (admission.Warnings, error) {
	newHetznerBareMetalMachineTemplate, ok := newRaw.(*HetznerBareMetalMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerBareMetalMachineTemplate but got a %T", newRaw))
	}
	oldHetznerBareMetalMachineTemplate, ok := oldRaw.(*HetznerBareMetalMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerBareMetalMachineTemplate but got a %T", oldRaw))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a admission.Request inside context: %v", err))
	}

	allErrs := validateHetznerBareMetalMachineSpecUpdate(oldHetznerBareMetalMachineTemplate.Spec.Template.Spec, newHetznerBareMetalMachineTemplate.Spec.Template.Spec)

	if !topology.ShouldSkipImmutabilityChecks(req, newHetznerBareMetalMachineTemplate) && !reflect.DeepEqual(newHetznerBareMetalMachineTemplate.Spec, oldHetznerBareMetalMachineTemplate.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec"), newHetznerBareMetalMachineTemplate, "HetznerBareMetalMachineTemplate.Spec is immutable"))
	}

	return nil, aggregateObjErrors(newHetznerBareMetalMachineTemplate.GroupVersionKind().GroupKind(), newHetznerBareMetalMachineTemplate.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachineTemplateWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
