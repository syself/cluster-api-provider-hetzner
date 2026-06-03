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

// This file contains v1beta1 <-> v1beta2 conversion hooks.
// Generated converters handle fields that still map directly between versions.
// Hand-written converters handle fields whose API shape changed.

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

// ConvertTo converts this HetznerCluster to the Hub version (v1beta2).
func (src *HetznerCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerCluster)
	if err := Convert_v1beta1_HetznerCluster_To_v1beta2_HetznerCluster(src, dst, nil); err != nil {
		return err
	}

	// Recover hub-only data stored by a previous down-conversion so the round trip is lossless.
	restored := &infrav1.HetznerCluster{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	// status.ready (bool) maps to status.initialization.provisioned (*bool). The CAPI helper only
	// produces *false when the value was intentionally *false before (restored); otherwise a false
	// ready becomes nil, matching the one-time provisioning signal semantics.
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &dst.Status.Initialization.Provisioned)

	return nil
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerCluster.
func (dst *HetznerCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerCluster)
	if err := Convert_v1beta2_HetznerCluster_To_v1beta1_HetznerCluster(src, dst, nil); err != nil {
		return err
	}

	// Preserve the hub object in a data annotation so the next up-conversion can restore hub-only intent (see ConvertTo).
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts this HetznerClusterTemplate to the Hub version (v1beta2).
func (src *HetznerClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerClusterTemplate)
	return Convert_v1beta1_HetznerClusterTemplate_To_v1beta2_HetznerClusterTemplate(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerClusterTemplate.
func (dst *HetznerClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerClusterTemplate)
	return Convert_v1beta2_HetznerClusterTemplate_To_v1beta1_HetznerClusterTemplate(src, dst, nil)
}

// ConvertTo converts this HCloudMachine to the Hub version (v1beta2).
func (src *HCloudMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HCloudMachine)
	return Convert_v1beta1_HCloudMachine_To_v1beta2_HCloudMachine(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HCloudMachine.
func (dst *HCloudMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HCloudMachine)
	return Convert_v1beta2_HCloudMachine_To_v1beta1_HCloudMachine(src, dst, nil)
}

// ConvertTo converts this HCloudMachineTemplate to the Hub version (v1beta2).
func (src *HCloudMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HCloudMachineTemplate)
	return Convert_v1beta1_HCloudMachineTemplate_To_v1beta2_HCloudMachineTemplate(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HCloudMachineTemplate.
func (dst *HCloudMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HCloudMachineTemplate)
	return Convert_v1beta2_HCloudMachineTemplate_To_v1beta1_HCloudMachineTemplate(src, dst, nil)
}

// ConvertTo converts this HCloudRemediation to the Hub version (v1beta2).
func (src *HCloudRemediation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HCloudRemediation)
	return Convert_v1beta1_HCloudRemediation_To_v1beta2_HCloudRemediation(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HCloudRemediation.
func (dst *HCloudRemediation) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HCloudRemediation)
	return Convert_v1beta2_HCloudRemediation_To_v1beta1_HCloudRemediation(src, dst, nil)
}

// ConvertTo converts this HCloudRemediationTemplate to the Hub version (v1beta2).
func (src *HCloudRemediationTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HCloudRemediationTemplate)
	return Convert_v1beta1_HCloudRemediationTemplate_To_v1beta2_HCloudRemediationTemplate(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HCloudRemediationTemplate.
func (dst *HCloudRemediationTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HCloudRemediationTemplate)
	return Convert_v1beta2_HCloudRemediationTemplate_To_v1beta1_HCloudRemediationTemplate(src, dst, nil)
}

// ConvertTo converts this HetznerBareMetalHost to the Hub version (v1beta2).
func (src *HetznerBareMetalHost) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerBareMetalHost)
	return Convert_v1beta1_HetznerBareMetalHost_To_v1beta2_HetznerBareMetalHost(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerBareMetalHost.
func (dst *HetznerBareMetalHost) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerBareMetalHost)
	return Convert_v1beta2_HetznerBareMetalHost_To_v1beta1_HetznerBareMetalHost(src, dst, nil)
}

