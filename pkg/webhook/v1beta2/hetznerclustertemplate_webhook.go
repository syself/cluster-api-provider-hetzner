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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// HetznerClusterTemplateWebhook implements admission webhooks for HetznerClusterTemplate.
type HetznerClusterTemplateWebhook struct{}

// log is for logging in this package.
var hetznerclustertemplatelog = utils.GetDefaultLogger("info").WithName("hetznerclustertemplate-resource")

// SetupWebhookWithManager initializes webhook manager for HetznerClusterTemplate.
func (webhook *HetznerClusterTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav2.HetznerClusterTemplate{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

var _ admission.Defaulter[*infrav2.HetznerClusterTemplate] = &HetznerClusterTemplateWebhook{}

// Default implements admission.Defaulter so a webhook will be registered for HetznerClusterTemplate.
func (*HetznerClusterTemplateWebhook) Default(context.Context, *infrav2.HetznerClusterTemplate) error {
	return nil
}

var _ admission.Validator[*infrav2.HetznerClusterTemplate] = &HetznerClusterTemplateWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HetznerClusterTemplate.
func (*HetznerClusterTemplateWebhook) ValidateCreate(context.Context, *infrav2.HetznerClusterTemplate) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HetznerClusterTemplate.
func (*HetznerClusterTemplateWebhook) ValidateUpdate(_ context.Context, old, r *infrav2.HetznerClusterTemplate) (admission.Warnings, error) {
	hetznerclustertemplatelog.V(1).Info("validate update", "name", r.Name)

	if !reflect.DeepEqual(r.Spec, old.Spec) {
		return nil, apierrors.NewBadRequest("HetznerClusterTemplate.Spec is immutable")
	}
	return nil, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HetznerClusterTemplate.
func (*HetznerClusterTemplateWebhook) ValidateDelete(context.Context, *infrav2.HetznerClusterTemplate) (admission.Warnings, error) {
	return nil, nil
}
