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

package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	hcloudfake "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

func TestHCloudRemediationReconcilerReconcileMissingProviderID(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add cluster-api to scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add infrastructure types to scheme: %v", err)
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "HetznerCluster",
				Name:       "hetzner-cluster",
				Namespace:  "default",
			},
		},
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: cluster.Name,
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "HCloudMachine",
				Name:       "hcloud-machine",
				Namespace:  "default",
			},
		},
	}

	hcloudMachine := &infrav1.HCloudMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hcloud-machine",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       machine.Name,
					UID:        machine.UID,
				},
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
		},
	}

	hetznerCluster := &infrav1.HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hetzner-cluster",
			Namespace: "default",
			UID:       types.UID("hetzner-cluster-uid"),
		},
		Spec: getDefaultHetznerClusterSpec(),
	}

	hetznerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hetznerCluster.Spec.HetznerSecret.Name,
			Namespace: "default",
		},
		Data: map[string][]byte{
			hetznerCluster.Spec.HetznerSecret.Key.HCloudToken: []byte("test-token"),
		},
	}

	remediation := &infrav1.HCloudRemediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "remediation",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       machine.Name,
					UID:        machine.UID,
				},
			},
		},
		Spec: infrav1.HCloudRemediationSpec{
			Strategy: &infrav1.RemediationStrategy{
				Type:       infrav1.RemediationTypeReboot,
				RetryLimit: 1,
				Timeout:    &metav1.Duration{Duration: time.Minute},
			},
		},
	}

	c := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(machine, remediation).
		WithObjects(cluster, machine, hcloudMachine, hetznerCluster, hetznerSecret, remediation).
		Build()

	reconciler := &HCloudRemediationReconciler{
		Client:              c,
		APIReader:           c,
		RateLimitWaitTime:   5 * time.Minute,
		HCloudClientFactory: hcloudfake.NewHCloudClientFactory(),
	}

	ctx := ctrl.LoggerInto(context.Background(), funcr.New(func(string, string) {}, funcr.Options{}))
	result, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(remediation),
	})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result != (reconcile.Result{}) {
		t.Fatalf("expected empty reconcile result, got %+v", result)
	}

	updatedRemediation := &infrav1.HCloudRemediation{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(remediation), updatedRemediation); err != nil {
		t.Fatalf("failed to fetch remediation: %v", err)
	}
	if updatedRemediation.Status.Phase != infrav1.PhaseDeleting {
		t.Fatalf("expected remediation phase %q, got %q", infrav1.PhaseDeleting, updatedRemediation.Status.Phase)
	}

	updatedMachine := &clusterv1.Machine{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(machine), updatedMachine); err != nil {
		t.Fatalf("failed to fetch machine: %v", err)
	}

	condition := conditions.Get(updatedMachine, clusterv1.MachineOwnerRemediatedCondition)
	if condition == nil {
		t.Fatalf("expected %s condition to be set", clusterv1.MachineOwnerRemediatedCondition)
	}
	if condition.Status != corev1.ConditionFalse {
		t.Fatalf("expected condition status %q, got %q", corev1.ConditionFalse, condition.Status)
	}
	if condition.Reason != clusterv1.WaitingForRemediationReason {
		t.Fatalf("expected condition reason %q, got %q", clusterv1.WaitingForRemediationReason, condition.Reason)
	}
}
