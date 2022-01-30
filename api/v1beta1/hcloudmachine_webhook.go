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
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
func (r *HCloudMachine) Default() {}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=create;update,versions=v1beta1,name=validation.hcloudmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HCloudMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateCreate() error {
	hcloudmachinelog.V(1).Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	for _, err := range checkHCloudSSHKeys(r.Spec.SSHKeys) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "sshKeys"), r.Spec.SSHKeys, err.Error()),
		)
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateUpdate(old runtime.Object) error {
	hcloudmachinelog.V(1).Info("validate update", "name", r.Name)

	oldM, ok := old.(*HCloudMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an HCloudMachine but got a %T", old))
	}

	var allErrs field.ErrorList

	// Type is immutable
	if !reflect.DeepEqual(oldM.Spec.Type, r.Spec.Type) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "type"), r.Spec.Type, "field is immutable"),
		)
	}

	// ImageName is immutable
	if !reflect.DeepEqual(oldM.Spec.ImageName, r.Spec.ImageName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "imageName"), r.Spec.ImageName, "field is immutable"),
		)
	}

	// SSHKeys is immutable
	if !reflect.DeepEqual(oldM.Spec.SSHKeys, r.Spec.SSHKeys) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "sshKeys"), r.Spec.SSHKeys, "field is immutable"),
		)
	}

	// Placement group name is immutable
	if !reflect.DeepEqual(oldM.Spec.PlacementGroupName, r.Spec.PlacementGroupName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "placementGroupName"), r.Spec.PlacementGroupName, "field is immutable"),
		)
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateDelete() error {
	hcloudmachinelog.V(1).Info("validate delete", "name", r.Name)
	return nil
}