// ConvertTo converts this HetznerBareMetalMachine to the Hub version (v1beta2).
func (src *HetznerBareMetalMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerBareMetalMachine)
	return Convert_v1beta1_HetznerBareMetalMachine_To_v1beta2_HetznerBareMetalMachine(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerBareMetalMachine.
func (dst *HetznerBareMetalMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerBareMetalMachine)
	return Convert_v1beta2_HetznerBareMetalMachine_To_v1beta1_HetznerBareMetalMachine(src, dst, nil)
}

// ConvertTo converts this HetznerBareMetalMachineTemplate to the Hub version (v1beta2).
func (src *HetznerBareMetalMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerBareMetalMachineTemplate)
	return Convert_v1beta1_HetznerBareMetalMachineTemplate_To_v1beta2_HetznerBareMetalMachineTemplate(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerBareMetalMachineTemplate.
func (dst *HetznerBareMetalMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerBareMetalMachineTemplate)
	return Convert_v1beta2_HetznerBareMetalMachineTemplate_To_v1beta1_HetznerBareMetalMachineTemplate(src, dst, nil)
}

// ConvertTo converts this HetznerBareMetalRemediation to the Hub version (v1beta2).
func (src *HetznerBareMetalRemediation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerBareMetalRemediation)
	return Convert_v1beta1_HetznerBareMetalRemediation_To_v1beta2_HetznerBareMetalRemediation(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerBareMetalRemediation.
func (dst *HetznerBareMetalRemediation) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerBareMetalRemediation)
	return Convert_v1beta2_HetznerBareMetalRemediation_To_v1beta1_HetznerBareMetalRemediation(src, dst, nil)
}

// ConvertTo converts this HetznerBareMetalRemediationTemplate to the Hub version (v1beta2).
func (src *HetznerBareMetalRemediationTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerBareMetalRemediationTemplate)
	return Convert_v1beta1_HetznerBareMetalRemediationTemplate_To_v1beta2_HetznerBareMetalRemediationTemplate(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerBareMetalRemediationTemplate.
func (dst *HetznerBareMetalRemediationTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerBareMetalRemediationTemplate)
	return Convert_v1beta2_HetznerBareMetalRemediationTemplate_To_v1beta1_HetznerBareMetalRemediationTemplate(src, dst, nil)
}

// Note: conversion-gen generates public Convert_... wrappers only when **every** field can be mapped.
// If a field has no peer in the other version, it only generates autoConvert_... helpers for the
// matching fields and leaves the unmatched field for manual conversion.

// ConsumerRef changed shape between v1beta1 and v1beta2. v1beta1 keeps the historical
// ObjectReference shape, while v1beta2 only persists the identity the host needs: kind, name, and
// API group. Namespace is not stored in the v1beta2 field because the consuming machine lives in
// the same namespace as the host.
//
// The type-level conversion functions below keep conversion-gen working for the spec field. The
// object-level v1beta2 to v1beta1 converter then restores ConsumerRef.Namespace from the enclosing
// host object for old clients.

// Convert_v1beta1_HetznerBareMetalHost_To_v1beta2_HetznerBareMetalHost converts a v1beta1
// HetznerBareMetalHost to v1beta2, including the v1beta2 ConsumerRef shape.
func Convert_v1beta1_HetznerBareMetalHost_To_v1beta2_HetznerBareMetalHost(in *HetznerBareMetalHost, out *infrav1.HetznerBareMetalHost, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HetznerBareMetalHost_To_v1beta2_HetznerBareMetalHost(in, out, s)
}

