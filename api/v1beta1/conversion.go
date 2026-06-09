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
	"math"
	"sort"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Read back the v1beta2 object that ConvertFrom stored in the annotation, so the values v1beta1
	// cannot represent can be restored below. This keeps the round trip lossless.
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

	// Preserve the whole hub (v1beta2) object in a data annotation. ConvertTo reads it back to restore
	// values v1beta1 cannot represent, like provisioned nil vs false, keeping the round trip lossless.
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
	if err := Convert_v1beta1_HCloudMachine_To_v1beta2_HCloudMachine(src, dst, nil); err != nil {
		return err
	}

	// Read back the v1beta2 object that ConvertFrom stored in the annotation, so the values v1beta1
	// cannot represent can be restored below. This keeps the round trip lossless.
	restored := &infrav1.HCloudMachine{}
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

// ConvertFrom converts the Hub version (v1beta2) to this HCloudMachine.
func (dst *HCloudMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HCloudMachine)
	if err := Convert_v1beta2_HCloudMachine_To_v1beta1_HCloudMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve the whole hub (v1beta2) object in a data annotation. ConvertTo reads it back to restore
	// values v1beta1 cannot represent, like provisioned nil vs false, keeping the round trip lossless.
	return utilconversion.MarshalData(src, dst)
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
// HCloudMachineStatus to v1beta2. The v1beta1 status.conditions (old clusterv1beta1.Conditions) and
// the v1beta2 status.conditions ([]metav1.Condition) share a field name but not a type and do not
// correspond, so HCloudMachineStatus is excluded from conversion-gen (+k8s:conversion-gen=false) and
// converted fully by hand here:
//   - status.v1beta2.conditions is promoted to status.conditions.
//   - status.conditions is demoted to status.deprecated.v1beta1.conditions (the old core/v1beta1
//     conditions are converted to the structurally identical core/v1beta2 deprecated conditions).
//   - status.addresses elements change from clusterv1beta1.MachineAddress to clusterv1.MachineAddress.
//   - status.instanceState changes from *hcloud.ServerStatus to the CAPH owned InstanceState value type.
//   - status.lastRemediatedAt moves from a pointer to a value.
//   - status.failureReason and status.failureMessage are dropped: they are deprecated and never
//     populated by CAPH, so they are not carried over to v1beta2.
//   - status.ready maps to status.initialization.provisioned at the object level
//     (HCloudMachine.ConvertTo), because that lossy bool -> *bool mapping needs the restored hub data.
func Convert_v1beta1_HCloudMachineStatus_To_v1beta2_HCloudMachineStatus(in *HCloudMachineStatus, out *infrav1.HCloudMachineStatus, s apiconversion.Scope) error {
	// Promote the staged v1beta2 conditions to the v1beta2 status.conditions.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Demote the old v1beta1 conditions to status.deprecated.v1beta1.conditions.
	if len(in.Conditions) > 0 {
		out.Deprecated = &infrav1.HCloudMachineDeprecatedStatus{
			V1Beta1: &infrav1.HCloudMachineV1Beta1DeprecatedStatus{
				Conditions: convertDeprecatedConditionsToV1Beta2(in.Conditions),
			},
		}
	}

	// Addresses change element type from the deprecated clusterv1beta1.MachineAddress to clusterv1.MachineAddress.
	if in.Addresses != nil {
		out.Addresses = make([]clusterv1.MachineAddress, len(in.Addresses))
		for i := range in.Addresses {
			if err := clusterv1beta1.Convert_v1beta1_MachineAddress_To_v1beta2_MachineAddress(&in.Addresses[i], &out.Addresses[i], s); err != nil {
				return err
			}
		}
	}

	out.Region = infrav1.Region(in.Region)

	if in.SSHKeys != nil {
		out.SSHKeys = make([]infrav1.SSHKey, len(in.SSHKeys))
		for i := range in.SSHKeys {
			if err := Convert_v1beta1_SSHKey_To_v1beta2_SSHKey(&in.SSHKeys[i], &out.SSHKeys[i], s); err != nil {
				return err
			}
		}
	}

	// instanceState changes from *hcloud.ServerStatus to the CAPH owned InstanceState; a nil pointer maps to the empty value.
	if in.InstanceState != nil {
		out.InstanceState = infrav1.InstanceState(*in.InstanceState)
	}

	out.BootState = infrav1.HCloudBootState(in.BootState)
	out.BootStateSince = in.BootStateSince

	if err := Convert_v1beta1_HCloudMachineStatusExternalIDs_To_v1beta2_HCloudMachineStatusExternalIDs(&in.ExternalIDs, &out.ExternalIDs, s); err != nil {
		return err
	}

	// lastRemediatedAt moves from a pointer to a value; a nil pointer maps to the zero time.
	if in.LastRemediatedAt != nil {
		out.LastRemediatedAt = *in.LastRemediatedAt
	}

	return nil
}

