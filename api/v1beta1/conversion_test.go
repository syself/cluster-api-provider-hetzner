/*
Copyright 2026 The Kubernetes Authors.

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
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/randfill"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

// TestFuzzyConversion checks that converting a CAPH object between v1beta1 and v1beta2 is
// lossless in both directions. For every root kind it runs two sub tests provided by
// utilconversion.FuzzTestFunc:
//
//  1. spoke-hub-spoke: fill a v1beta1 object with random data, convert it up to v1beta2, convert
//     it back down to v1beta1, and assert the result equals the original. This is the path the
//     apiserver takes when storage is v1beta1 and a client reads or writes as v1beta2.
//
//  2. hub-spoke-hub: fill a v1beta2 object with random data, convert it down to v1beta1, convert
//     it back up to v1beta2, and assert the result equals the original. This is the path the
//     apiserver takes when storage is v1beta2 and a client reads or writes as v1beta1.
//
// Each sub test runs 10000 iterations per kind, so any field the converters drop, reorder, or
// mistype will be caught here.
func TestFuzzyConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta1 to scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta2 to scheme: %v", err)
	}

	t.Run("for HetznerCluster", fuzzyConversionTestFunc(scheme, &infrav1.HetznerCluster{}, &HetznerCluster{}))
	t.Run("for HetznerClusterTemplate", fuzzyConversionTestFunc(scheme, &infrav1.HetznerClusterTemplate{}, &HetznerClusterTemplate{}))
	t.Run("for HCloudMachine", fuzzyConversionTestFunc(scheme, &infrav1.HCloudMachine{}, &HCloudMachine{}))
	t.Run("for HCloudMachineTemplate", fuzzyConversionTestFunc(scheme, &infrav1.HCloudMachineTemplate{}, &HCloudMachineTemplate{}))
	t.Run("for HCloudRemediation", fuzzyConversionTestFunc(scheme, &infrav1.HCloudRemediation{}, &HCloudRemediation{}))
	t.Run("for HCloudRemediationTemplate", fuzzyConversionTestFunc(scheme, &infrav1.HCloudRemediationTemplate{}, &HCloudRemediationTemplate{}))
	t.Run("for HetznerBareMetalHost", fuzzyConversionTestFunc(scheme, &infrav1.HetznerBareMetalHost{}, &HetznerBareMetalHost{}))
	t.Run("for HetznerBareMetalMachine", fuzzyConversionTestFunc(scheme, &infrav1.HetznerBareMetalMachine{}, &HetznerBareMetalMachine{}))
	t.Run("for HetznerBareMetalMachineTemplate", fuzzyConversionTestFunc(scheme, &infrav1.HetznerBareMetalMachineTemplate{}, &HetznerBareMetalMachineTemplate{}))
	t.Run("for HetznerBareMetalRemediation", fuzzyConversionTestFunc(scheme, &infrav1.HetznerBareMetalRemediation{}, &HetznerBareMetalRemediation{}))
	t.Run("for HetznerBareMetalRemediationTemplate", fuzzyConversionTestFunc(scheme, &infrav1.HetznerBareMetalRemediationTemplate{}, &HetznerBareMetalRemediationTemplate{}))
}

// These focused ConsumerRef tests cover the intentional non-round-tripped parts of the v1beta2
// shape change. The fuzzy conversion test still gives broad round-trip coverage, but its custom
// fuzzer below normalizes fields outside the v1beta2 reference shape. These tests make that
// smaller reference contract explicit.

// TestConvertHetznerBareMetalHostConsumerRefToV1Beta2 verifies that v1beta1 ConsumerRef converts
// to a v1beta2 reference with kind, name, and API group.
func TestConvertHetznerBareMetalHostConsumerRefToV1Beta2(t *testing.T) {
	src := &HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-a",
			Namespace: "host-namespace",
		},
		Spec: HetznerBareMetalHostSpec{
			ServerID: 1,
			ConsumerRef: &corev1.ObjectReference{
				APIVersion:      GroupVersion.String(),
				Kind:            "HetznerBareMetalMachine",
				Namespace:       "host-namespace",
				Name:            "machine-a",
				UID:             types.UID("uid-a"),
				ResourceVersion: "12345",
				FieldPath:       "spec.consumer",
			},
		},
	}
	dst := &infrav1.HetznerBareMetalHost{}

	if err := Convert_v1beta1_HetznerBareMetalHost_To_v1beta2_HetznerBareMetalHost(src, dst, nil); err != nil {
		t.Fatalf("failed to convert HetznerBareMetalHost to v1beta2: %v", err)
	}
	if dst.Spec.ConsumerRef == nil {
		t.Fatal("expected ConsumerRef to be converted")
	}
	if dst.Spec.ConsumerRef.Kind != "HetznerBareMetalMachine" {
		t.Fatalf("expected kind to be preserved, got %q", dst.Spec.ConsumerRef.Kind)
	}
	if dst.Spec.ConsumerRef.Name != "machine-a" {
		t.Fatalf("expected name to be preserved, got %q", dst.Spec.ConsumerRef.Name)
	}
	if dst.Spec.ConsumerRef.APIGroup != GroupVersion.Group {
		t.Fatalf("expected apiGroup %q, got %q", GroupVersion.Group, dst.Spec.ConsumerRef.APIGroup)
	}

	consumerRefJSON, err := json.Marshal(dst.Spec.ConsumerRef)
	if err != nil {
		t.Fatalf("failed to marshal converted ConsumerRef: %v", err)
	}
	var consumerRefFields map[string]interface{}
	if err := json.Unmarshal(consumerRefJSON, &consumerRefFields); err != nil {
		t.Fatalf("failed to unmarshal converted ConsumerRef: %v", err)
	}
	for _, omittedField := range []string{"apiVersion", "namespace", "uid", "resourceVersion", "fieldPath"} {
		if _, ok := consumerRefFields[omittedField]; ok {
			t.Fatalf("expected %q to be omitted from v1beta2 ConsumerRef", omittedField)
		}
	}
}

// TestConvertHetznerBareMetalHostConsumerRefToV1Beta1RestoresNamespace verifies that converting
// from v1beta2 restores the v1beta1 ConsumerRef namespace from the host namespace.
func TestConvertHetznerBareMetalHostConsumerRefToV1Beta1RestoresNamespace(t *testing.T) {
	src := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-a",
			Namespace: "host-namespace",
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			ServerID: 1,
			ConsumerRef: &infrav1.HetznerBareMetalHostConsumerReference{
				Kind:     "HetznerBareMetalMachine",
				Name:     "machine-a",
				APIGroup: infrav1.GroupVersion.Group,
			},
		},
	}
	dst := &HetznerBareMetalHost{}

	if err := Convert_v1beta2_HetznerBareMetalHost_To_v1beta1_HetznerBareMetalHost(src, dst, nil); err != nil {
		t.Fatalf("failed to convert HetznerBareMetalHost to v1beta1: %v", err)
	}
	if dst.Spec.ConsumerRef == nil {
		t.Fatal("expected ConsumerRef to be converted")
	}
	if dst.Spec.ConsumerRef.Kind != "HetznerBareMetalMachine" {
		t.Fatalf("expected kind to be preserved, got %q", dst.Spec.ConsumerRef.Kind)
	}
	if dst.Spec.ConsumerRef.Name != "machine-a" {
		t.Fatalf("expected name to be preserved, got %q", dst.Spec.ConsumerRef.Name)
	}
	if dst.Spec.ConsumerRef.APIVersion != GroupVersion.String() {
		t.Fatalf("expected apiVersion %q, got %q", GroupVersion.String(), dst.Spec.ConsumerRef.APIVersion)
	}
	if dst.Spec.ConsumerRef.Namespace != "host-namespace" {
		t.Fatalf("expected namespace to be restored from host namespace, got %q", dst.Spec.ConsumerRef.Namespace)
	}
}

// TestConvertHetznerBareMetalHostConsumerRefNil verifies that nil ConsumerRef values stay nil in
// both conversion directions.
func TestConvertHetznerBareMetalHostConsumerRefNil(t *testing.T) {
	toV1Beta2 := &infrav1.HetznerBareMetalHostSpec{}
	if err := Convert_v1beta1_HetznerBareMetalHostSpec_To_v1beta2_HetznerBareMetalHostSpec(&HetznerBareMetalHostSpec{}, toV1Beta2, nil); err != nil {
		t.Fatalf("failed to convert nil ConsumerRef to v1beta2: %v", err)
	}
	if toV1Beta2.ConsumerRef != nil {
		t.Fatal("expected nil ConsumerRef to stay nil when converting to v1beta2")
	}

	toV1Beta1 := &HetznerBareMetalHostSpec{}
	if err := Convert_v1beta2_HetznerBareMetalHostSpec_To_v1beta1_HetznerBareMetalHostSpec(&infrav1.HetznerBareMetalHostSpec{}, toV1Beta1, nil); err != nil {
		t.Fatalf("failed to convert nil ConsumerRef to v1beta1: %v", err)
	}
	if toV1Beta1.ConsumerRef != nil {
		t.Fatal("expected nil ConsumerRef to stay nil when converting to v1beta1")
	}
}

// TestConvertHetznerBareMetalHostConsumerRefInvalidAPIVersion verifies that an invalid v1beta1
// ConsumerRef APIVersion fails conversion to v1beta2.
func TestConvertHetznerBareMetalHostConsumerRefInvalidAPIVersion(t *testing.T) {
	src := &HetznerBareMetalHostSpec{
		ConsumerRef: &corev1.ObjectReference{
			APIVersion: "invalid/group/version",
			Kind:       "HetznerBareMetalMachine",
			Name:       "machine-a",
		},
	}
	dst := &infrav1.HetznerBareMetalHostSpec{}

	if err := Convert_v1beta1_HetznerBareMetalHostSpec_To_v1beta2_HetznerBareMetalHostSpec(src, dst, nil); err == nil {
		t.Fatal("expected invalid ConsumerRef apiVersion to fail conversion")
	}
}

// TestHetznerBareMetalHostConvertToMovesStatusToSubresource verifies that converting a v1beta1
// HetznerBareMetalHost to v1beta2 moves spec.status into the status subresource, promotes the staged
// v1beta2 conditions to status.conditions, demotes the v1beta1 conditions to
// status.deprecated.v1beta1.conditions, and maps rebootTriggeredAt from a pointer to a value.
func TestHetznerBareMetalHostConvertToMovesStatusToSubresource(t *testing.T) {
	legacyConditions := clusterv1beta1.Conditions{
		{
			Type:               clusterv1beta1.ConditionType("LegacyReady"),
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyReady",
			Message:            "legacy condition",
		},
	}
	v1beta2Conditions := []metav1.Condition{
		{
			Type:               clusterv1beta1.ReadyV1Beta2Condition,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Unix(2, 0),
			Reason:             clusterv1beta1.ReadyV1Beta2Reason,
			Message:            "ready",
		},
	}
	wantDeprecatedConditions := clusterv1.Conditions{
		{
			Type:               "LegacyReady",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyReady",
			Message:            "legacy condition",
		},
	}
	rebootTriggeredAt := metav1.Unix(3, 0)

	src := &HetznerBareMetalHost{
		Spec: HetznerBareMetalHostSpec{
			ServerID: 42,
			Status: ControllerGeneratedStatus{
				IPv4:              "1.2.3.4",
				IPv6:              "2001:db8::1",
				ProvisioningState: StateProvisioned,
				ErrorType:         FatalError,
				RebootTypes:       []RebootType{RebootTypeSoftware, RebootTypeHardware},
				RebootTriggeredAt: &rebootTriggeredAt,
				NodeBootID:        "boot-id",
				Conditions:        legacyConditions,
				V1Beta2:           &HetznerBareMetalHostV1Beta2Status{Conditions: v1beta2Conditions},
			},
		},
	}

	dst := &infrav1.HetznerBareMetalHost{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("failed to convert to v1beta2: %v", err)
	}

	if dst.Status.IPv4 != "1.2.3.4" || dst.Status.IPv6 != "2001:db8::1" {
		t.Fatalf("status addresses not moved to the subresource: %#v", dst.Status)
	}
	if dst.Status.ProvisioningState != infrav1.StateProvisioned || dst.Status.ErrorType != infrav1.FatalError {
		t.Fatalf("status fields not moved to the subresource: %#v", dst.Status)
	}
	if dst.Status.NodeBootID != "boot-id" {
		t.Fatalf("status.nodeBootID not moved: %q", dst.Status.NodeBootID)
	}
	if !reflect.DeepEqual(dst.Status.RebootTypes, []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}) {
		t.Fatalf("status.rebootTypes mismatch: %#v", dst.Status.RebootTypes)
	}
	if !dst.Status.RebootTriggeredAt.Equal(&rebootTriggeredAt) {
		t.Fatalf("status.rebootTriggeredAt mismatch: %#v", dst.Status.RebootTriggeredAt)
	}
	if !reflect.DeepEqual(dst.Status.Conditions, v1beta2Conditions) {
		t.Fatalf("status.conditions mismatch:\n got: %#v\nwant: %#v", dst.Status.Conditions, v1beta2Conditions)
	}
	if !reflect.DeepEqual(dst.GetV1Beta1Conditions(), wantDeprecatedConditions) {
		t.Fatalf("deprecated v1beta1 conditions were not preserved: %#v", dst.Status.Deprecated)
	}
}

// TestHetznerBareMetalHostConvertFromMovesStatusToSpec verifies that converting a v1beta2
// HetznerBareMetalHost back to v1beta1 moves the status subresource into spec.status, demotes the
// v1beta2 conditions to the staged status.v1beta2.conditions, and promotes the deprecated v1beta1
// conditions back to spec.status.conditions.
func TestHetznerBareMetalHostConvertFromMovesStatusToSpec(t *testing.T) {
	deprecatedConditions := clusterv1.Conditions{
		{
			Type:               "LegacyReady",
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyNotReady",
			Message:            "legacy condition",
		},
	}
	legacyConditions := clusterv1beta1.Conditions{
		{
			Type:               clusterv1beta1.ConditionType("LegacyReady"),
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyNotReady",
			Message:            "legacy condition",
		},
	}
	v1beta2Conditions := []metav1.Condition{
		{
			Type:               clusterv1beta1.ReadyV1Beta2Condition,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Unix(2, 0),
			Reason:             clusterv1beta1.NotReadyV1Beta2Reason,
			Message:            "not ready",
		},
	}

	src := &infrav1.HetznerBareMetalHost{
		Spec: infrav1.HetznerBareMetalHostSpec{ServerID: 7},
		Status: infrav1.HetznerBareMetalHostStatus{
			IPv4:       "5.6.7.8",
			Conditions: v1beta2Conditions,
			Deprecated: &infrav1.HetznerBareMetalHostDeprecatedStatus{
				V1Beta1: &infrav1.HetznerBareMetalHostV1Beta1DeprecatedStatus{
					Conditions: deprecatedConditions,
				},
			},
		},
	}

	dst := &HetznerBareMetalHost{}
	if err := dst.ConvertFrom(src); err != nil {
		t.Fatalf("failed to convert from v1beta2: %v", err)
	}

	if dst.Spec.Status.IPv4 != "5.6.7.8" {
		t.Fatalf("status.ipv4 not moved back to spec.status: %q", dst.Spec.Status.IPv4)
	}
	if dst.Spec.Status.V1Beta2 == nil || !reflect.DeepEqual(dst.Spec.Status.V1Beta2.Conditions, v1beta2Conditions) {
		t.Fatalf("v1beta2 conditions were not staged on v1beta1: %#v", dst.Spec.Status.V1Beta2)
	}
	if !reflect.DeepEqual(dst.Spec.Status.Conditions, legacyConditions) {
		t.Fatalf("legacy conditions mismatch:\n got: %#v\nwant: %#v", dst.Spec.Status.Conditions, legacyConditions)
	}
}

// TestHetznerBareMetalHostRoundTripPreservesDroppedStatusFields verifies that the spec.status fields
// with no v1beta2 equivalent survive a v1beta1 -> v1beta2 -> v1beta1 round trip through the conversion data
// annotation, including the nested hardwareDetails.cpu.flags.
func TestHetznerBareMetalHostRoundTripPreservesDroppedStatusFields(t *testing.T) {
	lastUpdated := metav1.Unix(10, 0)
	src := &HetznerBareMetalHost{
		Spec: HetznerBareMetalHostSpec{
			ServerID: 1,
			Status: ControllerGeneratedStatus{
				HetznerClusterRef: "cluster-a",
				UserData:          &corev1.SecretReference{Name: "user-data", Namespace: "default"},
				InstallImage:      &InstallImage{Image: Image{Name: "image", URL: "oci://example/image:1", Path: "/root.tar.gz"}},
				SSHSpec:           &SSHSpec{PortAfterInstallImage: 22, PortAfterCloudInit: 22},
				ErrorCount:        3,
				ErrorMessage:      "boom",
				LastUpdated:       &lastUpdated,
				HardwareDetails:   &HardwareDetails{RAMGB: 64, CPU: CPU{Threads: 8, Flags: []string{"sse", "avx"}}},
			},
		},
	}

	hub := &infrav1.HetznerBareMetalHost{}
	if err := src.ConvertTo(hub); err != nil {
		t.Fatalf("failed to convert to v1beta2: %v", err)
	}

	restored := &HetznerBareMetalHost{}
	if err := restored.ConvertFrom(hub); err != nil {
		t.Fatalf("failed to convert back to v1beta1: %v", err)
	}

	status := restored.Spec.Status
	if status.HetznerClusterRef != "cluster-a" {
		t.Fatalf("hetznerClusterRef not restored: %q", status.HetznerClusterRef)
	}
	if !reflect.DeepEqual(status.UserData, src.Spec.Status.UserData) {
		t.Fatalf("userData not restored: %#v", status.UserData)
	}
	if !reflect.DeepEqual(status.InstallImage, src.Spec.Status.InstallImage) {
		t.Fatalf("installImage not restored: %#v", status.InstallImage)
	}
	if !reflect.DeepEqual(status.SSHSpec, src.Spec.Status.SSHSpec) {
		t.Fatalf("sshSpec not restored: %#v", status.SSHSpec)
	}
	if status.ErrorCount != 3 || status.ErrorMessage != "boom" {
		t.Fatalf("error fields not restored: count=%d message=%q", status.ErrorCount, status.ErrorMessage)
	}
	if status.LastUpdated == nil || !status.LastUpdated.Equal(&lastUpdated) {
		t.Fatalf("lastUpdated not restored: %#v", status.LastUpdated)
	}
	if status.HardwareDetails == nil || !reflect.DeepEqual(status.HardwareDetails.CPU.Flags, []string{"sse", "avx"}) {
		t.Fatalf("hardwareDetails.cpu.flags not restored: %#v", status.HardwareDetails)
	}
}

// TestHetznerBareMetalHostRebootTriggeredAtNilRoundTrip verifies that a nil spec.status.rebootTriggeredAt
// in v1beta1 converts to the zero value in v1beta2 and back to a nil pointer in v1beta1.
func TestHetznerBareMetalHostRebootTriggeredAtNilRoundTrip(t *testing.T) {
	src := &HetznerBareMetalHost{
		Spec: HetznerBareMetalHostSpec{
			ServerID: 1,
			Status:   ControllerGeneratedStatus{RebootTriggeredAt: nil},
		},
	}

	hub := &infrav1.HetznerBareMetalHost{}
	if err := src.ConvertTo(hub); err != nil {
		t.Fatalf("failed to convert to v1beta2: %v", err)
	}
	if !hub.Status.RebootTriggeredAt.IsZero() {
		t.Fatalf("expected a zero rebootTriggeredAt in v1beta2, got %#v", hub.Status.RebootTriggeredAt)
	}

	restored := &HetznerBareMetalHost{}
	if err := restored.ConvertFrom(hub); err != nil {
		t.Fatalf("failed to convert back to v1beta1: %v", err)
	}
	if restored.Spec.Status.RebootTriggeredAt != nil {
		t.Fatalf("expected a nil rebootTriggeredAt in v1beta1, got %#v", restored.Spec.Status.RebootTriggeredAt)
	}
}

func fuzzyConversionTestFunc(scheme *runtime.Scheme, hub conversion.Hub, spoke conversion.Convertible) func(*testing.T) {
	return utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         hub,
		Spoke:       spoke,
		FuzzerFuncs: []fuzzer.FuzzerFuncs{spokeV1Beta2StatusFuzzFuncs},
	})
}

// TestRemediationRetryToPointer verifies the v1beta1 int to v1beta2 *int32 counter
// conversion, including the compatibility rule that v1beta1 zero maps to nil.
func TestRemediationRetryToPointer(t *testing.T) {
	tests := []struct {
		name    string
		in      int
		want    *int32
		wantErr bool
	}{
		{
			name: "zero maps to nil",
			in:   0,
			want: nil,
		},
		{
			name: "negative value maps to nil",
			in:   -1,
			want: nil,
		},
		{
			name: "positive value",
			in:   3,
			want: ptr.To[int32](3),
		},
		{
			name:    "value above int32 range fails",
			in:      int(math.MaxInt32) + 1,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := remediationRetryToPointer(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %d", *got)
				}
				return
			}
			if got == nil || *got != *tc.want {
				t.Fatalf("expected %d, got %v", *tc.want, got)
			}
		})
	}
}

// TestRemediationDurationToSeconds verifies the v1beta1 duration to v1beta2 seconds
// conversion, including the explicit sub-second ceiling and int32 range check.
func TestRemediationDurationToSeconds(t *testing.T) {
	tests := []struct {
		name    string
		in      time.Duration
		want    int32
		wantErr bool
	}{
		{
			name: "zero stays zero",
			in:   0,
			want: 0,
		},
		{
			name: "negative value clamps to zero",
			in:   -1 * time.Second,
			want: 0,
		},
		{
			name: "positive sub-second value ceils to one",
			in:   500 * time.Millisecond,
			want: 1,
		},
		{
			name: "whole seconds stay unchanged",
			in:   45 * time.Second,
			want: 45,
		},
		{
			name:    "value above int32 seconds range fails",
			in:      time.Duration(math.MaxInt32+1) * time.Second,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := remediationDurationToSeconds(metav1.Duration{Duration: tc.in})
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

// TestHetznerClusterConvertToPromoteV1Beta2Shape verifies that converting a v1beta1
// HetznerCluster to v1beta2 promotes the staged v1beta2 fields and maps renamed or
// reshaped contract fields into the final v1beta2 API shape.
func TestHetznerClusterConvertToPromoteV1Beta2Shape(t *testing.T) {
	legacyConditions := clusterv1beta1.Conditions{
		{
			Type:               clusterv1beta1.ConditionType("LegacyReady"),
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyReady",
			Message:            "legacy condition",
		},
	}
	v1beta2Conditions := []metav1.Condition{
		{
			Type:               clusterv1beta1.ReadyV1Beta2Condition,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Unix(2, 0),
			Reason:             clusterv1beta1.ReadyV1Beta2Reason,
			Message:            "ready",
		},
	}
	// The v1beta1 status.conditions (core/v1beta1) are demoted to status.deprecated.v1beta1.conditions,
	// which the v1beta2 object stores as the structurally identical core/v1beta2 copy.
	wantDeprecatedConditions := clusterv1.Conditions{
		{
			Type:               "LegacyReady",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyReady",
			Message:            "legacy condition",
		},
	}

	src := &HetznerCluster{
		Spec: HetznerClusterSpec{
			ControlPlaneEndpoint: &clusterv1beta1.APIEndpoint{Host: "1.2.3.4", Port: 6443},
			SSHKeys: HetznerSSHKeys{
				RobotRescueSecretRef: SSHSecretRef{
					Name: "rescue-ssh",
					Key: SSHSecretKeyRef{
						Name:       "name",
						PublicKey:  "public",
						PrivateKey: "private",
					},
				},
			},
		},
		Status: HetznerClusterStatus{
			Ready:      true,
			Conditions: legacyConditions,
			FailureDomains: clusterv1beta1.FailureDomains{
				"nbg1": {ControlPlane: false},
				"fsn1": {ControlPlane: true, Attributes: map[string]string{"zone": "eu-central"}},
			},
			V1Beta2: &HetznerClusterV1Beta2Status{
				Conditions: v1beta2Conditions,
			},
		},
	}

	dst := &infrav1.HetznerCluster{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("failed to convert to v1beta2: %v", err)
	}

	if !reflect.DeepEqual(dst.Status.Conditions, v1beta2Conditions) {
		t.Fatalf("v1beta2 status.conditions mismatch:\n got: %#v\nwant: %#v", dst.Status.Conditions, v1beta2Conditions)
	}
	if !reflect.DeepEqual(dst.GetV1Beta1Conditions(), wantDeprecatedConditions) {
		t.Fatalf("deprecated v1beta1 conditions were not preserved: %#v", dst.Status.Deprecated)
	}
	if dst.Status.Initialization.Provisioned == nil || !*dst.Status.Initialization.Provisioned {
		t.Fatalf("status.initialization.provisioned = %v, want true", dst.Status.Initialization.Provisioned)
	}
	if dst.Spec.ControlPlaneEndpoint.Host != "1.2.3.4" || dst.Spec.ControlPlaneEndpoint.Port != 6443 {
		t.Fatalf("controlPlaneEndpoint mismatch: %#v", dst.Spec.ControlPlaneEndpoint)
	}
	if !reflect.DeepEqual(dst.Spec.SSHKeys.RescueSecretRef, infrav1.SSHSecretRef{
		Name: "rescue-ssh",
		Key: infrav1.SSHSecretKeyRef{
			Name:       "name",
			PublicKey:  "public",
			PrivateKey: "private",
		},
	}) {
		t.Fatalf("rescueSecretRef mismatch: %#v", dst.Spec.SSHKeys.RescueSecretRef)
	}

	expectedFailureDomains := []clusterv1.FailureDomain{
		{Name: "fsn1", ControlPlane: ptr.To(true), Attributes: map[string]string{"zone": "eu-central"}},
		{Name: "nbg1", ControlPlane: ptr.To(false)},
	}
	if !reflect.DeepEqual(dst.Status.FailureDomains, expectedFailureDomains) {
		t.Fatalf("failureDomains mismatch:\n got: %#v\nwant: %#v", dst.Status.FailureDomains, expectedFailureDomains)
	}
}

// TestHetznerClusterConvertFromDemoteV1Beta2Shape verifies that converting a v1beta2
// HetznerCluster back to v1beta1 demotes v1beta2-only fields into the compatibility
// locations used by the v1beta1 API.
func TestHetznerClusterConvertFromDemoteV1Beta2Shape(t *testing.T) {
	// deprecatedConditions are stored on the v1beta2 object as the core/v1beta2 copy; after demotion
	// they become the structurally identical core/v1beta1 conditions on the v1beta1 status.conditions.
	deprecatedConditions := clusterv1.Conditions{
		{
			Type:               "LegacyReady",
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyNotReady",
			Message:            "legacy condition",
		},
	}
	legacyConditions := clusterv1beta1.Conditions{
		{
			Type:               clusterv1beta1.ConditionType("LegacyReady"),
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Unix(1, 0),
			Reason:             "LegacyNotReady",
			Message:            "legacy condition",
		},
	}
	v1beta2Conditions := []metav1.Condition{
		{
			Type:               clusterv1beta1.ReadyV1Beta2Condition,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Unix(2, 0),
			Reason:             clusterv1beta1.NotReadyV1Beta2Reason,
			Message:            "not ready",
		},
	}

	src := &infrav1.HetznerCluster{
		Spec: infrav1.HetznerClusterSpec{
			ControlPlaneEndpoint: infrav1.APIEndpoint{Host: "5.6.7.8", Port: 8443},
			SSHKeys: infrav1.HetznerSSHKeys{
				RescueSecretRef: infrav1.SSHSecretRef{
					Name: "rescue-ssh",
					Key: infrav1.SSHSecretKeyRef{
						Name:       "name",
						PublicKey:  "public",
						PrivateKey: "private",
					},
				},
			},
		},
		Status: infrav1.HetznerClusterStatus{
			Conditions: v1beta2Conditions,
			Initialization: infrav1.HetznerClusterInitializationStatus{
				Provisioned: ptr.To(true),
			},
			FailureDomains: []clusterv1.FailureDomain{
				{Name: "nbg1", ControlPlane: nil},
				{Name: "fsn1", ControlPlane: ptr.To(true), Attributes: map[string]string{"zone": "eu-central"}},
			},
			Deprecated: &infrav1.HetznerClusterDeprecatedStatus{
				V1Beta1: &infrav1.HetznerClusterV1Beta1DeprecatedStatus{
					Conditions: deprecatedConditions,
				},
			},
		},
	}

	dst := &HetznerCluster{}
	if err := dst.ConvertFrom(src); err != nil {
		t.Fatalf("failed to convert from v1beta2: %v", err)
	}

	if dst.Status.V1Beta2 == nil || !reflect.DeepEqual(dst.Status.V1Beta2.Conditions, v1beta2Conditions) {
		t.Fatalf("v1beta2 conditions were not staged on v1beta1: %#v", dst.Status.V1Beta2)
	}
	if !reflect.DeepEqual(dst.Status.Conditions, legacyConditions) {
		t.Fatalf("legacy conditions mismatch:\n got: %#v\nwant: %#v", dst.Status.Conditions, legacyConditions)
	}
	if !dst.Status.Ready {
		t.Fatal("status.ready = false, want true")
	}
	if dst.Spec.ControlPlaneEndpoint == nil ||
		dst.Spec.ControlPlaneEndpoint.Host != "5.6.7.8" ||
		dst.Spec.ControlPlaneEndpoint.Port != 8443 {
		t.Fatalf("controlPlaneEndpoint mismatch: %#v", dst.Spec.ControlPlaneEndpoint)
	}
	if !reflect.DeepEqual(dst.Spec.SSHKeys.RobotRescueSecretRef, SSHSecretRef{
		Name: "rescue-ssh",
		Key: SSHSecretKeyRef{
			Name:       "name",
			PublicKey:  "public",
			PrivateKey: "private",
		},
	}) {
		t.Fatalf("robotRescueSecretRef mismatch: %#v", dst.Spec.SSHKeys.RobotRescueSecretRef)
	}

	expectedFailureDomains := clusterv1beta1.FailureDomains{
		"fsn1": {ControlPlane: true, Attributes: map[string]string{"zone": "eu-central"}},
		"nbg1": {ControlPlane: false},
	}
	if !reflect.DeepEqual(dst.Status.FailureDomains, expectedFailureDomains) {
		t.Fatalf("failureDomains mismatch:\n got: %#v\nwant: %#v", dst.Status.FailureDomains, expectedFailureDomains)
	}
}

// TestHetznerClusterRoundTripPreservesFalseProvisionedIntent verifies that the lossy
// status.ready to status.initialization.provisioned conversion preserves an explicit
// false provisioned value through the stored hub annotation.
func TestHetznerClusterRoundTripPreservesFalseProvisionedIntent(t *testing.T) {
	src := &infrav1.HetznerCluster{
		Status: infrav1.HetznerClusterStatus{
			Initialization: infrav1.HetznerClusterInitializationStatus{
				Provisioned: ptr.To(false),
			},
		},
	}

	spoke := &HetznerCluster{}
	if err := spoke.ConvertFrom(src); err != nil {
		t.Fatalf("failed to convert from v1beta2: %v", err)
	}

	restored := &infrav1.HetznerCluster{}
	if err := spoke.ConvertTo(restored); err != nil {
		t.Fatalf("failed to convert back to v1beta2: %v", err)
	}

	if restored.Status.Initialization.Provisioned == nil || *restored.Status.Initialization.Provisioned {
		t.Fatalf("status.initialization.provisioned = %v, want false", restored.Status.Initialization.Provisioned)
	}
}

// TestHetznerClusterConvertToNilProvisionedForFalseReadyWithoutAnnotation verifies the
// storage-migration path: a v1beta1 HetznerCluster with status.ready=false and no stored hub
// annotation converts to status.initialization.provisioned=nil, since a false ready without a
// restored hub cannot be distinguished from "never provisioned".
func TestHetznerClusterConvertToNilProvisionedForFalseReadyWithoutAnnotation(t *testing.T) {
	src := &HetznerCluster{
		Status: HetznerClusterStatus{
			Ready: false,
		},
	}

	dst := &infrav1.HetznerCluster{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("failed to convert to v1beta2: %v", err)
	}

	if dst.Status.Initialization.Provisioned != nil {
		t.Fatalf("status.initialization.provisioned = %v, want nil", dst.Status.Initialization.Provisioned)
	}
}

func normalizeV1Beta2FailureDomains(in []clusterv1.FailureDomain) []clusterv1.FailureDomain {
	if in == nil {
		return nil
	}

	failureDomainsByName := make(map[string]clusterv1.FailureDomain, len(in))
	for _, failureDomain := range in {
		if failureDomain.ControlPlane == nil {
			failureDomain.ControlPlane = ptr.To(false)
		}
		failureDomainsByName[failureDomain.Name] = failureDomain
	}

	names := make([]string, 0, len(failureDomainsByName))
	for name := range failureDomainsByName {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]clusterv1.FailureDomain, 0, len(failureDomainsByName))
	for _, name := range names {
		out = append(out, failureDomainsByName[name])
	}

	return out
}

// spokeV1Beta2StatusFuzzFuncs normalizes fields that do not round-trip byte-for-byte because the
// two API versions represent empty or staged data differently.
func spokeV1Beta2StatusFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(in *HCloudMachineStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		func(in *HCloudMachineTemplateStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		func(in *HetznerBareMetalMachineStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		// HCloudRemediation v1beta1 status: keep retryCount in the non-negative v1beta2 int32 range,
		// collapse zero lastRemediated and empty condition slices to nil, and drop the V1Beta2
		// wrapper unless it carries conditions.
		func(in *HCloudRemediationStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.RetryCount = int(int32(in.RetryCount)) //nolint:gosec // keep fuzz input in the int32 range accepted by conversion
			if in.RetryCount < 0 {
				in.RetryCount = 0
			}

			if in.LastRemediated != nil && in.LastRemediated.IsZero() {
				in.LastRemediated = nil
			}
			if len(in.Conditions) == 0 {
				in.Conditions = nil
			}
			if in.V1Beta2 != nil && len(in.V1Beta2.Conditions) == 0 {
				in.V1Beta2 = nil
			}
		},
		// HetznerCluster v1beta1 status: collapse empty condition slices to nil, and drop the V1Beta2
		// wrapper unless it carries conditions, so the bare v1beta2 status.conditions round trips.
		func(in *HetznerClusterStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			if len(in.Conditions) == 0 {
				in.Conditions = nil
			}
			if in.V1Beta2 != nil && len(in.V1Beta2.Conditions) == 0 {
				in.V1Beta2 = nil
			}
		},
		// RemediationStrategy v1beta1: keep retryLimit in the non-negative v1beta2 int32 range and
		// quantize durations to the non-negative whole-second representation used by v1beta2.
		func(in *RemediationStrategy, c randfill.Continue) {
			c.FillNoCustom(in)
			in.RetryLimit = int(int32(in.RetryLimit)) //nolint:gosec // keep fuzz input in the int32 range accepted by conversion
			if in.RetryLimit < 0 {
				in.RetryLimit = 0
			}
			in.Timeout = quantizeRemediationDuration(in.Timeout)
			// Timeout is required, and v1beta2 stores it as a non-pointer int32, so a nil
			// timeout down-converts to the zero value. Normalize it here so the round trip matches.
			if in.Timeout == nil {
				in.Timeout = &metav1.Duration{}
			}
			in.Cooldown = quantizeRemediationDuration(in.Cooldown)
		},
		// HCloudRemediation v1beta2 status: nil and non-positive retryCount both down-convert
		// to the v1beta1 zero value, and empty condition wrappers collapse to nil.
		func(in *infrav1.HCloudRemediationStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			if in.RetryCount != nil && *in.RetryCount <= 0 {
				in.RetryCount = nil
			}
			if len(in.Conditions) == 0 {
				in.Conditions = nil
			}
			if in.Deprecated != nil && (in.Deprecated.V1Beta1 == nil || len(in.Deprecated.V1Beta1.Conditions) == 0) {
				in.Deprecated = nil
			}
		},
		// RemediationStrategy v1beta2: nil and non-positive retryLimit both down-convert to
		// the v1beta1 zero value, and negative second counters clamp to zero.
		func(in *infrav1.RemediationStrategy, c randfill.Continue) {
			c.FillNoCustom(in)
			if in.RetryLimit != nil && *in.RetryLimit <= 0 {
				in.RetryLimit = nil
			}
			if in.TimeoutSeconds < 0 {
				in.TimeoutSeconds = 0
			}
			if in.CooldownSeconds != nil && *in.CooldownSeconds < 0 {
				in.CooldownSeconds = ptr.To(int32(0))
			}
		},
		// HetznerCluster v1beta1 spec: the fuzzer can produce a pointer to an empty controlPlaneEndpoint
		// (&{}). v1beta2 stores this field as a value type with omitzero, so &{} converts up to the zero
		// value and back down to nil, which would fail the round trip. Collapse it to nil up front.
		//
		// This is safe because &{} and nil mean the same thing here: no endpoint is set. The controller
		// only ever writes a real endpoint (host and port) or leaves it unset, never &{}, and the v1beta2
		// type uses omitzero with MinProperties=1, so an empty endpoint is already treated as unset.
		func(in *HetznerClusterSpec, c randfill.Continue) {
			c.FillNoCustom(in)
			if in.ControlPlaneEndpoint != nil && in.ControlPlaneEndpoint.Host == "" && in.ControlPlaneEndpoint.Port == 0 {
				in.ControlPlaneEndpoint = nil
			}
		},
		// HetznerCluster v1beta2 status (hub side): conditions, deprecated and failureDomains have an
		// empty-vs-nil (or ordering) ambiguity, so normalize them to make the round trip match.
		// provisioned is left as-is: ConvertTo/ConvertFrom preserve it losslessly via the MarshalData
		// annotation, so nil, *true and *false all survive.
		func(in *infrav1.HetznerClusterStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			if len(in.Conditions) == 0 {
				in.Conditions = nil
			}
			if in.Deprecated == nil || in.Deprecated.V1Beta1 == nil || len(in.Deprecated.V1Beta1.Conditions) == 0 {
				in.Deprecated = nil
			}
			in.FailureDomains = normalizeV1Beta2FailureDomains(in.FailureDomains)
		},
		func(in *HetznerBareMetalHost, c randfill.Continue) {
			c.FillNoCustom(in)

			// v1beta1 keeps the controller-generated status in spec.status. The v1beta2 status carries
			// the bare metav1 conditions, so collapse empty condition slices to nil and drop the staged
			// V1Beta2 wrapper unless it carries conditions, so the conditions round trip. rebootTriggeredAt
			// moves to a value type and lastUpdated round trips through the conversion data annotation,
			// so collapse a pointer to the zero time to nil in both cases.
			status := &in.Spec.Status
			if len(status.Conditions) == 0 {
				status.Conditions = nil
			}
			if status.V1Beta2 != nil && len(status.V1Beta2.Conditions) == 0 {
				status.V1Beta2 = nil
			}
			if status.RebootTriggeredAt != nil && status.RebootTriggeredAt.IsZero() {
				status.RebootTriggeredAt = nil
			}
			if status.LastUpdated != nil && status.LastUpdated.IsZero() {
				status.LastUpdated = nil
			}

			if in.Spec.ConsumerRef == nil {
				return
			}
			// v1beta2 does not persist these ObjectReference-only fields. Keep the fuzzed
			// v1beta1 object inside the lossless subset so the round-trip test still checks the
			// rest of the host conversion.
			in.Spec.ConsumerRef.APIVersion = GroupVersion.String()
			in.Spec.ConsumerRef.Namespace = in.Namespace
			in.Spec.ConsumerRef.UID = ""
			in.Spec.ConsumerRef.ResourceVersion = ""
			in.Spec.ConsumerRef.FieldPath = ""
		},
		func(in *infrav1.HetznerBareMetalHost, c randfill.Continue) {
			c.FillNoCustom(in)

			// conditions and deprecated have an empty-vs-nil ambiguity, so normalize them to make the
			// round trip match.
			if len(in.Status.Conditions) == 0 {
				in.Status.Conditions = nil
			}
			if in.Status.Deprecated == nil || in.Status.Deprecated.V1Beta1 == nil || len(in.Status.Deprecated.V1Beta1.Conditions) == 0 {
				in.Status.Deprecated = nil
			}

			if in.Spec.ConsumerRef == nil {
				return
			}
			// Keep the fuzzed v1beta2 reference in the group that can be rebuilt as a v1beta1
			// ObjectReference by this package.
			in.Spec.ConsumerRef.APIGroup = infrav1.GroupVersion.Group
		},
	}
}

// quantizeRemediationDuration rounds a duration to non-negative whole seconds within the int32 range
// accepted by the v1beta2 *int32 seconds fields, so the fuzz round trip stays exact.
func quantizeRemediationDuration(d *metav1.Duration) *metav1.Duration {
	if d == nil {
		return nil
	}
	seconds := int64(d.Duration / time.Second)
	if seconds == 0 && d.Duration > 0 {
		seconds = 1
	}
	if seconds < 0 {
		seconds = 0
	}
	if seconds > math.MaxInt32 {
		seconds = math.MaxInt32
	}
	return &metav1.Duration{Duration: time.Duration(seconds) * time.Second}
}