// Convert_v1beta2_HetznerBareMetalHost_To_v1beta1_HetznerBareMetalHost converts a v1beta2
// HetznerBareMetalHost to v1beta1 and restores ConsumerRef.Namespace from the host namespace.
func Convert_v1beta2_HetznerBareMetalHost_To_v1beta1_HetznerBareMetalHost(in *infrav1.HetznerBareMetalHost, out *HetznerBareMetalHost, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_HetznerBareMetalHost_To_v1beta1_HetznerBareMetalHost(in, out, s); err != nil {
		return err
	}
	if out.Spec.ConsumerRef != nil {
		// v1beta2 does not carry ConsumerRef.Namespace, so restore the old ObjectReference shape
		// from the containing host object.
		out.Spec.ConsumerRef.Namespace = in.Namespace
	}
	return nil
}

// Convert_v1beta1_HetznerBareMetalHostSpec_To_v1beta2_HetznerBareMetalHostSpec converts the
// v1beta1 ObjectReference ConsumerRef to the smaller v1beta2 local reference.
func Convert_v1beta1_HetznerBareMetalHostSpec_To_v1beta2_HetznerBareMetalHostSpec(in *HetznerBareMetalHostSpec, out *infrav1.HetznerBareMetalHostSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HetznerBareMetalHostSpec_To_v1beta2_HetznerBareMetalHostSpec(in, out, s)
}

// Convert_v1beta2_HetznerBareMetalHostSpec_To_v1beta1_HetznerBareMetalHostSpec converts the
// v1beta2 ConsumerRef shape back to the v1beta1 ObjectReference shape without namespace.
func Convert_v1beta2_HetznerBareMetalHostSpec_To_v1beta1_HetznerBareMetalHostSpec(in *infrav1.HetznerBareMetalHostSpec, out *HetznerBareMetalHostSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta2_HetznerBareMetalHostSpec_To_v1beta1_HetznerBareMetalHostSpec(in, out, s)
}

// Convert_v1_ObjectReference_To_v1beta2_HetznerBareMetalHostConsumerReference converts the
// v1beta1 ObjectReference shape to the smaller v1beta2 ConsumerRef shape.
func Convert_v1_ObjectReference_To_v1beta2_HetznerBareMetalHostConsumerReference(in *corev1.ObjectReference, out *infrav1.HetznerBareMetalHostConsumerReference, _ apiconversion.Scope) error {
	apiGroup := ""
	if in.APIVersion != "" {
		gv, err := schema.ParseGroupVersion(in.APIVersion)
		if err != nil {
			return fmt.Errorf("failed to convert consumerRef: failed to parse apiVersion %q: %w", in.APIVersion, err)
		}
		apiGroup = gv.Group
	}

	// Persist only the fields that make up the v1beta2 host ownership state.
	out.Kind = in.Kind
	out.Name = in.Name
	out.APIGroup = apiGroup
	return nil
}

// Convert_v1beta2_HetznerBareMetalHostConsumerReference_To_v1_ObjectReference converts the
// v1beta2 ConsumerRef shape back to an ObjectReference without namespace.
func Convert_v1beta2_HetznerBareMetalHostConsumerReference_To_v1_ObjectReference(in *infrav1.HetznerBareMetalHostConsumerReference, out *corev1.ObjectReference, _ apiconversion.Scope) error {
	out.APIVersion = schema.GroupVersion{Group: in.APIGroup, Version: GroupVersion.Version}.String()
	out.Kind = in.Kind
	out.Name = in.Name
	return nil
}

// Convert_v1beta1_ControllerGeneratedStatus_To_v1beta2_ControllerGeneratedStatus converts
// the v1beta1 ControllerGeneratedStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_ControllerGeneratedStatus_To_v1beta2_ControllerGeneratedStatus(in *ControllerGeneratedStatus, out *infrav1.ControllerGeneratedStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_ControllerGeneratedStatus_To_v1beta2_ControllerGeneratedStatus(in, out, s)
}

