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

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func TestMachineExternalIP_FromMachineStatus(t *testing.T) {
	t.Parallel()

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
		},
		Status: clusterv1.MachineStatus{
			Addresses: []clusterv1.MachineAddress{
				{
					Type:    clusterv1.MachineExternalIP,
					Address: "203.0.113.10",
				},
			},
		},
	}

	ip, err := machineExternalIP(context.Background(), nil, machine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("expected ip %q, got %q", "203.0.113.10", ip)
	}
}

func TestMachineExternalIP_NormalizesCIDR(t *testing.T) {
	t.Parallel()

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
		},
		Status: clusterv1.MachineStatus{
			Addresses: []clusterv1.MachineAddress{
				{
					Type:    clusterv1.MachineExternalIP,
					Address: "136.243.69.167/26",
				},
			},
		},
	}

	ip, err := machineExternalIP(context.Background(), nil, machine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ip != "136.243.69.167" {
		t.Fatalf("expected ip %q, got %q", "136.243.69.167", ip)
	}
}

func TestMachineExternalIP_FromHBMMStatus(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add clusterv1 scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("add infrav1 scheme: %v", err)
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "HetznerBareMetalMachine",
				Name:       "hbmm-1",
			},
		},
	}

	hbmm := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hbmm-1",
			Namespace: "default",
		},
		Status: infrav1.HetznerBareMetalMachineStatus{
			Addresses: []clusterv1.MachineAddress{
				{
					Type:    clusterv1.MachineExternalIP,
					Address: "203.0.113.20",
				},
			},
		},
	}

	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(hbmm).Build()

	ip, err := machineExternalIP(context.Background(), c, machine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ip != "203.0.113.20" {
		t.Fatalf("expected ip %q, got %q", "203.0.113.20", ip)
	}
}

func TestMachineExternalIP_FromAssociatedHostWhenHBMMAddressesEmpty(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add clusterv1 scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("add infrav1 scheme: %v", err)
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "HetznerBareMetalMachine",
				Name:       "hbmm-1",
			},
		},
	}

	hbmm := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hbmm-1",
			Namespace: "default",
			Annotations: map[string]string{
				infrav1.HostAnnotation: "default/hbmh-1",
			},
		},
	}

	hbmh := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hbmh-1",
			Namespace: "default",
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				IPv4: "144.76.101.50/26",
			},
		},
	}

	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(hbmm, hbmh).Build()

	ip, err := machineExternalIP(context.Background(), c, machine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ip != "144.76.101.50" {
		t.Fatalf("expected ip %q, got %q", "144.76.101.50", ip)
	}
}

func TestMachineExternalIP_FromHCloudMachineStatus(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add clusterv1 scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("add infrav1 scheme: %v", err)
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "HCloudMachine",
				Name:       "hcloud-1",
			},
		},
	}

	hm := &infrav1.HCloudMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hcloud-1",
			Namespace: "default",
		},
		Status: infrav1.HCloudMachineStatus{
			Addresses: []clusterv1.MachineAddress{
				{
					Type:    clusterv1.MachineExternalIP,
					Address: "203.0.113.30",
				},
			},
		},
	}

	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(hm).Build()

	ip, err := machineExternalIP(context.Background(), c, machine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ip != "203.0.113.30" {
		t.Fatalf("expected ip %q, got %q", "203.0.113.30", ip)
	}
}

func TestMachineExternalIP_MissingEverywhere(t *testing.T) {
	t.Parallel()

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
		},
	}

	_, err := machineExternalIP(context.Background(), nil, machine)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestMachineExternalIP_FallbackGetError(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add clusterv1 scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("add infrav1 scheme: %v", err)
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "HetznerBareMetalMachine",
				Name:       "hbmm-does-not-exist",
			},
		},
	}

	c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

	_, err := machineExternalIP(context.Background(), c, machine)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "infrastructure fallback failed") {
		t.Fatalf("expected infrastructure fallback error, got %v", err)
	}
}

func TestAssociatedHostFromHBMM(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add clusterv1 scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("add infrav1 scheme: %v", err)
	}

	hbmm := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hbmm-1",
			Namespace: "default",
			Annotations: map[string]string{
				infrav1.HostAnnotation: "default/hbmh-1",
			},
		},
	}

	hbmh := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hbmh-1",
			Namespace: "default",
		},
	}

	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(hbmm, hbmh).Build()

	host, key, err := associatedHostFromHBMM(context.Background(), c, hbmm)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if host.Name != "hbmh-1" || host.Namespace != "default" {
		t.Fatalf("unexpected host: %s/%s", host.Namespace, host.Name)
	}
	if key.Name != "hbmh-1" || key.Namespace != "default" {
		t.Fatalf("unexpected host key: %s/%s", key.Namespace, key.Name)
	}
}

func TestMoveTempFileIfNonEmpty_EmptyFileGetsDropped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tempPath := filepath.Join(dir, "empty.tmp")
	finalPath := filepath.Join(dir, "final.log")
	if err := os.WriteFile(tempPath, []byte(""), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := moveTempFileIfNonEmpty(tempPath, finalPath); err != nil {
		t.Fatalf("move temp file: %v", err)
	}

	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Fatalf("expected final file to not exist, got err=%v", err)
	}
}

func TestMoveTempFileIfNonEmpty_NonEmptyFileGetsMoved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tempPath := filepath.Join(dir, "nonempty.tmp")
	finalPath := filepath.Join(dir, "final.log")
	content := "hello world\n"
	if err := os.WriteFile(tempPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := moveTempFileIfNonEmpty(tempPath, finalPath); err != nil {
		t.Fatalf("move temp file: %v", err)
	}

	b, err := os.ReadFile(finalPath) // #nosec G304 -- test reads file created in t.TempDir
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if string(b) != content {
		t.Fatalf("unexpected final content: %q", string(b))
	}
}
