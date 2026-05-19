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
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

func TestFuzzyConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta1 to scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta2 to scheme: %v", err)
	}

	t.Run("for HetznerCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HetznerCluster{},
		Spoke:  &HetznerCluster{},
	}))
	t.Run("for HetznerClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HetznerClusterTemplate{},
		Spoke:  &HetznerClusterTemplate{},
	}))
	t.Run("for HCloudMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HCloudMachine{},
		Spoke:  &HCloudMachine{},
	}))
	t.Run("for HCloudMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HCloudMachineTemplate{},
		Spoke:  &HCloudMachineTemplate{},
	}))
	t.Run("for HCloudRemediation", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HCloudRemediation{},
		Spoke:  &HCloudRemediation{},
	}))
	t.Run("for HCloudRemediationTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HCloudRemediationTemplate{},
		Spoke:  &HCloudRemediationTemplate{},
	}))
	t.Run("for HetznerBareMetalHost", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HetznerBareMetalHost{},
		Spoke:  &HetznerBareMetalHost{},
	}))
	t.Run("for HetznerBareMetalMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HetznerBareMetalMachine{},
		Spoke:  &HetznerBareMetalMachine{},
	}))
	t.Run("for HetznerBareMetalMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HetznerBareMetalMachineTemplate{},
		Spoke:  &HetznerBareMetalMachineTemplate{},
	}))
	t.Run("for HetznerBareMetalRemediation", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HetznerBareMetalRemediation{},
		Spoke:  &HetznerBareMetalRemediation{},
	}))
	t.Run("for HetznerBareMetalRemediationTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.HetznerBareMetalRemediationTemplate{},
		Spoke:  &HetznerBareMetalRemediationTemplate{},
	}))
}