// Convert_v1beta1_HCloudMachineStatus_To_v1beta2_HCloudMachineStatus converts the v1beta1
// HCloudMachineStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_HCloudMachineStatus_To_v1beta2_HCloudMachineStatus(in *HCloudMachineStatus, out *infrav1.HCloudMachineStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HCloudMachineStatus_To_v1beta2_HCloudMachineStatus(in, out, s)
}

// Convert_v1beta1_HCloudMachineTemplateStatus_To_v1beta2_HCloudMachineTemplateStatus converts
// the v1beta1 HCloudMachineTemplateStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_HCloudMachineTemplateStatus_To_v1beta2_HCloudMachineTemplateStatus(in *HCloudMachineTemplateStatus, out *infrav1.HCloudMachineTemplateStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HCloudMachineTemplateStatus_To_v1beta2_HCloudMachineTemplateStatus(in, out, s)
}

// Convert_v1beta1_HCloudRemediationStatus_To_v1beta2_HCloudRemediationStatus converts the
// v1beta1 HCloudRemediationStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_HCloudRemediationStatus_To_v1beta2_HCloudRemediationStatus(in *HCloudRemediationStatus, out *infrav1.HCloudRemediationStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HCloudRemediationStatus_To_v1beta2_HCloudRemediationStatus(in, out, s)
}

// Convert_v1beta1_HetznerBareMetalMachineStatus_To_v1beta2_HetznerBareMetalMachineStatus converts
// the v1beta1 HetznerBareMetalMachineStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_HetznerBareMetalMachineStatus_To_v1beta2_HetznerBareMetalMachineStatus(in *HetznerBareMetalMachineStatus, out *infrav1.HetznerBareMetalMachineStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HetznerBareMetalMachineStatus_To_v1beta2_HetznerBareMetalMachineStatus(in, out, s)
}

// Convert_v1beta1_HetznerClusterStatus_To_v1beta2_HetznerClusterStatus converts the v1beta1
// HetznerClusterStatus to v1beta2. The v1beta1 status.conditions (old clusterv1beta1.Conditions)
// and the v1beta2 status.conditions ([]metav1.Condition) share a field name but not a type and do
// not correspond, so HetznerClusterStatus is excluded from conversion-gen (+k8s:conversion-gen=false)
// and converted fully by hand here:
//   - status.v1beta2.conditions is promoted to status.conditions.
//   - status.conditions is demoted to status.deprecated.v1beta1.conditions.
//   - status.ready maps to status.initialization.provisioned at the object level
//     (HetznerCluster.ConvertTo), because that lossy bool -> *bool mapping needs the restored hub data.
func Convert_v1beta1_HetznerClusterStatus_To_v1beta2_HetznerClusterStatus(in *HetznerClusterStatus, out *infrav1.HetznerClusterStatus, s apiconversion.Scope) error {
	// Promote the staged v1beta2 conditions to the v1beta2 status.conditions.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Demote the old v1beta1 conditions to status.deprecated.v1beta1.conditions.
	if len(in.Conditions) > 0 {
		out.Deprecated = &infrav1.HetznerClusterDeprecatedStatus{
			V1Beta1: &infrav1.HetznerClusterV1Beta1DeprecatedStatus{
				Conditions: in.Conditions,
			},
		}
	}

	if in.Network != nil {
		out.Network = &infrav1.NetworkStatus{}
		if err := Convert_v1beta1_NetworkStatus_To_v1beta2_NetworkStatus(in.Network, out.Network, s); err != nil {
			return err
		}
	}

	if in.ControlPlaneLoadBalancer != nil {
		out.ControlPlaneLoadBalancer = &infrav1.LoadBalancerStatus{}
		if err := Convert_v1beta1_LoadBalancerStatus_To_v1beta2_LoadBalancerStatus(in.ControlPlaneLoadBalancer, out.ControlPlaneLoadBalancer, s); err != nil {
			return err
		}
	}

	if in.HCloudPlacementGroups != nil {
		out.HCloudPlacementGroups = make([]infrav1.HCloudPlacementGroupStatus, len(in.HCloudPlacementGroups))
		for i := range in.HCloudPlacementGroups {
			if err := Convert_v1beta1_HCloudPlacementGroupStatus_To_v1beta2_HCloudPlacementGroupStatus(&in.HCloudPlacementGroups[i], &out.HCloudPlacementGroups[i], s); err != nil {
				return err
			}
		}
	}

	out.FailureDomains = convertFailureDomainsToV1Beta2(in.FailureDomains)

	return nil
}

