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

// v1beta1 and v1beta2 have almost the same fields today. The only difference is that v1beta1
// status types have a nested V1Beta2 field for the new metav1.Conditions.
//
// v1beta2 still keeps the old v1beta1 conditions at status.conditions. The later status-shape work
// will move those old conditions to status.deprecated.v1beta1.conditions and promote the v1beta1
// status.v1beta2.conditions field to v1beta2 status.conditions. This conversion plumbing keeps
// that field move out of scope.
//
// Because of that, most spec and object level ConvertTo / ConvertFrom below are just pass throughs
// to the generated converters. Status converters and selected v1beta2 API shape changes need hand
// written conversion.
//
// TODO(#2017): later issues will add new fields that exist only in v1beta2. When we convert from
// v1beta2 to v1beta1, those new fields would be lost. To keep the round trip safe, we will save
// them as an annotation on the v1beta1 object using utilconversion.MarshalData, and restore them
// when converting back to v1beta2.

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// conversion-gen generates public Convert_... wrappers only when **every** field can be mapped.
// If a field has no peer in the other version, it only generates autoConvert_... helpers for the
// matching fields and leaves the unmatched field for manual conversion. Here, that unmatched field
// is v1beta1's V1Beta2.
//
// The generated helpers contain:
//
//	WARNING: in.V1Beta2 requires manual conversion: does not exist in peer-type
//
// That warning is expected in this PR. v1beta2 still uses status.conditions for the old v1beta1
// clusterv1.Conditions, so copying V1Beta2.Conditions there would mix two condition formats. The
// per-resource status migration will add the correct destination fields:
// status.conditions for metav1.Conditions and status.deprecated.v1beta1.conditions for the old
// clusterv1.Conditions. Until then, the safest conversion is to copy the shared fields and leave
// the staged v1beta1 V1Beta2 field behind.

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
// HetznerClusterStatus to v1beta2, dropping the V1Beta2 field.
func Convert_v1beta1_HetznerClusterStatus_To_v1beta2_HetznerClusterStatus(in *HetznerClusterStatus, out *infrav1.HetznerClusterStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_HetznerClusterStatus_To_v1beta2_HetznerClusterStatus(in, out, s)
}
