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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type hcloudRemediationWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for HCloudRemediation.
func (r *HCloudRemediation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hcloudRemediationWebhook)
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

var _ admission.Defaulter[*HCloudRemediation] = &hcloudRemediationWebhook{}

// Default implements admission.Defaulter[*HCloudRemediation] so a webhook will be registered for the type.
func (*hcloudRemediationWebhook) Default(context.Context, *HCloudRemediation) error {
	return nil
}

var _ admission.Validator[*HCloudRemediation] = &hcloudRemediationWebhook{}

// ValidateCreate implements admission.Validator[*HCloudRemediation] so a webhook will be registered for the type.
func (*hcloudRemediationWebhook) ValidateCreate(context.Context, *HCloudRemediation) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements admission.Validator[*HCloudRemediation] so a webhook will be registered for the type.
func (*hcloudRemediationWebhook) ValidateUpdate(_ context.Context, _, _ *HCloudRemediation) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements admission.Validator[*HCloudRemediation] so a webhook will be registered for the type.
func (*hcloudRemediationWebhook) ValidateDelete(context.Context, *HCloudRemediation) (admission.Warnings, error) {
	return nil, nil
}
