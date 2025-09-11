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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager initializes webhook manager for HCloudMachineTemplate.
func (r *HCloudMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hcloudMachineTemplateWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// HCloudMachineTemplateWebhook implements a custom validation webhook for HCloudMachineTemplate.
// +kubebuilder:object:generate=false
type hcloudMachineTemplateWebhook struct{}

// Default implements admission.CustomDefaulter.
func (*hcloudMachineTemplateWebhook) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachinetemplate,mutating=false,sideEffects=None,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachinetemplates,verbs=create;update,versions=v1beta1,name=validation.hcloudmachinetemplate.infrastructure.x-k8s.io,admissionReviewVersions=v1;v1beta1

var (
	_ webhook.CustomValidator = &hcloudMachineTemplateWebhook{}
	_ webhook.CustomDefaulter = &hcloudMachineTemplateWebhook{}
)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hcloudMachineTemplateWebhook) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hcloudMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldRaw runtime.Object, newRaw runtime.Object) (admission.Warnings, error) {
	newHCloudMachineTemplate, ok := newRaw.(*HCloudMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HCloudMachineTemplate but got a %T", newRaw))
	}
	oldHCloudMachineTemplate, ok := oldRaw.(*HCloudMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HCloudMachineTemplate but got a %T", oldRaw))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a admission.Request inside context: %v", err))
	}
	var allErrs field.ErrorList
	if !topology.ShouldSkipImmutabilityChecks(req, newHCloudMachineTemplate) && !reflect.DeepEqual(newHCloudMachineTemplate.Spec, oldHCloudMachineTemplate.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec"), newHCloudMachineTemplate, "HCloudMachineTemplate.Spec is immutable"))
	}

	return nil, aggregateObjErrors(newHCloudMachineTemplate.GroupVersionKind().GroupKind(), newHCloudMachineTemplate.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hcloudMachineTemplateWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
