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
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager initializes webhook manager for HCloudMachineTemplate.
func (r *HCloudMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hcloudMachineTemplateWebhook)
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// HCloudMachineTemplateWebhook implements a custom validation webhook for HCloudMachineTemplate.
// +kubebuilder:object:generate=false
type hcloudMachineTemplateWebhook struct{}

var _ admission.Defaulter[*HCloudMachineTemplate] = &hcloudMachineTemplateWebhook{}

// Default implements admission.CustomDefaulter.
func (*hcloudMachineTemplateWebhook) Default(context.Context, *HCloudMachineTemplate) error {
	return nil
}

var _ admission.Validator[*HCloudMachineTemplate] = &hcloudMachineTemplateWebhook{}

// ValidateCreate implements admission.Validator[*HCloudMachineTemplate] so a webhook will be registered for the type.
func (*hcloudMachineTemplateWebhook) ValidateCreate(_ context.Context, r *HCloudMachineTemplate) (admission.Warnings, error) {
	allErrs := validateHCloudMachineSpec(r.Spec.Template.Spec)
	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements admission.Validator[*HCloudMachineTemplate] so a webhook will be registered for the type.
func (*hcloudMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldHCloudMachineTemplate *HCloudMachineTemplate, newHCloudMachineTemplate *HCloudMachineTemplate) (admission.Warnings, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a admission.Request inside context: %v", err))
	}
	var allErrs field.ErrorList
	if !topology.IsDryRunRequest(req, newHCloudMachineTemplate) && !reflect.DeepEqual(newHCloudMachineTemplate.Spec, oldHCloudMachineTemplate.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec"), newHCloudMachineTemplate, "HCloudMachineTemplate.Spec is immutable"))
	}
	allErrs = append(allErrs, validateHCloudMachineSpec(newHCloudMachineTemplate.Spec.Template.Spec)...)

	return nil, aggregateObjErrors(newHCloudMachineTemplate.GroupVersionKind().GroupKind(), newHCloudMachineTemplate.Name, allErrs)
}

// ValidateDelete implements admission.Validator[*HCloudMachineTemplate] so a webhook will be registered for the type.
func (*hcloudMachineTemplateWebhook) ValidateDelete(context.Context, *HCloudMachineTemplate) (admission.Warnings, error) {
	return nil, nil
}
