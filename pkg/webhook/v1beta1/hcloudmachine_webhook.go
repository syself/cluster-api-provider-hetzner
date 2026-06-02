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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// HCloudMachineWebhook implements admission webhooks for HCloudMachine.
type HCloudMachineWebhook struct{}

// log is for logging in this package.
var hcloudmachinelog = utils.GetDefaultLogger("info").WithName("hcloudmachine-resource")

// SetupWebhookWithManager initializes webhook manager for HCloudMachine.
func (webhook *HCloudMachineWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.HCloudMachine{}).
		WithDefaulter(webhook).
		WithValidator(webhook).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=create;update,versions=v1beta1,name=mutation.hcloudmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Defaulter[*infrav1.HCloudMachine] = &HCloudMachineWebhook{}

// Default implements admission.Defaulter so a webhook will be registered for HCloudMachine.
func (*HCloudMachineWebhook) Default(_ context.Context, r *infrav1.HCloudMachine) error {
	if r.Spec.PublicNetwork == nil {
		r.Spec.PublicNetwork = &infrav1.PublicNetworkSpec{
			EnableIPv4: true,
			EnableIPv6: true,
		}
	}
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=create;update,versions=v1beta1,name=validation.hcloudmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*infrav1.HCloudMachine] = &HCloudMachineWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HCloudMachine.
func (*HCloudMachineWebhook) ValidateCreate(_ context.Context, r *infrav1.HCloudMachine) (admission.Warnings, error) {
	hcloudmachinelog.V(1).Info("validate create", "name", r.Name)

	allErrs := validateHCloudMachineSpec(r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for HCloudMachine.
func (*HCloudMachineWebhook) ValidateUpdate(_ context.Context, oldM, r *infrav1.HCloudMachine) (admission.Warnings, error) {
	hcloudmachinelog.V(1).Info("validate update", "name", r.Name)

	allErrs := validateHCloudMachineSpecUpdate(oldM.Spec, r.Spec)

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HCloudMachine.
func (*HCloudMachineWebhook) ValidateDelete(context.Context, *infrav1.HCloudMachine) (admission.Warnings, error) {
	return nil, nil
}
