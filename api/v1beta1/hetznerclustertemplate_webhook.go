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
	"fmt"
	"reflect"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hetznerclustertemplatelog = utils.GetDefaultLogger("info").WithName("hetznerclustertemplate-resource")

// SetupWebhookWithManager initializes webhook manager for HetznerClusterTemplate.
func (r *HetznerClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerclustertemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclustertemplates,verbs=create;update,versions=v1beta1,name=mutation.hetznerclustertemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}
var _ webhook.Defaulter = &HetznerClusterTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *HetznerClusterTemplate) Default() {
	hetznerclustertemplatelog.V(1).Info("default", "name", r.Name)
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerclustertemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclustertemplates,verbs=create;update,versions=v1beta1,name=validation.hetznerclustertemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HetznerClusterTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerClusterTemplate) ValidateCreate() error {
	hetznerclustertemplatelog.V(1).Info("validate create", "name", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerClusterTemplate) ValidateUpdate(oldRaw runtime.Object) error {
	hetznerclustertemplatelog.V(1).Info("validate update", "name", r.Name)
	old, ok := oldRaw.(*HetznerClusterTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an HetznerClusterTemplate but got a %T", oldRaw))
	}

	if !reflect.DeepEqual(r.Spec, old.Spec) {
		return apierrors.NewBadRequest("HetznerClusterTemplate.Spec is immutable")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerClusterTemplate) ValidateDelete() error {
	hetznerclustertemplatelog.V(1).Info("validate delete", "name", r.Name)
	return nil
}
