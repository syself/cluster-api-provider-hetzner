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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachineTemplate.
func (r *HetznerBareMetalMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachinetemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachinetemplates,verbs=create;update,versions=v1beta1,name=mutation.hetznerbaremetalmachinetemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HetznerBareMetalMachineTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *HetznerBareMetalMachineTemplate) Default() {
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachinetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachinetemplates,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachinetemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HetznerBareMetalMachineTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachineTemplate) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachineTemplate) ValidateUpdate(old runtime.Object) error {
	oldHetznerBareMetalMachineTemplate := old.(*HetznerBareMetalMachineTemplate)
	if !reflect.DeepEqual(r.Spec, oldHetznerBareMetalMachineTemplate.Spec) {
		hcloudmachinetemplatelog.Info("not equal", "new HetznerBareMetalMachineTemplateSpec", r.Spec, "old HetznerBareMetalMachineTemplateSpec", oldHetznerBareMetalMachineTemplate.Spec)
		return apierrors.NewBadRequest("HetznerBareMetalMachineTemplate.Spec is immutable")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerBareMetalMachineTemplate) ValidateDelete() error {
	return nil
}
