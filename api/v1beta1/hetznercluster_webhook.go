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
	"net"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// log is for logging in this package.
var hetznerclusterlog = utils.GetDefaultLogger("info").WithName("hetznercluster-resource")

var regionNetworkZoneMap = map[string]string{
	"fsn1": "eu-central",
	"nbg1": "eu-central",
	"hel1": "eu-central",
	"ash":  "us-east",
	"hil":  "us-west",
	"sin":  "ap-southeast",
}

const (
	// DefaultCIDRBlock specifies the default CIDR block used by the HCloudNetwork.
	DefaultCIDRBlock = "10.0.0.0/16"

	// DefaultSubnetCIDRBlock specifies the default subnet CIDR block used by the HCloudNetwork.
	DefaultSubnetCIDRBlock = "10.0.0.0/24"

	// DefaultNetworkZone specifies the default network zone used by the HCloudNetwork.
	DefaultNetworkZone = "eu-central"
)

// SetupWebhookWithManager initializes webhook manager for HetznerCluster.
func (r *HetznerCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(r).
		Complete()
}

// Could go in own webhook file hetznerclusterlist_webhook.

// SetupWebhookWithManager initializes webhook manager for HetznerClusterList.
func (r *HetznerClusterList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznercluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=create;update,versions=v1beta1,name=mutation.hetznercluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomDefaulter = &HetznerCluster{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (r *HetznerCluster) Default(_ context.Context, obj runtime.Object) error {
	hetznerclusterlog.V(1).Info("default", "name", r.Name)

	cluster, ok := obj.(*HetznerCluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an HetznerCluster but got a %T", obj))
	}

	if !cluster.Spec.HCloudNetwork.Enabled {
		return nil
	}

	if cluster.Spec.HCloudNetwork.ID != nil {
		return nil
	}

	if cluster.Spec.HCloudNetwork.CIDRBlock == nil {
		cluster.Spec.HCloudNetwork.CIDRBlock = ptr.To(DefaultCIDRBlock)
	}
	if cluster.Spec.HCloudNetwork.SubnetCIDRBlock == nil {
		cluster.Spec.HCloudNetwork.SubnetCIDRBlock = ptr.To(DefaultSubnetCIDRBlock)
	}
	if cluster.Spec.HCloudNetwork.NetworkZone == nil {
		cluster.Spec.HCloudNetwork.NetworkZone = ptr.To[HCloudNetworkZone](DefaultNetworkZone)
	}

	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznercluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=create;update,versions=v1beta1,name=validation.hetznercluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HetznerCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerCluster) ValidateCreate() (admission.Warnings, error) {
	hetznerclusterlog.V(1).Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	allowEmptyControlPlaneAddress := r.Annotations[AllowEmptyControlPlaneAddressAnnotation] == "true"

	if !allowEmptyControlPlaneAddress && len(r.Spec.ControlPlaneRegions) == 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "controlPlaneRegions"),
			r.Spec.ControlPlaneRegions,
			"control plane regions must not be empty",
		))
	}

	for _, region := range r.Spec.ControlPlaneRegions {
		if _, ok := regionNetworkZoneMap[string(region)]; !ok {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "controlPlaneRegions"),
				r.Spec.ControlPlaneRegions,
				"wrong control plane region. Should be fsn1, nbg1, hel1, ash, hil or sin",
			))
		}
	}

	if r.Spec.ControlPlaneLoadBalancer.Enabled {
		if r.Spec.ControlPlaneLoadBalancer.Region == Region("") {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "controlPlaneLoadBalancer", "region"),
				r.Spec.ControlPlaneLoadBalancer.Region,
				"region should not be empty if load balancer is enabled"),
			)
		}
	}

	// Check whether regions are all in same network zone
	if !r.Spec.HCloudNetwork.Enabled {
		if err := isNetworkZoneSameForAllRegions(r.Spec.ControlPlaneRegions, nil); err != nil {
			allErrs = append(allErrs, err)
		}
	} else {
		// If ID is given check that all other network settings are empty.
		if r.Spec.HCloudNetwork.ID != nil {
			if errs := areCIDRsAndNetworkZoneEmpty(r.Spec.HCloudNetwork); errs != nil {
				allErrs = append(allErrs, errs...)
			}
		} else {
			// If no ID is given check the other network settings for valid entries.
			if r.Spec.HCloudNetwork.NetworkZone != nil {
				givenZone := string(*r.Spec.HCloudNetwork.NetworkZone)

				var validNetworkZone bool
				for _, z := range regionNetworkZoneMap {
					if givenZone == z {
						validNetworkZone = true
						break
					}
				}
				if !validNetworkZone {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec", "hcloudNetwork", "networkZone"),
						r.Spec.HCloudNetwork.NetworkZone,
						"wrong network zone. Should be eu-central, us-east, us-west or ap-southeast"),
					)
				}
			}

			if r.Spec.HCloudNetwork.CIDRBlock != nil {
				_, _, err := net.ParseCIDR(*r.Spec.HCloudNetwork.CIDRBlock)
				if err != nil {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec", "hcloudNetwork", "cidrBlock"),
						r.Spec.HCloudNetwork.CIDRBlock,
						"malformed cidrBlock"),
					)
				}
			}

			if r.Spec.HCloudNetwork.SubnetCIDRBlock != nil {
				_, _, err := net.ParseCIDR(*r.Spec.HCloudNetwork.SubnetCIDRBlock)
				if err != nil {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec", "hcloudNetwork", "subnetCIDRBlock"),
						r.Spec.HCloudNetwork.SubnetCIDRBlock,
						"malformed cidrBlock"),
					)
				}
			}
		}
	}

	// Check whether controlPlaneEndpoint is specified if allow empty is not set or false

	if !allowEmptyControlPlaneAddress && !r.Spec.ControlPlaneLoadBalancer.Enabled {
		if r.Spec.ControlPlaneEndpoint == nil ||
			r.Spec.ControlPlaneEndpoint.Host == "" ||
			r.Spec.ControlPlaneEndpoint.Port == 0 {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "controlPlaneEndpoint"),
					r.Spec.ControlPlaneEndpoint,
					"controlPlaneEndpoint has to be specified if controlPlaneLoadBalancer is not enabled",
				),
			)
		}
	}

	if err := r.validateHetznerSecretKey(); err != nil {
		allErrs = append(allErrs, err)
	}

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
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
func (r *HetznerCluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	hetznerclusterlog.V(1).Info("validate update", "name", r.Name)
	var allErrs field.ErrorList

	oldC, ok := old.(*HetznerCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an HetznerCluster but got a %T", old))
	}

	if oldC.Spec.HCloudNetwork.Enabled != r.Spec.HCloudNetwork.Enabled {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "hcloudNetwork", "enabled"), r.Spec.HCloudNetwork.Enabled, "field is immutable"),
		)
	}

	if !oldC.Spec.HCloudNetwork.Enabled {
		// If the network is disabled check that all other network related fields are empty.
		if r.Spec.HCloudNetwork.ID != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "hcloudNetwork", "id"), oldC.Spec.HCloudNetwork.ID, "field must be empty"),
			)
		}
		if errs := areCIDRsAndNetworkZoneEmpty(r.Spec.HCloudNetwork); errs != nil {
			allErrs = append(allErrs, errs...)
		}
	}

	if oldC.Spec.HCloudNetwork.Enabled {
		// Only allow updating the network ID when it was not set previously. This makes it possible to e.g. adopt the
		// network that was created initially by CAPH.
		if oldC.Spec.HCloudNetwork.ID != nil && !reflect.DeepEqual(oldC.Spec.HCloudNetwork.ID, r.Spec.HCloudNetwork.ID) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "hcloudNetwork", "id"), r.Spec.HCloudNetwork.ID, "field is immutable"),
			)
		}

		if !reflect.DeepEqual(oldC.Spec.HCloudNetwork.CIDRBlock, r.Spec.HCloudNetwork.CIDRBlock) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "hcloudNetwork", "cidrBlock"), r.Spec.HCloudNetwork.CIDRBlock, "field is immutable"),
			)
		}

		if !reflect.DeepEqual(oldC.Spec.HCloudNetwork.SubnetCIDRBlock, r.Spec.HCloudNetwork.SubnetCIDRBlock) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "hcloudNetwork", "subnetCIDRBlock"), r.Spec.HCloudNetwork.SubnetCIDRBlock, "field is immutable"),
			)
		}

		if !reflect.DeepEqual(oldC.Spec.HCloudNetwork.NetworkZone, r.Spec.HCloudNetwork.NetworkZone) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "hcloudNetwork", "networkZone"), r.Spec.HCloudNetwork.NetworkZone, "field is immutable"),
			)
		}
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

	// Load balancer enabled/disabled is immutable
	if oldC.Spec.ControlPlaneLoadBalancer.Enabled != r.Spec.ControlPlaneLoadBalancer.Enabled {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "controlPlaneLoadBalancer", "enabled"), r.Spec.ControlPlaneLoadBalancer.Enabled, "field is immutable"),
		)
	}

	// Load balancer region and port are immutable
	if oldC.Spec.ControlPlaneLoadBalancer.Port != r.Spec.ControlPlaneLoadBalancer.Port {
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

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

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
func (r *HetznerCluster) ValidateDelete() (admission.Warnings, error) {
	hetznerclusterlog.V(1).Info("validate delete", "name", r.Name)
	return nil, nil
}

func areCIDRsAndNetworkZoneEmpty(hcloudNetwork HCloudNetworkSpec) field.ErrorList {
	var allErrs field.ErrorList
	if hcloudNetwork.CIDRBlock != nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "hcloudNetwork", "cidrBlock"), hcloudNetwork.CIDRBlock, "field must be empty"),
		)
	}

	if hcloudNetwork.SubnetCIDRBlock != nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "hcloudNetwork", "subnetCIDRBlock"), hcloudNetwork.SubnetCIDRBlock, "field must be empty"),
		)
	}

	if hcloudNetwork.NetworkZone != nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "hcloudNetwork", "networkZone"), hcloudNetwork.NetworkZone, "field must be empty"),
		)
	}

	return allErrs
}
