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

package v1beta2

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type hetznerBareMetalRemediationWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalRemediation.
func (r *HetznerBareMetalRemediation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hetznerBareMetalRemediationWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-hetznerbaremetalremediation,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations,verbs=create;update,versions=v1beta2,name=mutation.hetznerbaremetalremediation.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta2}

var _ webhook.CustomDefaulter = &hetznerBareMetalRemediationWebhook{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationWebhook) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-hetznerbaremetalremediation,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations,verbs=create;update,versions=v1beta2,name=validation.hetznerbaremetalremediation.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta2}

var _ webhook.CustomValidator = &hetznerBareMetalRemediationWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationWebhook) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationWebhook) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hetznerBareMetalRemediationWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