// Convert_v1beta2_HCloudMachineStatus_To_v1beta1_HCloudMachineStatus converts the v1beta2
// HCloudMachineStatus back to v1beta1. It is the inverse of the function above:
//   - status.conditions is demoted to the staged status.v1beta2.conditions.
//   - status.deprecated.v1beta1.conditions is promoted back to status.conditions.
//   - status.addresses elements change back to clusterv1beta1.MachineAddress.
//   - status.instanceState changes back to *hcloud.ServerStatus (the empty value maps to a nil pointer).
//   - status.lastRemediatedAt moves from a value to a pointer (zero time -> nil).
//   - status.initialization.provisioned maps back to status.ready.
//   - status.failureReason and status.failureMessage are promoted back from status.deprecated.v1beta1.
func Convert_v1beta2_HCloudMachineStatus_To_v1beta1_HCloudMachineStatus(in *infrav1.HCloudMachineStatus, out *HCloudMachineStatus, s apiconversion.Scope) error {
	// Demote the v1beta2 conditions back to the staged v1beta1 status.v1beta2.conditions.
	if len(in.Conditions) > 0 {
		out.V1Beta2 = &HCloudMachineV1Beta2Status{
			Conditions: in.Conditions,
		}
	}

	// Promote the deprecated v1beta1 conditions back to the old status.conditions.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = convertDeprecatedConditionsToV1Beta1(in.Deprecated.V1Beta1.Conditions)
	}

	if in.Addresses != nil {
		out.Addresses = make([]clusterv1beta1.MachineAddress, len(in.Addresses))
		for i := range in.Addresses {
			if err := clusterv1beta1.Convert_v1beta2_MachineAddress_To_v1beta1_MachineAddress(&in.Addresses[i], &out.Addresses[i], s); err != nil {
				return err
			}
		}
	}

	out.Region = Region(in.Region)

	if in.SSHKeys != nil {
		out.SSHKeys = make([]SSHKey, len(in.SSHKeys))
		for i := range in.SSHKeys {
			if err := Convert_v1beta2_SSHKey_To_v1beta1_SSHKey(&in.SSHKeys[i], &out.SSHKeys[i], s); err != nil {
				return err
			}
		}
	}

	// instanceState changes back from the CAPH owned InstanceState to *hcloud.ServerStatus; the empty value maps to a nil pointer.
	if in.InstanceState != "" {
		instanceState := hcloud.ServerStatus(in.InstanceState)
		out.InstanceState = &instanceState
	}

	out.BootState = HCloudBootState(in.BootState)
	out.BootStateSince = in.BootStateSince

	if err := Convert_v1beta2_HCloudMachineStatusExternalIDs_To_v1beta1_HCloudMachineStatusExternalIDs(&in.ExternalIDs, &out.ExternalIDs, s); err != nil {
		return err
	}

	// lastRemediatedAt moves from a value to a pointer; the zero time maps to a nil pointer.
	if !in.LastRemediatedAt.IsZero() {
		lastRemediatedAt := in.LastRemediatedAt
		out.LastRemediatedAt = &lastRemediatedAt
	}

	// status.initialization.provisioned maps back to status.ready during the compatibility window.
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	return nil
}

// Convert_v1beta1_HCloudMachineTemplateResource_To_v1beta2_HCloudMachineTemplateResource converts the
// v1beta1 HCloudMachineTemplateResource to v1beta2. The template metadata changes type from the
// deprecated clusterv1beta1.ObjectMeta to clusterv1.ObjectMeta, which carry the same labels and
// annotations, so those are copied by hand and the spec uses the generated converter. The resource is
// excluded from conversion-gen (+k8s:conversion-gen=false) so this pass does not depend on a shared
// ObjectMeta converter introduced by another v1beta2 resource pass.
func Convert_v1beta1_HCloudMachineTemplateResource_To_v1beta2_HCloudMachineTemplateResource(in *HCloudMachineTemplateResource, out *infrav1.HCloudMachineTemplateResource, s apiconversion.Scope) error {
	out.ObjectMeta.Labels = in.ObjectMeta.Labels
	out.ObjectMeta.Annotations = in.ObjectMeta.Annotations
	return Convert_v1beta1_HCloudMachineSpec_To_v1beta2_HCloudMachineSpec(&in.Spec, &out.Spec, s)
}

