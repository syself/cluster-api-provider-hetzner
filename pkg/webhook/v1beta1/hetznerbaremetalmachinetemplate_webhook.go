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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// HetznerBareMetalMachineTemplateWebhook implements a custom validation webhook for HetznerBareMetalMachineTemplate.
// +kubebuilder:object:generate=false
type HetznerBareMetalMachineTemplateWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachineTemplate.
func (webhook *HetznerBareMetalMachineTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.HetznerBareMetalMachineTemplate{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

var _ admission.Defaulter[*infrav1.HetznerBareMetalMachineTemplate] = &HetznerBareMetalMachineTemplateWebhook{}

// Default implements admission.CustomDefaulter.
func (*HetznerBareMetalMachineTemplateWebhook) Default(context.Context, *infrav1.HetznerBareMetalMachineTemplate) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachinetemplate,mutating=false,sideEffects=None,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachinetemplates,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachinetemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*infrav1.HetznerBareMetalMachineTemplate] = &HetznerBareMetalMachineTemplateWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HetznerBareMetalMachineTemplate.
func (*HetznerBareMetalMachineTemplateWebhook) ValidateCreate(context.Context, *infrav1.HetznerBareMetalMachineTemplate) (admission.Warnings, error) {
	// TODO: Cannot validate it because ClusterClass applies empty template objects
	// allErrs := validateHetznerBareMetalMachineSpecCreate(hbmmt.Spec.Template.Spec)
	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HetznerBareMetalMachineTemplate.
func (*HetznerBareMetalMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldHetznerBareMetalMachineTemplate *infrav1.HetznerBareMetalMachineTemplate, newHetznerBareMetalMachineTemplate *infrav1.HetznerBareMetalMachineTemplate) (admission.Warnings, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a admission.Request inside context: %v", err))
	}
	var allErrs field.ErrorList

	if !topology.IsDryRunRequest(req, newHetznerBareMetalMachineTemplate) && !reflect.DeepEqual(newHetznerBareMetalMachineTemplate.Spec, oldHetznerBareMetalMachineTemplate.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec"), newHetznerBareMetalMachineTemplate, "HetznerBareMetalMachineTemplate.Spec is immutable"))
	}

	return nil, aggregateObjErrors(newHetznerBareMetalMachineTemplate.GroupVersionKind().GroupKind(), newHetznerBareMetalMachineTemplate.Name, allErrs)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HetznerBareMetalMachineTemplate.
func (*HetznerBareMetalMachineTemplateWebhook) ValidateDelete(context.Context, *infrav1.HetznerBareMetalMachineTemplate) (admission.Warnings, error) {
	return nil, nil
}
