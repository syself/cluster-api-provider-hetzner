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
var hetznerclusterlog = utils.GetDefaultLogger("info").WithName("hetznercluster-resource")

var regionNetworkZoneMap = map[string]string{
	"fsn1": "eu-central",
	"nbg1": "eu-central",
	"hel1": "eu-central",
	"ash":  "us-east",
}

// SetupWebhookWithManager initializes webhook manager for HetznerCluster.
func (r *HetznerCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// Could go in own webhook file hetznerclusterlist_webhook

// SetupWebhookWithManager initializes webhook manager for HetznerClusterList.
func (r *HetznerClusterList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznercluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=create;update,versions=v1beta1,name=mutation.hetznercluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HetznerCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *HetznerCluster) Default() {}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznercluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=create;update,versions=v1beta1,name=validation.hetznercluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HetznerCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerCluster) ValidateCreate() error {
	hetznerclusterlog.V(1).Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	// Check whether regions are all in same network zone
	if !r.Spec.HCloudNetwork.Enabled {
		if err := isNetworkZoneSameForAllRegions(r.Spec.ControlPlaneRegions, nil); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	if err := r.validateHetznerSecretKey(); err != nil {
		allErrs = append(allErrs, err)
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

func isNetworkZoneSameForAllRegions(regions []Region, defaultNetworkZone *string) *field.Error {
	if len(regions) == 0 {
		return nil
	}

	defaultNZ := regionNetworkZoneMap[string(regions[0])]
	if defaultNetworkZone != nil {
		defaultNZ = *defaultNetworkZone
	}
	for _, region := range regions {
		if regionNetworkZoneMap[string(region)] != defaultNZ {
			return field.Invalid(field.NewPath("spec", "controlPlaneRegions"), regions, "regions are not in one network zone")
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerCluster) ValidateUpdate(old runtime.Object) error {
	hetznerclusterlog.V(1).Info("validate update", "name", r.Name)
	var allErrs field.ErrorList

	oldC, ok := old.(*HetznerCluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an HetznerCluster but got a %T", old))
	}

	// Network settings are immutable
	if !reflect.DeepEqual(oldC.Spec.HCloudNetwork, r.Spec.HCloudNetwork) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "hcloudNetwork"), r.Spec.HCloudNetwork, "field is immutable"),
		)
	}

	// Check if all regions are in the same network zone if a private network is enabled
	if oldC.Spec.HCloudNetwork.Enabled {
		var defaultNetworkZone *string
		if len(oldC.Spec.ControlPlaneRegions) > 0 {
			str := regionNetworkZoneMap[string(oldC.Spec.ControlPlaneRegions[0])]
			defaultNetworkZone = &str
		}

		if err := isNetworkZoneSameForAllRegions(r.Spec.ControlPlaneRegions, defaultNetworkZone); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// Load balancer region and port are immutable
	if !reflect.DeepEqual(oldC.Spec.ControlPlaneLoadBalancer.Port, r.Spec.ControlPlaneLoadBalancer.Port) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "controlPlaneLoadBalancer", "port"), r.Spec.ControlPlaneLoadBalancer.Port, "field is immutable"),
		)
	}
	if !reflect.DeepEqual(oldC.Spec.ControlPlaneLoadBalancer.Region, r.Spec.ControlPlaneLoadBalancer.Region) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "controlPlaneLoadBalancer", "region"), r.Spec.ControlPlaneLoadBalancer.Region, "field is immutable"),
		)
	}

	if err := r.validateHetznerSecretKey(); err != nil {
		allErrs = append(allErrs, err)
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

func (r *HetznerCluster) isNetworkZoneSameForAllRegions(regions []Region, defaultNetworkZone *string) *field.Error {
	if len(regions) == 0 {
		return nil
	}

	defaultNZ := regionNetworkZoneMap[string(regions[0])]
	if defaultNetworkZone != nil {
		defaultNZ = *defaultNetworkZone
	}
	for _, region := range regions {
		if regionNetworkZoneMap[string(region)] != defaultNZ {
			return field.Invalid(field.NewPath("spec", "controlPlaneRegions"), r.Spec.ControlPlaneRegions, "regions are not in one network zone")
		}
	}
	return nil
} //lint:ignore U1000 Ignore unused function temporarily for debugging

func (r *HetznerCluster) validateHetznerSecretKey() *field.Error {
	// Hetzner secret key needs to contain either HCloud or Hrobot credentials
	if r.Spec.HetznerSecret.Key.HCloudToken == "" &&
		(r.Spec.HetznerSecret.Key.HetznerRobotUser == "" || r.Spec.HetznerSecret.Key.HetznerRobotPassword == "") {
		return field.Invalid(
			field.NewPath("spec", "hetznerSecret", "key"),
			r.Spec.HetznerSecret.Key,
			"need to specify credentials for either HCloud or Hetzner robot",
		)
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerCluster) ValidateDelete() error {
	hetznerclusterlog.V(1).Info("validate delete", "name", r.Name)
	return nil
}
