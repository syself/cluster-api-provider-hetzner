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