// Convert_v1beta2_HCloudMachineTemplateResource_To_v1beta1_HCloudMachineTemplateResource converts the
// v1beta2 HCloudMachineTemplateResource back to v1beta1, mapping the clusterv1.ObjectMeta labels and
// annotations back to the deprecated clusterv1beta1.ObjectMeta.
func Convert_v1beta2_HCloudMachineTemplateResource_To_v1beta1_HCloudMachineTemplateResource(in *infrav1.HCloudMachineTemplateResource, out *HCloudMachineTemplateResource, s apiconversion.Scope) error {
	out.ObjectMeta.Labels = in.ObjectMeta.Labels
	out.ObjectMeta.Annotations = in.ObjectMeta.Annotations
	return Convert_v1beta2_HCloudMachineSpec_To_v1beta1_HCloudMachineSpec(&in.Spec, &out.Spec, s)
}

// Convert_v1beta1_HCloudMachineTemplateStatus_To_v1beta2_HCloudMachineTemplateStatus converts
// the v1beta1 HCloudMachineTemplateStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_HCloudMachineTemplateStatus_To_v1beta2_HCloudMachineTemplateStatus(in *HCloudMachineTemplateStatus, out *infrav1.HCloudMachineTemplateStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HCloudMachineTemplateStatus_To_v1beta2_HCloudMachineTemplateStatus(in, out, s)
}

// remediationRetryToPointer maps a v1beta1 int counter to the v1beta2 *int32 form. v1beta1 stores an
// unset counter as 0, while v1beta2 stores it as a nil pointer, so non-positive values map to nil.
func remediationRetryToPointer(in int) (*int32, error) {
	if in <= 0 {
		return nil, nil
	}
	if in > math.MaxInt32 || in < math.MinInt32 {
		return nil, fmt.Errorf("remediation retry counter %d is outside the int32 range", in)
	}
	return ptr.To(int32(in)), nil
}

// remediationRetryFromPointer maps a v1beta2 *int32 counter back to the v1beta1 int form.
func remediationRetryFromPointer(in *int32) int {
	if in == nil {
		return 0
	}
	return int(*in)
}

// remediationDurationToSeconds converts a duration to whole seconds for the v1beta2 form. Negative
// values clamp to 0 to satisfy the v1beta2 schema, and a positive value below one second ceils to 1
// so a small configured duration never collapses to 0.
func remediationDurationToSeconds(in metav1.Duration) (int32, error) {
	if in.Duration < 0 {
		return 0, nil
	}
	seconds := in.Duration / time.Second
	if seconds == 0 && in.Duration > 0 {
		return 1, nil
	}
	if seconds > math.MaxInt32 {
		return 0, fmt.Errorf("remediation duration %s is outside the int32 seconds range", in.Duration)
	}
	return int32(seconds), nil //nolint:gosec // checked against the int32 range above
}

// Convert_v1beta1_RemediationStrategy_To_v1beta2_RemediationStrategy converts a v1beta1
// RemediationStrategy to v1beta2. It is hand-written (the type is tagged +k8s:conversion-gen=false)
// because v1beta2 stores the timeout and cooldown as whole-second *int32 counters and the retry limit
// as a *int32. It is shared with HetznerBareMetalRemediation.
func Convert_v1beta1_RemediationStrategy_To_v1beta2_RemediationStrategy(in *RemediationStrategy, out *infrav1.RemediationStrategy, _ apiconversion.Scope) error {
	var err error
	out.Type = infrav1.RemediationType(in.Type)
	out.RetryLimit, err = remediationRetryToPointer(in.RetryLimit)
	if err != nil {
		return err
	}
	if in.Timeout != nil {
		timeoutSeconds, err := remediationDurationToSeconds(*in.Timeout)
		if err != nil {
			return err
		}
		out.TimeoutSeconds = timeoutSeconds
	}
	if in.Cooldown != nil {
		cooldownSeconds, err := remediationDurationToSeconds(*in.Cooldown)
		if err != nil {
			return err
		}
		out.CooldownSeconds = ptr.To(cooldownSeconds)
	}
	return nil
}

// Convert_v1beta2_RemediationStrategy_To_v1beta1_RemediationStrategy converts a v1beta2
// RemediationStrategy back to v1beta1, restoring the durations from the whole-second counters.
func Convert_v1beta2_RemediationStrategy_To_v1beta1_RemediationStrategy(in *infrav1.RemediationStrategy, out *RemediationStrategy, _ apiconversion.Scope) error {
	out.Type = RemediationType(in.Type)
	out.RetryLimit = remediationRetryFromPointer(in.RetryLimit)
	out.Timeout = &metav1.Duration{Duration: time.Duration(in.TimeoutSeconds) * time.Second}
	if in.CooldownSeconds != nil {
		out.Cooldown = &metav1.Duration{Duration: time.Duration(*in.CooldownSeconds) * time.Second}
	}
	return nil
}

