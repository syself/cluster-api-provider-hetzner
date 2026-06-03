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
	"math"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
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

// spokeV1Beta2StatusFuzzFuncs normalizes fields that do not round-trip byte-for-byte because the
// two API versions represent empty, staged or reshaped data differently.
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
		func(in *HetznerBareMetalMachineStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.V1Beta2 = nil
		},
		// HCloudRemediation v1beta1 status: keep retryCount in the v1beta2 int32 range,
		// collapse zero lastRemediated and empty condition slices to nil, and drop the V1Beta2
		// wrapper unless it carries conditions.
		func(in *HCloudRemediationStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			in.RetryCount = int(int32(in.RetryCount)) //nolint:gosec // keep fuzz input in the int32 range accepted by conversion

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
		// RemediationStrategy v1beta1: keep retryLimit in the v1beta2 int32 range and
		// quantize durations to the whole-second representation used by v1beta2.
		func(in *RemediationStrategy, c randfill.Continue) {
			c.FillNoCustom(in)
			in.RetryLimit = int(int32(in.RetryLimit)) //nolint:gosec // keep fuzz input in the int32 range accepted by conversion
			in.Timeout = quantizeRemediationDuration(in.Timeout)
			// Timeout is required, and v1beta2 stores it as a non-pointer int32, so a nil
			// timeout down-converts to the zero value. Normalize it here so the round trip matches.
			if in.Timeout == nil {
				in.Timeout = &metav1.Duration{}
			}
			in.Cooldown = quantizeRemediationDuration(in.Cooldown)
		},
		// HCloudRemediation v1beta2 status: nil and zero retryCount both down-convert
		// to the v1beta1 zero value, and empty condition wrappers collapse to nil.
		func(in *infrav1.HCloudRemediationStatus, c randfill.Continue) {
			c.FillNoCustom(in)
			if in.RetryCount != nil && *in.RetryCount == 0 {
				in.RetryCount = nil
			}
			if len(in.Conditions) == 0 {
				in.Conditions = nil
			}
			if in.Deprecated != nil && (in.Deprecated.V1Beta1 == nil || len(in.Deprecated.V1Beta1.Conditions) == 0) {
				in.Deprecated = nil
			}
		},
		// RemediationStrategy v1beta2: nil and zero retryLimit both down-convert to
		// the v1beta1 zero value, so normalize the hub side before comparing.
		func(in *infrav1.RemediationStrategy, c randfill.Continue) {
			c.FillNoCustom(in)
			if in.RetryLimit != nil && *in.RetryLimit == 0 {
				in.RetryLimit = nil
			}
		},
	}
}

// quantizeRemediationDuration rounds a duration to whole seconds within the int32 range accepted
// by the v1beta2 *int32 seconds fields, so the fuzz round trip stays exact.
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
