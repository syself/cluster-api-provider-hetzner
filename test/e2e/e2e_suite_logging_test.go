/*
Copyright 2022 The Kubernetes Authors.

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

package e2e

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func TestIsCCMDeploymentName(t *testing.T) {
	t.Parallel()

	testCases := map[string]bool{
		"ccm":                             true,
		"hcloud-cloud-controller-manager": true,
		"syself-ccm-hetzner":              true,
		"some-ccm-hcloud":                 true,
		"coredns":                         false,
		"controller-manager":              false,
	}

	for deploymentName, want := range testCases {
		if got := isCCMDeploymentName(deploymentName); got != want {
			t.Fatalf("isCCMDeploymentName(%q) = %t, want %t", deploymentName, got, want)
		}
	}
}

func TestCCMContainerNames(t *testing.T) {
	t.Parallel()

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init-a"},
				{Name: "shared"},
			},
			Containers: []corev1.Container{
				{Name: "shared"},
				{Name: "manager"},
			},
		},
	}

	got := ccmContainerNames(pod)
	want := []string{"init-a", "shared", "manager"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ccmContainerNames() = %v, want %v", got, want)
	}
}

func TestCollectNewCCMLogLinesDedupesAndHighlights(t *testing.T) {
	t.Parallel()

	state := newCCMLogState(time.Unix(0, 0))
	firstBatch, err := collectNewCCMLogLines(
		strings.NewReader("plain line\nerror E1234 happened\nplain line\n"),
		"wl-cluster test",
		"ccm-pod",
		"manager",
		state,
	)
	if err != nil {
		t.Fatalf("collectNewCCMLogLines() unexpected error: %v", err)
	}
	if len(firstBatch) != 2 {
		t.Fatalf("collectNewCCMLogLines() returned %d lines, want 2", len(firstBatch))
	}
	if strings.Contains(firstBatch[0], "\x1b[") {
		t.Fatalf("plain line should not be highlighted: %q", firstBatch[0])
	}
	if !strings.Contains(firstBatch[1], "\x1b[1;30;43m") {
		t.Fatalf("error line should be highlighted: %q", firstBatch[1])
	}

	secondBatch, err := collectNewCCMLogLines(
		strings.NewReader("plain line\nerror E1234 happened\nfresh line\n"),
		"wl-cluster test",
		"ccm-pod",
		"manager",
		state,
	)
	if err != nil {
		t.Fatalf("collectNewCCMLogLines() unexpected error on second batch: %v", err)
	}
	if len(secondBatch) != 1 {
		t.Fatalf("collectNewCCMLogLines() second batch returned %d lines, want 1", len(secondBatch))
	}
	if !strings.Contains(secondBatch[0], "fresh line") {
		t.Fatalf("expected fresh line in second batch, got %q", secondBatch[0])
	}
}

func TestFormatCCMPodStateLinesIncludesSchedulingAndContainerState(t *testing.T) {
	t.Parallel()

	pod := corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  "Unschedulable",
					Message: "0/2 nodes are available",
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "manager",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "ContainerCreating",
							Message: "creating",
						},
					},
				},
			},
		},
	}
	pod.Name = "ccm-pod"

	lines := formatCCMPodStateLines("wl-cluster test", pod)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "phase=Pending") {
		t.Fatalf("expected pod phase in output, got %q", joined)
	}
	if !strings.Contains(joined, "condition PodScheduled=False reason=Unschedulable") {
		t.Fatalf("expected PodScheduled condition in output, got %q", joined)
	}
	if !strings.Contains(joined, "container manager waiting reason=ContainerCreating") {
		t.Fatalf("expected waiting container state in output, got %q", joined)
	}
}

func TestCollectNewCCMPodStateLinesDedupes(t *testing.T) {
	t.Parallel()

	state := newCCMLogState(time.Unix(0, 0))
	pod := corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
	pod.Name = "ccm-pod"

	firstBatch := collectNewCCMPodStateLines("wl-cluster test", pod, state)
	if len(firstBatch) == 0 {
		t.Fatal("expected initial pod state lines")
	}

	secondBatch := collectNewCCMPodStateLines("wl-cluster test", pod, state)
	if len(secondBatch) != 0 {
		t.Fatalf("expected duplicate pod state lines to be suppressed, got %v", secondBatch)
	}
}

func TestPodSupportsLogStreaming(t *testing.T) {
	t.Parallel()

	if podSupportsLogStreaming(corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}) {
		t.Fatal("pending pod should not support log streaming")
	}
	if !podSupportsLogStreaming(corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}) {
		t.Fatal("running pod should support log streaming")
	}
}