// Convert_v1beta1_HCloudRemediationStatus_To_v1beta2_HCloudRemediationStatus is hand-written (the type
// is tagged +k8s:conversion-gen=false) because the conditions and counters change shape between
// versions. It promotes the staged status.v1beta2.conditions to status.conditions, demotes the old
// status.conditions to status.deprecated.v1beta1.conditions, and maps retryCount and lastRemediated to
// their v1beta2 forms.
func Convert_v1beta1_HCloudRemediationStatus_To_v1beta2_HCloudRemediationStatus(in *HCloudRemediationStatus, out *infrav1.HCloudRemediationStatus, _ apiconversion.Scope) error {
	var err error
	out.Phase = in.Phase
	out.RetryCount, err = remediationRetryToPointer(in.RetryCount)
	if err != nil {
		return err
	}
	if in.LastRemediated != nil {
		out.LastRemediated = *in.LastRemediated
	}
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}
	if len(in.Conditions) > 0 {
		out.Deprecated = &infrav1.HCloudRemediationDeprecatedStatus{
			V1Beta1: &infrav1.HCloudRemediationV1Beta1DeprecatedStatus{
				Conditions: convertDeprecatedConditionsToV1Beta2(in.Conditions),
			},
		}
	}
	return nil
}

// Convert_v1beta2_HCloudRemediationStatus_To_v1beta1_HCloudRemediationStatus restores the staged
// v1beta2 conditions and the old condition slice from their v1beta2 homes.
func Convert_v1beta2_HCloudRemediationStatus_To_v1beta1_HCloudRemediationStatus(in *infrav1.HCloudRemediationStatus, out *HCloudRemediationStatus, _ apiconversion.Scope) error {
	out.Phase = in.Phase
	out.RetryCount = remediationRetryFromPointer(in.RetryCount)
	if !in.LastRemediated.IsZero() {
		lastRemediated := in.LastRemediated
		out.LastRemediated = &lastRemediated
	}
	if len(in.Conditions) > 0 {
		out.V1Beta2 = &HCloudRemediationV1Beta2Status{Conditions: in.Conditions}
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = convertDeprecatedConditionsToV1Beta1(in.Deprecated.V1Beta1.Conditions)
	}
	return nil
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
				Conditions: convertDeprecatedConditionsToV1Beta2(in.Conditions),
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
		out.Conditions = convertDeprecatedConditionsToV1Beta1(in.Deprecated.V1Beta1.Conditions)
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

// convertDeprecatedConditionsToV1Beta2 converts the old core/v1beta1 conditions (carried by the
// v1beta1 status) into the structurally identical core/v1beta2 deprecated conditions stored under
// status.deprecated.v1beta1.conditions on the v1beta2 object. The two Condition types are
// field-for-field identical; only the package-local Type and Severity typedefs need a cast.
func convertDeprecatedConditionsToV1Beta2(in clusterv1beta1.Conditions) clusterv1.Conditions {
	if in == nil {
		return nil
	}

	out := make(clusterv1.Conditions, len(in))
	for i := range in {
		out[i] = clusterv1.Condition{
			Type:               clusterv1.ConditionType(in[i].Type),
			Status:             in[i].Status,
			Severity:           clusterv1.ConditionSeverity(in[i].Severity),
			LastTransitionTime: in[i].LastTransitionTime,
			Reason:             in[i].Reason,
			Message:            in[i].Message,
		}
	}

	return out
}

// convertDeprecatedConditionsToV1Beta1 is the inverse of convertDeprecatedConditionsToV1Beta2:
// it converts the core/v1beta2 deprecated conditions back into the core/v1beta1 conditions that the
// v1beta1 status.conditions field uses.
func convertDeprecatedConditionsToV1Beta1(in clusterv1.Conditions) clusterv1beta1.Conditions {
	if in == nil {
		return nil
	}

	out := make(clusterv1beta1.Conditions, len(in))
	for i := range in {
		out[i] = clusterv1beta1.Condition{
			Type:               clusterv1beta1.ConditionType(in[i].Type),
			Status:             in[i].Status,
			Severity:           clusterv1beta1.ConditionSeverity(in[i].Severity),
			LastTransitionTime: in[i].LastTransitionTime,
			Reason:             in[i].Reason,
			Message:            in[i].Message,
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
