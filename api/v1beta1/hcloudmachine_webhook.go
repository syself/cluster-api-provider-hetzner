/*
Copyright 2021.

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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hcloudmachinelog = logf.Log.WithName("hcloudmachine-resource")

func (r *HCloudMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

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

type hCloudMachineSpecer interface {
	GroupVersionKind() schema.GroupVersionKind
	GetName() string
	HCloudMachineSpec() *HCloudMachineSpec
}

func validateHCloudMachineSpec(r hCloudMachineSpecer) error {
	var allErrs field.ErrorList

	if len(r.HCloudMachineSpec().Type) == 0 {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "type"), r.HCloudMachineSpec().Type, "field cannot be empty"),
		)
	}
	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.GetName(), allErrs)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateCreate() error {
	hcloudmachinelog.Info("validate create", "name", r.Name)
	return validateHCloudMachineSpec(r)
	//TODO: validate SSHKEYNAME, ErrorList
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateUpdate(old runtime.Object) error {
	hcloudmachinelog.Info("validate update", "name", r.Name)

	var allErrs field.ErrorList

	oldM, ok := old.(*HCloudMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an HCloudMachine but got a %T", old))
	}

	if r.Spec.Type != oldM.Spec.Type {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "type"), r.Spec.Type, "field is immutable"),
		)
	}
	// TODO: test all Machine Specs if they changed.
	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HCloudMachine) ValidateDelete() error {
	hcloudmachinelog.Info("validate delete", "name", r.Name)
	return nil
}
