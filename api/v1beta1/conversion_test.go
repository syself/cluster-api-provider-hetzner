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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
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

func fuzzyConversionTestFunc(scheme *runtime.Scheme, hub conversion.Hub, spoke conversion.Convertible) func(*testing.T) {
	return utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         hub,
		Spoke:       spoke,
		FuzzerFuncs: []fuzzer.FuzzerFuncs{spokeV1Beta2StatusFuzzFuncs},
	})
}

// spokeV1Beta2StatusFuzzFuncs returns custom randfill functions for v1beta1 status types that
// embed a V1Beta2 sub struct (the current home for the new style metav1.Conditions in v1beta1 resources).
// Our hand written status converters in conversion.go drop this sub struct on the way to v1beta2,
// because v1beta2 does not **yet** have a destination for it. If we let the default fuzzer set
// V1Beta2 to random non nil content, the spoke-hub-spoke round trip would fail every time.
// These overrides fill the rest of the status normally and force V1Beta2 to nil, so the fuzz
// test only exercises the lossless parts of the conversion. Once we add a top level
// conditions field and a Deprecated.V1Beta1.Conditions wrapper on v1beta2, the converters become
// lossless and these overrides can be removed.
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
		func(in *HetznerBareMetalMachineStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		func(in *HetznerBareMetalHost, c randfill.Continue) {
			c.FillNoCustom(in)
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
			if in.Spec.ConsumerRef == nil {
				return
			}
			// Keep the fuzzed v1beta2 reference in the group that can be rebuilt as a v1beta1
			// ObjectReference by this package.
			in.Spec.ConsumerRef.APIGroup = infrav1.GroupVersion.Group
		},
	}
}
