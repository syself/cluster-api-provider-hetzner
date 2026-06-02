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

// HetznerBareMetalRemediationWebhook implements admission webhooks for HetznerBareMetalRemediation.
type HetznerBareMetalRemediationWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalRemediation.
func (webhook *HetznerBareMetalRemediationWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.HetznerBareMetalRemediation{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalremediation,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations,verbs=create;update,versions=v1beta1,name=mutation.hetznerbaremetalremediation.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Defaulter[*infrav1.HetznerBareMetalRemediation] = &HetznerBareMetalRemediationWebhook{}

// Default implements admission.Defaulter so a webhook will be registered for HetznerBareMetalRemediation.
func (*HetznerBareMetalRemediationWebhook) Default(context.Context, *infrav1.HetznerBareMetalRemediation) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalremediation,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalremediation.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*infrav1.HetznerBareMetalRemediation] = &HetznerBareMetalRemediationWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HetznerBareMetalRemediation.
func (*HetznerBareMetalRemediationWebhook) ValidateCreate(context.Context, *infrav1.HetznerBareMetalRemediation) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HetznerBareMetalRemediation.
func (*HetznerBareMetalRemediationWebhook) ValidateUpdate(_ context.Context, _, _ *infrav1.HetznerBareMetalRemediation) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HetznerBareMetalRemediation.
func (*HetznerBareMetalRemediationWebhook) ValidateDelete(context.Context, *infrav1.HetznerBareMetalRemediation) (admission.Warnings, error) {
	return nil, nil
}
