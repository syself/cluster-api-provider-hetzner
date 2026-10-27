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

package v1beta2

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// HetznerClusterWebhook implements admission webhooks for HetznerCluster.
type HetznerClusterWebhook struct{}

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

// SetupWebhookWithManager initializes webhook manager for HetznerCluster.
func (webhook *HetznerClusterWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.HetznerCluster{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

var _ admission.Defaulter[*infrav1.HetznerCluster] = &HetznerClusterWebhook{}

// Default implements admission.Defaulter so a webhook will be registered for HetznerCluster.
func (*HetznerClusterWebhook) Default(context.Context, *infrav1.HetznerCluster) error {
	return nil
}

var _ admission.Validator[*infrav1.HetznerCluster] = &HetznerClusterWebhook{}

// ValidateCreate implements admission.Validator so a webhook will be registered for HetznerCluster.
func (*HetznerClusterWebhook) ValidateCreate(_ context.Context, r *infrav1.HetznerCluster) (admission.Warnings, error) {
	hetznerclusterlog.V(1).Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	allowEmptyControlPlaneAddress := r.Annotations[infrav1.AllowEmptyControlPlaneAddressAnnotation] == "true"

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

	if ptr.Deref(r.Spec.ControlPlaneLoadBalancer.Enabled, false) {
		if r.Spec.ControlPlaneLoadBalancer.Region == infrav1.Region("") {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "controlPlaneLoadBalancer", "region"),
				r.Spec.ControlPlaneLoadBalancer.Region,
				"region should not be empty if load balancer is enabled"),
			)
		}
	}

	// Check whether regions are all in same network zone
	if !ptr.Deref(r.Spec.HCloudNetwork.Enabled, false) {
		if err := isNetworkZoneSameForAllRegions(r.Spec.ControlPlaneRegions, nil); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// Check whether controlPlaneEndpoint is specified if allow empty is not set or false

	if !allowEmptyControlPlaneAddress && !ptr.Deref(r.Spec.ControlPlaneLoadBalancer.Enabled, false) {
		if r.Spec.ControlPlaneEndpoint.Host == "" ||
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

	if err := validateHetznerSecretKey(r); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateLoadBalancerHealthCheck(r.Spec.ControlPlaneLoadBalancer.HealthCheck); err != nil {
		allErrs = append(allErrs, err)
	}

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

func isNetworkZoneSameForAllRegions(regions []infrav1.Region, defaultNetworkZone *string) *field.Error {
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

// ValidateUpdate implements admission.Validator so a webhook will be registered for HetznerCluster.
func (*HetznerClusterWebhook) ValidateUpdate(_ context.Context, oldC, r *infrav1.HetznerCluster) (admission.Warnings, error) {
	hetznerclusterlog.V(1).Info("validate update", "name", r.Name)
	var allErrs field.ErrorList

	// Network settings are immutable
	if !reflect.DeepEqual(normalizeHetznerClusterSpec(oldC.Spec).HCloudNetwork, normalizeHetznerClusterSpec(r.Spec).HCloudNetwork) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "hcloudNetwork"), r.Spec.HCloudNetwork, "field is immutable"),
		)
	}

	// Check if all regions are in the same network zone if a private network is enabled
	if ptr.Deref(oldC.Spec.HCloudNetwork.Enabled, false) {
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
	if ptr.Deref(oldC.Spec.ControlPlaneLoadBalancer.Enabled, false) != ptr.Deref(r.Spec.ControlPlaneLoadBalancer.Enabled, false) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "controlPlaneLoadBalancer", "enabled"), r.Spec.ControlPlaneLoadBalancer.Enabled, "field is immutable"),
		)
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

	if ptr.Deref(oldC.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol, false) && !ptr.Deref(r.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol, false) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "controlPlaneLoadBalancer", "enableProxyProtocol"),
				r.Spec.ControlPlaneLoadBalancer.EnableProxyProtocol, "proxy protocol cannot be disabled once enabled"),
		)
	}

	if err := validateHetznerSecretKey(r); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateLoadBalancerHealthCheck(r.Spec.ControlPlaneLoadBalancer.HealthCheck); err != nil {
		allErrs = append(allErrs, err)
	}

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// validateLoadBalancerHealthCheck rejects HTTP(S)-only fields set while the health check protocol
// is tcp. All other combinations, including any change to an already-set health check, are allowed.
func validateLoadBalancerHealthCheck(hc *infrav1.LoadBalancerHealthCheckSpec) *field.Error {
	if hc == nil || hc.Protocol == "http" || hc.Protocol == "https" {
		return nil
	}

	if hc.Path != nil {
		return field.Invalid(
			field.NewPath("spec", "controlPlaneLoadBalancer", "healthCheck", "path"),
			*hc.Path, "path must not be set when protocol is tcp",
		)
	}

	if hc.Domain != nil {
		return field.Invalid(
			field.NewPath("spec", "controlPlaneLoadBalancer", "healthCheck", "domain"),
			*hc.Domain, "domain must not be set when protocol is tcp",
		)
	}

	if hc.Response != nil {
		return field.Invalid(
			field.NewPath("spec", "controlPlaneLoadBalancer", "healthCheck", "response"),
			*hc.Response, "response must not be set when protocol is tcp",
		)
	}

	if len(hc.StatusCodes) > 0 {
		return field.Invalid(
			field.NewPath("spec", "controlPlaneLoadBalancer", "healthCheck", "statusCodes"),
			hc.StatusCodes, "statusCodes must not be set when protocol is tcp",
		)
	}

	return nil
}

