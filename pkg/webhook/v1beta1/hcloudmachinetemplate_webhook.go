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
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// HCloudMachineTemplateWebhook implements a custom validation webhook for HCloudMachineTemplate.
// +kubebuilder:object:generate=false
type HCloudMachineTemplateWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HCloudMachineTemplate.
func (webhook *HCloudMachineTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.HCloudMachineTemplate{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

var _ admission.Defaulter[*infrav1.HCloudMachineTemplate] = &HCloudMachineTemplateWebhook{}

// Default implements admission.CustomDefaulter.
func (*HCloudMachineTemplateWebhook) Default(context.Context, *infrav1.HCloudMachineTemplate) error {
	return nil
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachinetemplate,mutating=false,sideEffects=None,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachinetemplates,verbs=create;update,versions=v1beta1,name=validation.hcloudmachinetemplate.infrastructure.x-k8s.io,admissionReviewVersions=v1;v1beta1

var _ admission.Validator[*infrav1.HCloudMachineTemplate] = &HCloudMachineTemplateWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HCloudMachineTemplate.
func (*HCloudMachineTemplateWebhook) ValidateCreate(_ context.Context, r *infrav1.HCloudMachineTemplate) (admission.Warnings, error) {
	allErrs := validateHCloudMachineSpec(r.Spec.Template.Spec)
	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HCloudMachineTemplate.
func (*HCloudMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldHCloudMachineTemplate *infrav1.HCloudMachineTemplate, newHCloudMachineTemplate *infrav1.HCloudMachineTemplate) (admission.Warnings, error) {
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

// ValidateDelete implements admission.Validator so a webhook will be registered for HCloudMachineTemplate.
func (*HCloudMachineTemplateWebhook) ValidateDelete(context.Context, *infrav1.HCloudMachineTemplate) (admission.Warnings, error) {
	return nil, nil
}
