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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/randfill"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

// TestFuzzyConversion checks that converting a CAPH object between v1beta1 and v1beta2 is
// lossless in both directions. For every root kind it runs two sub tests provided by CAPI's
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

func fuzzyConversionTestFunc(scheme *runtime.Scheme, hub conversion.Hub, spoke conversion.Convertible) func(*testing.T) {
	return utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         hub,
		Spoke:       spoke,
		FuzzerFuncs: []fuzzer.FuzzerFuncs{spokeV1Beta2StatusFuzzFuncs},
	})
}

// TestHetznerBareMetalMachineConvertToPromoteV1Beta2Shape verifies that converting a v1beta1
// HetznerBareMetalMachine to v1beta2 promotes the staged v1beta2 conditions, demotes the old
// v1beta1 conditions, maps status.ready to status.initialization.provisioned, and moves the
// pointer timestamps to their value form.
func TestHetznerBareMetalMachineConvertToPromoteV1Beta2Shape(t *testing.T) {
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
	lastUpdated := metav1.Unix(3, 0)
	lastRemediatedAt := metav1.Unix(4, 0)

	src := &HetznerBareMetalMachine{
		Status: HetznerBareMetalMachineStatus{
			Ready:            true,
			Conditions:       legacyConditions,
			LastUpdated:      &lastUpdated,
			LastRemediatedAt: &lastRemediatedAt,
			V1Beta2: &HetznerBareMetalMachineV1Beta2Status{
				Conditions: v1beta2Conditions,
			},
		},
	}

	dst := &infrav1.HetznerBareMetalMachine{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("failed to convert to v1beta2: %v", err)
	}

	if !reflect.DeepEqual(dst.Status.Conditions, v1beta2Conditions) {
		t.Fatalf("v1beta2 status.conditions mismatch:\n got: %#v\nwant: %#v", dst.Status.Conditions, v1beta2Conditions)
	}
	if !reflect.DeepEqual(dst.GetV1Beta1Conditions(), legacyConditions) {
		t.Fatalf("deprecated v1beta1 conditions were not preserved: %#v", dst.Status.Deprecated)
	}
	if dst.Status.Initialization.Provisioned == nil || !*dst.Status.Initialization.Provisioned {
		t.Fatalf("status.initialization.provisioned = %v, want true", dst.Status.Initialization.Provisioned)
	}
	if !dst.Status.LastUpdated.Equal(&lastUpdated) {
		t.Fatalf("lastUpdated mismatch: got %v, want %v", dst.Status.LastUpdated, lastUpdated)
	}
	if !dst.Status.LastRemediatedAt.Equal(&lastRemediatedAt) {
		t.Fatalf("lastRemediatedAt mismatch: got %v, want %v", dst.Status.LastRemediatedAt, lastRemediatedAt)
	}
}

// TestHetznerBareMetalMachineConvertFromDemoteV1Beta2Shape verifies that converting a v1beta2
// HetznerBareMetalMachine back to v1beta1 demotes v1beta2-only fields into the compatibility
// locations used by the v1beta1 API.
func TestHetznerBareMetalMachineConvertFromDemoteV1Beta2Shape(t *testing.T) {
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
	lastUpdated := metav1.Unix(3, 0)
	lastRemediatedAt := metav1.Unix(4, 0)

	src := &infrav1.HetznerBareMetalMachine{
		Status: infrav1.HetznerBareMetalMachineStatus{
			Conditions: v1beta2Conditions,
			Initialization: infrav1.HetznerBareMetalMachineInitializationStatus{
				Provisioned: ptr.To(true),
			},
			LastUpdated:      lastUpdated,
			LastRemediatedAt: lastRemediatedAt,
			Deprecated: &infrav1.HetznerBareMetalMachineDeprecatedStatus{
				V1Beta1: &infrav1.HetznerBareMetalMachineV1Beta1DeprecatedStatus{
					Conditions: legacyConditions,
				},
			},
		},
	}

	dst := &HetznerBareMetalMachine{}
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
	if dst.Status.LastUpdated == nil || !dst.Status.LastUpdated.Equal(&lastUpdated) {
		t.Fatalf("lastUpdated mismatch: %#v", dst.Status.LastUpdated)
	}
	if dst.Status.LastRemediatedAt == nil || !dst.Status.LastRemediatedAt.Equal(&lastRemediatedAt) {
		t.Fatalf("lastRemediatedAt mismatch: %#v", dst.Status.LastRemediatedAt)
	}
}