func validateHetznerSecretKey(r *infrav1.HetznerCluster) *field.Error {
	// Hetzner secret key needs to contain either HCloud or Hrobot credentials
	key := r.Spec.HetznerSecret.Key
	if key.GetHCloudToken() == "" && (key.GetHetznerRobotUser() == "" || key.GetHetznerRobotPassword() == "") {
		return field.Invalid(
			field.NewPath("spec", "hetznerSecret", "key"),
			r.Spec.HetznerSecret.Key,
			"need to specify credentials for either HCloud or Hetzner robot",
		)
	}
	return nil
}

// normalizeHetznerClusterSpec returns a copy of spec in which every unset field gets the value that an
// unset field means. A stored object and an update request can differ in which unset fields they send.
// Without this, such an update looks like a change of an immutable field.
func normalizeHetznerClusterSpec(spec infrav1.HetznerClusterSpec) infrav1.HetznerClusterSpec {
	if spec.HCloudNetwork.Enabled == nil {
		spec.HCloudNetwork.Enabled = ptr.To(false)
	}
	if spec.SSHKeys.RescueSecretRef == nil {
		spec.SSHKeys.RescueSecretRef = &infrav1.SSHSecretRef{}
	}
	// A controlPlaneLoadBalancer that is left out gets no defaults, so an unset enabled means false.
	if spec.ControlPlaneLoadBalancer.Enabled == nil {
		spec.ControlPlaneLoadBalancer.Enabled = ptr.To(false)
	}
	if spec.ControlPlaneLoadBalancer.EnableProxyProtocol == nil {
		spec.ControlPlaneLoadBalancer.EnableProxyProtocol = ptr.To(false)
	}
	if spec.SkipCreatingHetznerSecretInWorkloadCluster == nil {
		spec.SkipCreatingHetznerSecretInWorkloadCluster = ptr.To(false)
	}
	// An unset key means the default of that key.
	key := &spec.HetznerSecret.Key
	key.HCloudToken = ptr.To(key.GetHCloudToken())
	key.HetznerRobotUser = ptr.To(key.GetHetznerRobotUser())
	key.HetznerRobotPassword = ptr.To(key.GetHetznerRobotPassword())
	key.SSHKey = ptr.To(key.GetSSHKey())
	return spec
}

// ValidateDelete implements admission.Validator so a webhook will be registered for HetznerCluster.
func (*HetznerClusterWebhook) ValidateDelete(context.Context, *infrav1.HetznerCluster) (admission.Warnings, error) {
	return nil, nil
}
