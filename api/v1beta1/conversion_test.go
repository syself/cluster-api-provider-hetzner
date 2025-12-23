/*
Copyright 2025 The Kubernetes Authors.

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

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1" // Deprecated, will be removed

	infrav1beta2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

func TestHetznerClusterFailureDomainsConversion(t *testing.T) {
	t.Parallel()

	transitionTime := metav1.Now()
	original := &HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"},
		Status: HetznerClusterStatus{
			FailureDomains: clusterv1.FailureDomains{
				"fd-1": {
					ControlPlane: true,
					Attributes:   map[string]string{"region": "nbg"},
				},
				"fd-2": {
					ControlPlane: false,
				},
			},
			Conditions: clusterv1.Conditions{
				{
					Type:               clusterv1.ConditionType("Ready"),
					Status:             corev1.ConditionTrue,
					Severity:           clusterv1.ConditionSeverityNone,
					LastTransitionTime: transitionTime,
					Reason:             "TestReady",
					Message:            "ready",
				},
			},
		},
	}

	hub := &infrav1beta2.HetznerCluster{}
	if err := original.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo failed: %v", err)
	}

	if hub.Status.Initialization.Provisioned != nil {
		t.Fatalf("expected unset initialization after ConvertTo, got %v", hub.Status.Initialization)
	}

	if len(hub.Status.FailureDomains) != len(original.Status.FailureDomains) {
		t.Fatalf("expected %d failure domains, got %d", len(original.Status.FailureDomains), len(hub.Status.FailureDomains))
	}

	found := map[string]struct{}{}
	for _, domain := range hub.Status.FailureDomains {
		found[domain.Name] = struct{}{}
	}
	for expected := range original.Status.FailureDomains {
		if _, ok := found[expected]; !ok {
			t.Fatalf("failure domain %q missing after ConvertTo", expected)
		}
	}

	restored := &HetznerCluster{}
	if err := restored.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}

	normalizeConditionTimes(original.Status.Conditions)
	normalizeConditionTimes(restored.Status.Conditions)

	if diff := cmp.Diff(original.Status.FailureDomains, restored.Status.FailureDomains); diff != "" {
		t.Fatalf("failure domains mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(original.Status.Conditions, restored.Status.Conditions); diff != "" {
		t.Fatalf("conditions mismatch (-want +got):\n%s", diff)
	}
}

func TestHCloudMachineConditionsConversion(t *testing.T) {
	t.Parallel()

	transitionTime := metav1.Now()
	original := &HCloudMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "machine", Namespace: "default"},
		Status: HCloudMachineStatus{
			Conditions: clusterv1.Conditions{
				{
					Type:               clusterv1.ConditionType("Ready"),
					Status:             corev1.ConditionTrue,
					Severity:           clusterv1.ConditionSeverityNone,
					LastTransitionTime: transitionTime,
					Reason:             "TestReady",
					Message:            "ready",
				},
			},
		},
	}

	hub := &infrav1beta2.HCloudMachine{}
	if err := original.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo failed: %v", err)
	}

	if len(hub.Status.Conditions) != len(original.Status.Conditions) {
		t.Fatalf("expected %d conditions, got %d", len(original.Status.Conditions), len(hub.Status.Conditions))
	}

	if hub.Status.Initialization.Provisioned != nil {
		t.Fatalf("expected unset initialization after ConvertTo, got %v", hub.Status.Initialization)
	}

	restored := &HCloudMachine{}
	if err := restored.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}

	normalizeConditionTimes(original.Status.Conditions)
	normalizeConditionTimes(restored.Status.Conditions)

	if diff := cmp.Diff(original.Status.Conditions, restored.Status.Conditions); diff != "" {
		t.Fatalf("conditions mismatch (-want +got):\n%s", diff)
	}
}

func normalizeConditionTimes(conditions clusterv1.Conditions) {
	for i := range conditions {
		conditions[i].LastTransitionTime = metav1.Time{}
	}
}
