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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// hetznerBareMetalHostWebhook implements validating and defaulting webhook for HetznerBareMetalHost.
// +k8s:deepcopy-gen=false
type hetznerBareMetalHostWebhook struct {
	c client.Client
}

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalHost.
func (host *HetznerBareMetalHost) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(hetznerBareMetalHostWebhook)
	w.c = mgr.GetClient()

	return ctrl.NewWebhookManagedBy(mgr).
		For(host).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-hetznerbaremetalhost,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts,verbs=create;update,versions=v1beta2,name=mutation.hetznerbaremetalhost.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta2}

var _ webhook.CustomDefaulter = &hetznerBareMetalHostWebhook{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (hw *hetznerBareMetalHostWebhook) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-hetznerbaremetalhost,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts,verbs=create;update,versions=v1beta2,name=validation.hetznerbaremetalhost.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta2}

var _ webhook.CustomValidator = &hetznerBareMetalHostWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (hw *hetznerBareMetalHostWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	host, ok := (obj).(*HetznerBareMetalHost)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected HetznerBareMetalHost, but got %T", host))
	}

	var allErrs field.ErrorList

	hetznerBareMetalHostList := &HetznerBareMetalHostList{}
	if err := hw.c.List(ctx, hetznerBareMetalHostList, &client.ListOptions{}); err != nil {
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

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (hw *hetznerBareMetalHostWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldHost, ok := oldObj.(*HetznerBareMetalHost)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected an ClusterStack but got a %T", oldObj))
	}

	newHost, ok := newObj.(*HetznerBareMetalHost)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected an ClusterStack but got a %T", newHost))
	}

	var allErrs field.ErrorList

	if newHost.Spec.ServerID != oldHost.Spec.ServerID {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "serverID"), newHost.Spec.ServerID, "serverID is immutable"),
		)
	}

	return nil, aggregateObjErrors(newHost.GroupVersionKind().GroupKind(), newHost.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (hw *hetznerBareMetalHostWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
