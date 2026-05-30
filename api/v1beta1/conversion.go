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
// Because of that, the spec and object level ConvertTo / ConvertFrom below are just pass throughs
// to the generated converters, and only the status converters need to be hand written.
//
// TODO(#2017): later issues will add new fields that exist only in v1beta2. When we convert from
// v1beta2 to v1beta1, those new fields would be lost. To keep the round trip safe, we will save
// them as an annotation on the v1beta1 object using utilconversion.MarshalData, and restore them
// when converting back to v1beta2.

import (
	"fmt"
	"math"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
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

// HCloudRemediation and the shared RemediationStrategy reshape their fields in v1beta2, so
// their conversions are hand-written here (the v1beta1 types are tagged +k8s:conversion-gen=false).
//
// Status conditions move home: the staged v1beta1 status.v1beta2.conditions become the v1beta2
// status.conditions, and the old v1beta1 status.conditions become status.deprecated.v1beta1.conditions.
// Counters move from int to *int32 (0 maps to nil), lastRemediated moves from a pointer to a value,
// and the RemediationStrategy durations move to whole-second *int32 counters.

// remediationRetryToPointer maps a v1beta1 int counter to the v1beta2 *int32 form. A zero value
// (the v1beta1 "unset" representation) maps to nil so the v1beta2 "not yet computed" state is preserved.
func remediationRetryToPointer(in int) (*int32, error) {
	if in == 0 {
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

// remediationDurationToSeconds converts a duration to whole seconds for the v1beta2 form. A nonzero
// value below one second ceils to 1 so a small cooldown never collapses to 0 (which would disable
// the guard); an explicit 0 stays 0.
func remediationDurationToSeconds(in metav1.Duration) (int32, error) {
	seconds := in.Duration / time.Second
	if seconds == 0 && in.Duration > 0 {
		return 1, nil
	}
	if seconds > math.MaxInt32 || seconds < math.MinInt32 {
		return 0, fmt.Errorf("remediation duration %s is outside the int32 seconds range", in.Duration)
	}
	return int32(seconds), nil //nolint:gosec // checked against the int32 range above
}

// Convert_v1beta1_RemediationStrategy_To_v1beta2_RemediationStrategy converts a v1beta1
// RemediationStrategy to v1beta2, moving the durations to whole-second counters and the retry
// limit to a pointer. It is shared with HetznerBareMetalRemediation.
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
		out.TimeoutSeconds = ptr.To(timeoutSeconds)
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
	if in.TimeoutSeconds != nil {
		out.Timeout = &metav1.Duration{Duration: time.Duration(*in.TimeoutSeconds) * time.Second}
	}
	if in.CooldownSeconds != nil {
		out.Cooldown = &metav1.Duration{Duration: time.Duration(*in.CooldownSeconds) * time.Second}
	}
	return nil
}

// Convert_v1beta1_HCloudRemediationStatus_To_v1beta2_HCloudRemediationStatus promotes the staged
// v1beta2 conditions to status.conditions, demotes the old conditions to
// status.deprecated.v1beta1.conditions, and reshapes the counters and lastRemediated.
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
				Conditions: in.Conditions,
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
		out.Conditions = in.Deprecated.V1Beta1.Conditions
	}
	return nil
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