// Convert_v1beta2_HetznerClusterStatus_To_v1beta1_HetznerClusterStatus converts the v1beta2
// HetznerClusterStatus back to v1beta1. It is the inverse of the function above:
//   - status.conditions is demoted to the staged status.v1beta2.conditions.
//   - status.deprecated.v1beta1.conditions is promoted back to status.conditions.
//   - status.initialization.provisioned maps back to status.ready.
func Convert_v1beta2_HetznerClusterStatus_To_v1beta1_HetznerClusterStatus(in *infrav1.HetznerClusterStatus, out *HetznerClusterStatus, s apiconversion.Scope) error {
	// Demote the v1beta2 conditions back to the staged v1beta1 status.v1beta2.conditions.
	if len(in.Conditions) > 0 {
		out.V1Beta2 = &HetznerClusterV1Beta2Status{
			Conditions: in.Conditions,
		}
	}

	// Promote the deprecated v1beta1 conditions back to the old status.conditions.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = in.Deprecated.V1Beta1.Conditions
	}

	if in.Network != nil {
		out.Network = &NetworkStatus{}
		if err := Convert_v1beta2_NetworkStatus_To_v1beta1_NetworkStatus(in.Network, out.Network, s); err != nil {
			return err
		}
	}

	if in.ControlPlaneLoadBalancer != nil {
		out.ControlPlaneLoadBalancer = &LoadBalancerStatus{}
		if err := Convert_v1beta2_LoadBalancerStatus_To_v1beta1_LoadBalancerStatus(in.ControlPlaneLoadBalancer, out.ControlPlaneLoadBalancer, s); err != nil {
			return err
		}
	}

	if in.HCloudPlacementGroups != nil {
		out.HCloudPlacementGroups = make([]HCloudPlacementGroupStatus, len(in.HCloudPlacementGroups))
		for i := range in.HCloudPlacementGroups {
			if err := Convert_v1beta2_HCloudPlacementGroupStatus_To_v1beta1_HCloudPlacementGroupStatus(&in.HCloudPlacementGroups[i], &out.HCloudPlacementGroups[i], s); err != nil {
				return err
			}
		}
	}

	out.FailureDomains = convertFailureDomainsToV1Beta1(in.FailureDomains)

	// status.initialization.provisioned maps back to status.ready during the compatibility window.
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	return nil
}

// convertFailureDomainsToV1Beta2 converts the v1beta1 FailureDomains map into the v1beta2
// FailureDomain slice. Entries are sorted by name so the output is deterministic (the v1beta2 field
// is a +listType=map keyed by name), and each ControlPlane bool is wrapped into a *bool.
func convertFailureDomainsToV1Beta2(in clusterv1beta1.FailureDomains) []clusterv1.FailureDomain {
	if in == nil {
		return nil
	}

	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]clusterv1.FailureDomain, 0, len(in))
	for _, name := range names {
		failureDomain := in[name]
		out = append(out, clusterv1.FailureDomain{
			Name:         name,
			ControlPlane: ptr.To(failureDomain.ControlPlane),
			Attributes:   failureDomain.Attributes,
		})
	}

	return out
}

// convertFailureDomainsToV1Beta1 converts the v1beta2 FailureDomain slice back into the v1beta1
// FailureDomains map, dereferencing each ControlPlane *bool (a nil pointer becomes false).
func convertFailureDomainsToV1Beta1(in []clusterv1.FailureDomain) clusterv1beta1.FailureDomains {
	if in == nil {
		return nil
	}

	out := make(clusterv1beta1.FailureDomains, len(in))
	for _, failureDomain := range in {
		out[failureDomain.Name] = clusterv1beta1.FailureDomainSpec{
			ControlPlane: ptr.Deref(failureDomain.ControlPlane, false),
			Attributes:   failureDomain.Attributes,
		}
	}

	return out
}

func Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in *clusterv1beta1.ObjectMeta, out *clusterv1.ObjectMeta, _ apiconversion.Scope) error {
	out.Labels = in.Labels
	out.Annotations = in.Annotations
	return nil
}

func Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in *clusterv1.ObjectMeta, out *clusterv1beta1.ObjectMeta, _ apiconversion.Scope) error {
	out.Labels = in.Labels
	out.Annotations = in.Annotations
	return nil
}

// Convert_v1beta1_HetznerClusterSpec_To_v1beta2_HetznerClusterSpec converts the v1beta1
// HetznerClusterSpec to v1beta2, mapping the pointer controlPlaneEndpoint to the v1beta2 value type.
func Convert_v1beta1_HetznerClusterSpec_To_v1beta2_HetznerClusterSpec(in *HetznerClusterSpec, out *infrav1.HetznerClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_HetznerClusterSpec_To_v1beta2_HetznerClusterSpec(in, out, s); err != nil {
		return err
	}

	if in.ControlPlaneEndpoint != nil {
		out.ControlPlaneEndpoint = infrav1.APIEndpoint{
			Host: in.ControlPlaneEndpoint.Host,
			Port: in.ControlPlaneEndpoint.Port,
		}
	}

	return nil
}

// Convert_v1beta2_HetznerClusterSpec_To_v1beta1_HetznerClusterSpec converts the v1beta2
// HetznerClusterSpec back to v1beta1, mapping the value controlPlaneEndpoint to the pointer type.
// A zero endpoint maps to a nil pointer.
func Convert_v1beta2_HetznerClusterSpec_To_v1beta1_HetznerClusterSpec(in *infrav1.HetznerClusterSpec, out *HetznerClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_HetznerClusterSpec_To_v1beta1_HetznerClusterSpec(in, out, s); err != nil {
		return err
	}

	if in.ControlPlaneEndpoint != (infrav1.APIEndpoint{}) {
		out.ControlPlaneEndpoint = &clusterv1beta1.APIEndpoint{
			Host: in.ControlPlaneEndpoint.Host,
			Port: in.ControlPlaneEndpoint.Port,
		}
	}

	return nil
}

// Convert_v1beta1_HetznerSSHKeys_To_v1beta2_HetznerSSHKeys converts the v1beta1 HetznerSSHKeys to
// v1beta2, mapping the renamed robotRescueSecretRef field to rescueSecretRef.
func Convert_v1beta1_HetznerSSHKeys_To_v1beta2_HetznerSSHKeys(in *HetznerSSHKeys, out *infrav1.HetznerSSHKeys, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_HetznerSSHKeys_To_v1beta2_HetznerSSHKeys(in, out, s); err != nil {
		return err
	}
	return Convert_v1beta1_SSHSecretRef_To_v1beta2_SSHSecretRef(&in.RobotRescueSecretRef, &out.RescueSecretRef, s)
}

// Convert_v1beta2_HetznerSSHKeys_To_v1beta1_HetznerSSHKeys converts the v1beta2 HetznerSSHKeys back
// to v1beta1, mapping the renamed rescueSecretRef field back to robotRescueSecretRef.
func Convert_v1beta2_HetznerSSHKeys_To_v1beta1_HetznerSSHKeys(in *infrav1.HetznerSSHKeys, out *HetznerSSHKeys, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_HetznerSSHKeys_To_v1beta1_HetznerSSHKeys(in, out, s); err != nil {
		return err
	}
	return Convert_v1beta2_SSHSecretRef_To_v1beta1_SSHSecretRef(&in.RescueSecretRef, &out.RobotRescueSecretRef, s)
}
