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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type hetznerBareMetalRemediationTemplateWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalRemediationTemplate.
func (r *HetznerBareMetalRemediationTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hetznerBareMetalRemediationTemplateWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalremediationtemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediationtemplates,verbs=create;update,versions=v1beta1,name=mhetznerbaremetalremediationtemplate.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &hetznerBareMetalRemediationTemplateWebhook{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationTemplateWebhook) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalremediationtemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediationtemplates,verbs=create;update,versions=v1beta1,name=vhetznerbaremetalremediationtemplate.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &hetznerBareMetalRemediationTemplateWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationTemplateWebhook) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationTemplateWebhook) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationTemplateWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
