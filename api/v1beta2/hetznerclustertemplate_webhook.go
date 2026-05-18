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

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

type hetznerClusterTemplateWebhook struct{}

// log is for logging in this package.
var hetznerclustertemplatelog = utils.GetDefaultLogger("info").WithName("hetznerclustertemplate-resource")

// SetupWebhookWithManager initializes webhook manager for HetznerClusterTemplate.
func (r *HetznerClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hetznerClusterTemplateWebhook)
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerclustertemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclustertemplates,verbs=create;update,versions=v1beta1,name=mutation.hetznerclustertemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}
var _ admission.Defaulter[*HetznerClusterTemplate] = &hetznerClusterTemplateWebhook{}

// Default implements admission.Defaulter[*HetznerClusterTemplate] so a webhook will be registered for the type.
func (*hetznerClusterTemplateWebhook) Default(context.Context, *HetznerClusterTemplate) error {
	return nil
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerclustertemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclustertemplates,verbs=create;update,versions=v1beta1,name=validation.hetznerclustertemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*HetznerClusterTemplate] = &hetznerClusterTemplateWebhook{}

// ValidateCreate implements admission.Validator[*HetznerClusterTemplate] so a webhook will be registered for the type.
func (*hetznerClusterTemplateWebhook) ValidateCreate(context.Context, *HetznerClusterTemplate) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements admission.Validator[*HetznerClusterTemplate] so a webhook will be registered for the type.
func (*hetznerClusterTemplateWebhook) ValidateUpdate(_ context.Context, old, r *HetznerClusterTemplate) (admission.Warnings, error) {
	hetznerclustertemplatelog.V(1).Info("validate update", "name", r.Name)

	if !reflect.DeepEqual(r.Spec, old.Spec) {
		return nil, apierrors.NewBadRequest("HetznerClusterTemplate.Spec is immutable")
	}
	return nil, nil
}

// ValidateDelete implements admission.Validator[*HetznerClusterTemplate] so a webhook will be registered for the type.
func (*hetznerClusterTemplateWebhook) ValidateDelete(context.Context, *HetznerClusterTemplate) (admission.Warnings, error) {
	return nil, nil
}
