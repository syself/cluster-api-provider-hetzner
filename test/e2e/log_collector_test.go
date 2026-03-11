package e2e

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hbmm).Build()

	ip, err := machineExternalIP(context.Background(), c, machine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ip != "203.0.113.20" {
		t.Fatalf("expected ip %q, got %q", "203.0.113.20", ip)
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

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hm).Build()

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
