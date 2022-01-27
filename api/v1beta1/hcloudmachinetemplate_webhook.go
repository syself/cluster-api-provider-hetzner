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
	"reflect"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hcloudmachinetemplatelog = utils.GetDefaultLogger("info").WithName("hcloudmachinetemplate-resource")

// SetupWebhookWithManager initializes webhook manager for HetznerMachineTemplate.
func (r *HCloudMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// SetupWebhookWithManager initializes webhook manager for HetznerMachineTemplateList.
func (r *HCloudMachineTemplateList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachinetemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachinetemplates,verbs=create;update,versions=v1beta1,name=mutation.hcloudmachinetemplate.infrastructure.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HCloudMachineTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *HCloudMachineTemplate) Default() {}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachinetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachinetemplates,verbs=create;update,versions=v1beta1,name=validation.hcloudmachinetemplate.infrastructure.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HCloudMachineTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachineTemplate) ValidateCreate() error {
	hcloudmachinetemplatelog.V(1).Info("validate create", "name", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachineTemplate) ValidateUpdate(old runtime.Object) error {
	hcloudmachinetemplatelog.V(1).Info("validate update", "name", r.Name)
	oldHCloudMachineTemplate := old.(*HCloudMachineTemplate)
	if !reflect.DeepEqual(r.Spec, oldHCloudMachineTemplate.Spec) {
		hcloudmachinetemplatelog.Info("not equal", "new HcloudMachineTemplateSpec", r.Spec, "old HcloudMachineTemplateSpec", oldHCloudMachineTemplate.Spec)
		return apierrors.NewBadRequest("HCloudMachineTemplate.Spec is immutable")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachineTemplate) ValidateDelete() error {
	hcloudmachinetemplatelog.V(1).Info("validate delete", "name", r.Name)
	return nil
}
