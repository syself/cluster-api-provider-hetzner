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
	"maps"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

// ConvertTo converts this HetznerCluster to the Hub version (v1beta2).
func (src *HetznerCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.HetznerCluster)
	return Convert_v1beta1_HetznerCluster_To_v1beta2_HetznerCluster(src, dst, nil)
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerCluster.
func (dst *HetznerCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerCluster)
	return Convert_v1beta2_HetznerCluster_To_v1beta1_HetznerCluster(src, dst, nil)
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
	if err := Convert_v1beta1_HetznerBareMetalMachine_To_v1beta2_HetznerBareMetalMachine(src, dst, nil); err != nil {
		return err
	}

	// Recover hub-only data stored by a previous down-conversion so the round trip is lossless.
	restored := &infrav1.HetznerBareMetalMachine{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	// status.ready (bool) maps to status.initialization.provisioned (*bool). The CAPI helper only
	// produces *false when the value was intentionally *false before (restored); otherwise a false
	// ready becomes nil, matching the one-time provisioning signal semantics.
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &dst.Status.Initialization.Provisioned)

	// status.failureReason and status.failureMessage have no home in the v1beta2 InfraMachine shape.
	// Stash the v1beta1 values in the conversion data annotation on the hub so the matching ConvertFrom
	// can restore them and the round trip stays lossless. Note the annotation holds different content
	// depending on the object it lives on: the hub object stored on a v1beta1 object (written by
	// ConvertFrom), and these dropped v1beta1 fields stored on a v1beta2 object (written here).
	if src.Status.FailureReason != nil || src.Status.FailureMessage != nil {
		failure := &HetznerBareMetalMachine{
			Status: HetznerBareMetalMachineStatus{
				FailureReason:  src.Status.FailureReason,
				FailureMessage: src.Status.FailureMessage,
			},
		}
		// dst currently shares src's annotations map (out.ObjectMeta = in.ObjectMeta), so clone it
		// first; otherwise MarshalData would write the annotation back onto src too.
		dst.SetAnnotations(maps.Clone(dst.GetAnnotations()))
		if err := utilconversion.MarshalData(failure, dst); err != nil {
			return err
		}
	}

	return nil
}

// ConvertFrom converts the Hub version (v1beta2) to this HetznerBareMetalMachine.
func (dst *HetznerBareMetalMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.HetznerBareMetalMachine)
	if err := Convert_v1beta2_HetznerBareMetalMachine_To_v1beta1_HetznerBareMetalMachine(src, dst, nil); err != nil {
		return err
	}

	// Restore status.failureReason and status.failureMessage that a previous ConvertTo stashed in the
	// conversion data annotation (they have no v1beta2 home), keeping the round trip lossless.
	failure := &HetznerBareMetalMachine{}
	if ok, err := utilconversion.UnmarshalData(src, failure); err != nil {
		return err
	} else if ok {
		dst.Status.FailureReason = failure.Status.FailureReason
		dst.Status.FailureMessage = failure.Status.FailureMessage
	}

	// Preserve the hub object so the next up-conversion can restore hub-only intent (see ConvertTo).
	return utilconversion.MarshalData(src, dst)
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