// TestHetznerBareMetalMachineRoundTripPreservesFalseProvisionedIntent verifies that the lossy
// status.ready to status.initialization.provisioned conversion preserves an explicit false
// provisioned value through the stored hub annotation.
func TestHetznerBareMetalMachineRoundTripPreservesFalseProvisionedIntent(t *testing.T) {
	src := &infrav1.HetznerBareMetalMachine{
		Status: infrav1.HetznerBareMetalMachineStatus{
			Initialization: infrav1.HetznerBareMetalMachineInitializationStatus{
				Provisioned: ptr.To(false),
			},
		},
	}

	spoke := &HetznerBareMetalMachine{}
	if err := spoke.ConvertFrom(src); err != nil {
		t.Fatalf("failed to convert from v1beta2: %v", err)
	}

	restored := &infrav1.HetznerBareMetalMachine{}
	if err := spoke.ConvertTo(restored); err != nil {
		t.Fatalf("failed to convert back to v1beta2: %v", err)
	}

	if restored.Status.Initialization.Provisioned == nil || *restored.Status.Initialization.Provisioned {
		t.Fatalf("status.initialization.provisioned = %v, want false", restored.Status.Initialization.Provisioned)
	}
}

// TestHetznerBareMetalMachineRoundTripPreservesFailureFields verifies that status.failureReason and
// status.failureMessage, which are dropped from the v1beta2 type, survive a v1beta1 -> v1beta2 ->
// v1beta1 round trip via the conversion data annotation.
func TestHetznerBareMetalMachineRoundTripPreservesFailureFields(t *testing.T) {
	src := &HetznerBareMetalMachine{
		Status: HetznerBareMetalMachineStatus{
			FailureReason:  ptr.To("boom"),
			FailureMessage: ptr.To("it broke"),
		},
	}

	hub := &infrav1.HetznerBareMetalMachine{}
	if err := src.ConvertTo(hub); err != nil {
		t.Fatalf("failed to convert to v1beta2: %v", err)
	}

	dst := &HetznerBareMetalMachine{}
	if err := dst.ConvertFrom(hub); err != nil {
		t.Fatalf("failed to convert from v1beta2: %v", err)
	}

	if dst.Status.FailureReason == nil || *dst.Status.FailureReason != "boom" {
		t.Fatalf("status.failureReason = %v, want boom", dst.Status.FailureReason)
	}
	if dst.Status.FailureMessage == nil || *dst.Status.FailureMessage != "it broke" {
		t.Fatalf("status.failureMessage = %v, want it broke", dst.Status.FailureMessage)
	}
}

// spokeV1Beta2StatusFuzzFuncs normalizes fields that do not round-trip byte-for-byte because the
// two API versions represent empty or staged data differently.
func spokeV1Beta2StatusFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(in *ControllerGeneratedStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		func(in *HetznerClusterStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		func(in *HCloudMachineStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		func(in *HCloudMachineTemplateStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		func(in *HCloudRemediationStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		// HetznerBareMetalMachine v1beta1 status: collapse empty condition slices to nil, drop the
		// V1Beta2 wrapper unless it carries conditions, and collapse a non-nil pointer to the zero time
		// to nil so the round trip matches. failureReason/failureMessage are NOT normalized here: they
		// are dropped from the v1beta2 type but stashed in the conversion data annotation, so the round
		// trip preserves them losslessly.
		func(in *HetznerBareMetalMachineStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			if len(in.Conditions) == 0 {
				in.Conditions = nil
			}
			if in.V1Beta2 != nil && len(in.V1Beta2.Conditions) == 0 {
				in.V1Beta2 = nil
			}
			if in.LastUpdated != nil && in.LastUpdated.IsZero() {
				in.LastUpdated = nil
			}
			if in.LastRemediatedAt != nil && in.LastRemediatedAt.IsZero() {
				in.LastRemediatedAt = nil
			}
		},
		// HetznerBareMetalMachine v1beta2 status (hub side): conditions and the deprecated wrapper have
		// an empty-vs-nil ambiguity, so normalize them to make the round trip match. provisioned is left
		// as-is: ConvertTo/ConvertFrom preserve it losslessly via the MarshalData annotation, so nil,
		// *true and *false all survive.
		func(in *infrav1.HetznerBareMetalMachineStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			if len(in.Conditions) == 0 {
				in.Conditions = nil
			}
			if in.Deprecated == nil || in.Deprecated.V1Beta1 == nil || len(in.Deprecated.V1Beta1.Conditions) == 0 {
				in.Deprecated = nil
			}
		},
	}
}
