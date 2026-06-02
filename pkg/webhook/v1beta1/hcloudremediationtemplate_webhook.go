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

// HCloudRemediationTemplateWebhook implements admission webhooks for HCloudRemediationTemplate.
type HCloudRemediationTemplateWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HCloudRemediationTemplate.
func (webhook *HCloudRemediationTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.HCloudRemediationTemplate{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudremediationtemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudremediationtemplates,verbs=create;update,versions=v1beta1,name=mhcloudremediationtemplate.kb.io,admissionReviewVersions=v1

var _ admission.Defaulter[*infrav1.HCloudRemediationTemplate] = &HCloudRemediationTemplateWebhook{}

// Default implements admission.Defaulter so a webhook will be registered for HCloudRemediationTemplate.
func (*HCloudRemediationTemplateWebhook) Default(context.Context, *infrav1.HCloudRemediationTemplate) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudremediationtemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudremediationtemplates,verbs=create;update,versions=v1beta1,name=vhcloudremediationtemplate.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*infrav1.HCloudRemediationTemplate] = &HCloudRemediationTemplateWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HCloudRemediationTemplate.
func (*HCloudRemediationTemplateWebhook) ValidateCreate(context.Context, *infrav1.HCloudRemediationTemplate) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HCloudRemediationTemplate.
func (*HCloudRemediationTemplateWebhook) ValidateUpdate(_ context.Context, _, _ *infrav1.HCloudRemediationTemplate) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HCloudRemediationTemplate.
func (*HCloudRemediationTemplateWebhook) ValidateDelete(context.Context, *infrav1.HCloudRemediationTemplate) (admission.Warnings, error) {
	return nil, nil
}
