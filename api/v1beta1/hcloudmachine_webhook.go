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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// log is for logging in this package.
var hcloudmachinelog = utils.GetDefaultLogger("info").WithName("hcloudmachine-resource")

// SetupWebhookWithManager initializes webhook manager for HCloudMachine.
func (r *HCloudMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// SetupWebhookWithManager initializes webhook manager for HCloudMachineList.
func (r *HCloudMachineList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=create;update,versions=v1beta1,name=mutation.hcloudmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HCloudMachine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *HCloudMachine) Default() {
	if r.Spec.PublicNetwork == nil {
		r.Spec.PublicNetwork = &PublicNetworkSpec{
			EnableIPv4: true,
			EnableIPv6: true,
		}
	}
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=create;update,versions=v1beta1,name=validation.hcloudmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HCloudMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateCreate() (admission.Warnings, error) {
	hcloudmachinelog.V(1).Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	hcloudmachinelog.V(1).Info("validate update", "name", r.Name)

	oldM, ok := old.(*HCloudMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an HCloudMachine but got a %T", old))
	}

	allErrs := validateHCloudMachineSpec(oldM.Spec, r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateDelete() (admission.Warnings, error) {
	hcloudmachinelog.V(1).Info("validate delete", "name", r.Name)
	return nil, nil
}
