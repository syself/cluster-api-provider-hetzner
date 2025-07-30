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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

type hcloudMachineWebhook struct{}

// log is for logging in this package.
var hcloudmachinelog = utils.GetDefaultLogger("info").WithName("hcloudmachine-resource")

// SetupWebhookWithManager initializes webhook manager for HCloudMachine.
func (r *HCloudMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hcloudMachineWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// SetupWebhookWithManager initializes webhook manager for HCloudMachineList.
func (r *HCloudMachineList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=create;update,versions=v1beta1,name=mutation.hcloudmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomDefaulter = &hcloudMachineWebhook{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (*hcloudMachineWebhook) Default(_ context.Context, obj runtime.Object) error {
	r, ok := obj.(*HCloudMachine)
	if !ok {
		return fmt.Errorf("expected an HCloudMachine object but got %T", r)
	}
	if r.Spec.PublicNetwork == nil {
		r.Spec.PublicNetwork = &PublicNetworkSpec{
			EnableIPv4: true,
			EnableIPv6: true,
		}
	}
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=create;update,versions=v1beta1,name=validation.hcloudmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &hcloudMachineWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hcloudMachineWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	r, ok := obj.(*HCloudMachine)
	if !ok {
		return nil, fmt.Errorf("expected an HCloudMachine object but got %T", r)
	}

	hcloudmachinelog.V(1).Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*hcloudMachineWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	r, ok := newObj.(*HCloudMachine)
	if !ok {
		return nil, fmt.Errorf("expected an HCloudMachine object but got %T", r)
	}
	hcloudmachinelog.V(1).Info("validate update", "name", r.Name)

	oldM, ok := oldObj.(*HCloudMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an HCloudMachine but got a %T", oldObj))
	}

	allErrs := validateHCloudMachineSpec(oldM.Spec, r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *hcloudMachineWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