// Convert_v1beta1_HetznerBareMetalMachineStatus_To_v1beta2_HetznerBareMetalMachineStatus converts the
// v1beta1 HetznerBareMetalMachineStatus to v1beta2. The v1beta1 status.conditions (old
// clusterv1beta1.Conditions) and the v1beta2 status.conditions ([]metav1.Condition) share a field name
// but not a type and do not correspond, so HetznerBareMetalMachineStatus is excluded from conversion-gen
// (+k8s:conversion-gen=false) and converted fully by hand here:
//   - status.v1beta2.conditions is promoted to status.conditions.
//   - status.conditions is demoted to status.deprecated.v1beta1.conditions.
//   - status.lastUpdated and status.lastRemediatedAt move from pointer to value.
//   - status.failureReason and status.failureMessage are dropped here (no v1beta2 home) and stashed in
//     the conversion data annotation at the object level (HetznerBareMetalMachine.ConvertTo).
//   - status.ready maps to status.initialization.provisioned at the object level
//     (HetznerBareMetalMachine.ConvertTo), because that lossy bool -> *bool mapping needs the restored hub data.
func Convert_v1beta1_HetznerBareMetalMachineStatus_To_v1beta2_HetznerBareMetalMachineStatus(in *HetznerBareMetalMachineStatus, out *infrav1.HetznerBareMetalMachineStatus, _ apiconversion.Scope) error {
	// Promote the staged v1beta2 conditions to the v1beta2 status.conditions.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Demote the old v1beta1 conditions to status.deprecated.v1beta1.conditions.
	if len(in.Conditions) > 0 {
		out.Deprecated = &infrav1.HetznerBareMetalMachineDeprecatedStatus{
			V1Beta1: &infrav1.HetznerBareMetalMachineV1Beta1DeprecatedStatus{
				Conditions: in.Conditions,
			},
		}
	}

	out.Addresses = in.Addresses
	out.Phase = in.Phase

	// lastUpdated and lastRemediatedAt move from a pointer to a value; a nil pointer maps to the zero time.
	if in.LastUpdated != nil {
		out.LastUpdated = *in.LastUpdated
	}
	if in.LastRemediatedAt != nil {
		out.LastRemediatedAt = *in.LastRemediatedAt
	}

	return nil
}

// Convert_v1beta2_HetznerBareMetalMachineStatus_To_v1beta1_HetznerBareMetalMachineStatus converts the
// v1beta2 HetznerBareMetalMachineStatus back to v1beta1. It is the inverse of the function above:
//   - status.conditions is demoted to the staged status.v1beta2.conditions.
//   - status.deprecated.v1beta1.conditions is promoted back to status.conditions.
//   - status.lastUpdated and status.lastRemediatedAt move from value to pointer (zero time -> nil).
//   - status.initialization.provisioned maps back to status.ready.
//   - status.failureReason and status.failureMessage are restored from the conversion data annotation
//     at the object level (HetznerBareMetalMachine.ConvertFrom); they have no v1beta2 source field.
func Convert_v1beta2_HetznerBareMetalMachineStatus_To_v1beta1_HetznerBareMetalMachineStatus(in *infrav1.HetznerBareMetalMachineStatus, out *HetznerBareMetalMachineStatus, _ apiconversion.Scope) error {
	// Demote the v1beta2 conditions back to the staged v1beta1 status.v1beta2.conditions.
	if len(in.Conditions) > 0 {
		out.V1Beta2 = &HetznerBareMetalMachineV1Beta2Status{
			Conditions: in.Conditions,
		}
	}

	// Promote the deprecated v1beta1 conditions back to the old status.conditions.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = in.Deprecated.V1Beta1.Conditions
	}

	out.Addresses = in.Addresses
	out.Phase = in.Phase

	// lastUpdated and lastRemediatedAt move from a value to a pointer; the zero time maps to a nil pointer.
	if !in.LastUpdated.IsZero() {
		lastUpdated := in.LastUpdated
		out.LastUpdated = &lastUpdated
	}
	if !in.LastRemediatedAt.IsZero() {
		lastRemediatedAt := in.LastRemediatedAt
		out.LastRemediatedAt = &lastRemediatedAt
	}

	// status.initialization.provisioned maps back to status.ready during the compatibility window.
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	return nil
}

// Convert_v1beta1_HetznerClusterStatus_To_v1beta2_HetznerClusterStatus converts the v1beta1
// HetznerClusterStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_HetznerClusterStatus_To_v1beta2_HetznerClusterStatus(in *HetznerClusterStatus, out *infrav1.HetznerClusterStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HetznerClusterStatus_To_v1beta2_HetznerClusterStatus(in, out, s)
}
