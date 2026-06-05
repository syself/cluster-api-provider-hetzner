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
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

// HetznerBareMetalHostWebhook implements validating and defaulting webhook for HetznerBareMetalHost.
// +k8s:deepcopy-gen=false
type HetznerBareMetalHostWebhook struct {
	Client client.Client
}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalHost.
func (webhook *HetznerBareMetalHostWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if webhook.Client == nil {
		webhook.Client = mgr.GetClient()
	}
	return ctrl.NewWebhookManagedBy(mgr, &infrav2.HetznerBareMetalHost{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

var _ admission.Defaulter[*infrav2.HetznerBareMetalHost] = &HetznerBareMetalHostWebhook{}

// Default implements admission.Defaulter so a webhook will be registered for HetznerBareMetalHost.
func (*HetznerBareMetalHostWebhook) Default(context.Context, *infrav2.HetznerBareMetalHost) error {
	return nil
}

var _ admission.Validator[*infrav2.HetznerBareMetalHost] = &HetznerBareMetalHostWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HetznerBareMetalHost.
func (webhook *HetznerBareMetalHostWebhook) ValidateCreate(ctx context.Context, host *infrav2.HetznerBareMetalHost) (admission.Warnings, error) {
	var allErrs field.ErrorList

	hetznerBareMetalHostList := &infrav2.HetznerBareMetalHostList{}
	if err := webhook.Client.List(ctx, hetznerBareMetalHostList, &client.ListOptions{}); err != nil {
		return admission.Warnings{fmt.Sprintf("could not verify that the host has a unique serverID: %s", err.Error())}, nil
	}

	for _, hetznerBareMetalHost := range hetznerBareMetalHostList.Items {
		if hetznerBareMetalHost.Spec.ServerID == host.Spec.ServerID {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "ServerID"), host.Spec.ServerID, fmt.Sprintf("%q host exist with same serverID: %d", hetznerBareMetalHost.Name, host.Spec.ServerID)),
			)
		}
	}

	return nil, aggregateObjErrors(hetznerBareMetalHostList.GroupVersionKind().GroupKind(), host.Name, allErrs)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HetznerBareMetalHost.
func (*HetznerBareMetalHostWebhook) ValidateUpdate(_ context.Context, oldHost, newHost *infrav2.HetznerBareMetalHost) (admission.Warnings, error) {
	var allErrs field.ErrorList

	if newHost.Spec.ServerID != oldHost.Spec.ServerID {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "serverID"), newHost.Spec.ServerID, "serverID is immutable"),
		)
	}

	return nil, aggregateObjErrors(newHost.GroupVersionKind().GroupKind(), newHost.Name, allErrs)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HetznerBareMetalHost.
func (*HetznerBareMetalHostWebhook) ValidateDelete(context.Context, *infrav2.HetznerBareMetalHost) (admission.Warnings, error) {
	return nil, nil
}
