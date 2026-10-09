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

package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

func TestIgnoreInsignificantHCloudMachineStatusUpdates(t *testing.T) {
	logger := klog.Background()
	predicate := IgnoreInsignificantHCloudMachineStatusUpdates(logger)

	testCases := []struct {
		name     string
		oldObj   *infrav1.HCloudMachine
		newObj   *infrav1.HCloudMachine
		expected bool
	}{
		{
			name: "No significant changes",
			oldObj: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: infrav1.HCloudMachineStatus{
					Initialization: infrav1.HCloudMachineInitializationStatus{Provisioned: ptr.To(true)},
				},
			},
			newObj: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-machine",
					Namespace:       "default",
					ResourceVersion: "2",
				},
				Status: infrav1.HCloudMachineStatus{
					Initialization: infrav1.HCloudMachineInitializationStatus{Provisioned: ptr.To(true)},
				},
			},
			expected: false,
		},
		{
			name: "Significant changes in spec",
			oldObj: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Spec: infrav1.HCloudMachineSpec{
					Type: "cx11",
				},
			},
			newObj: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Spec: infrav1.HCloudMachineSpec{
					Type: "cx23",
				},
			},
			expected: true,
		},
		{
			name: "Empty status in new object",
			oldObj: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: infrav1.HCloudMachineStatus{
					InstanceState: infrav1.InstanceStateRunning,
				},
			},
			newObj: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: infrav1.HCloudMachineStatus{},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updateEvent := event.UpdateEvent{
				ObjectOld: tc.oldObj,
				ObjectNew: tc.newObj,
			}
			result := predicate.Update(updateEvent)
			if result != tc.expected {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}

func TestIgnoreInsignificantMachineStatusUpdates(t *testing.T) {
	logger := klog.Background()
	predicate := IgnoreInsignificantMachineStatusUpdates(logger)

	testCases := []struct {
		name     string
		oldObj   *clusterv1.Machine
		newObj   *clusterv1.Machine
		expected bool
	}{
		{
			name: "No significant changes",
			oldObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: clusterv1.MachineStatus{},
			},
			newObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-machine",
					Namespace:       "default",
					ResourceVersion: "2",
				},
				Status: clusterv1.MachineStatus{},
			},
			expected: false,
		},
		{
			name: "Significant changes in spec",
			oldObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: "old-cluster",
				},
			},
			newObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: "new-cluster",
				},
			},
			expected: true,
		},
		{
			name: "Changes only in status",
			oldObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: clusterv1.MachineStatus{
					Phase: "Pending",
				},
			},
			newObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: clusterv1.MachineStatus{
					Phase: "Running",
				},
			},
			expected: false,
		},
		{
			name: "Changes in APIServerPodHealthy condition",
			oldObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: clusterv1.MachineStatus{
					Conditions: []metav1.Condition{
						{
							Type:   controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			newObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: clusterv1.MachineStatus{
					Conditions: []metav1.Condition{
						{
							Type:   controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Machine starts deletion (DeletionTimestamp set)",
			oldObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "default",
				},
				Status: clusterv1.MachineStatus{},
			},
			newObj: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-machine",
					Namespace:         "default",
					DeletionTimestamp: ptr.To(metav1.Now()),
					Finalizers:        []string{"test-finalizer"},
				},
				Status: clusterv1.MachineStatus{},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updateEvent := event.UpdateEvent{
				ObjectOld: tc.oldObj,
				ObjectNew: tc.newObj,
			}
			result := predicate.Update(updateEvent)
			if result != tc.expected {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}

func TestIgnoreInsignificantHetznerClusterUpdates_TargetChanges(t *testing.T) {
	logger := klog.Background()
	predicate := IgnoreInsignificantHetznerClusterUpdates(logger)

	oldObj := &infrav1.HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Status: infrav1.HetznerClusterStatus{
			ControlPlaneLoadBalancer: &infrav1.LoadBalancerStatus{
				ID: 1,
				Target: []infrav1.LoadBalancerTarget{
					{
						Type:     infrav1.LoadBalancerTargetTypeServer,
						ServerID: 1,
					},
				},
			},
		},
	}

	newObj := oldObj.DeepCopy()
	newObj.ResourceVersion = "2"
	newObj.Status.ControlPlaneLoadBalancer.Target = []infrav1.LoadBalancerTarget{
		{
			Type:     infrav1.LoadBalancerTargetTypeServer,
			ServerID: 1,
		},
		{
			Type:     infrav1.LoadBalancerTargetTypeServer,
			ServerID: 2,
		},
	}

	updateEvent := event.UpdateEvent{
		ObjectOld: oldObj,
		ObjectNew: newObj,
	}
	if !predicate.Update(updateEvent) {
		t.Errorf("expected control plane load balancer target change to trigger reconcile")
	}

	t.Run("unchanged targets", func(t *testing.T) {
		unchangedObj := oldObj.DeepCopy()
		unchangedObj.ResourceVersion = "2"

		updateEvent := event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: unchangedObj,
		}
		if predicate.Update(updateEvent) {
			t.Errorf("expected unchanged control plane load balancer targets to not trigger reconcile")
		}
	})

	t.Run("nil ControlPlaneLoadBalancer", func(t *testing.T) {
		oldNilLB := &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
		}
		newNilLB := oldNilLB.DeepCopy()
		newNilLB.ResourceVersion = "2"

		updateEvent := event.UpdateEvent{
			ObjectOld: oldNilLB,
			ObjectNew: newNilLB,
		}
		if predicate.Update(updateEvent) {
			t.Errorf("expected nil control plane load balancer on both sides to not trigger reconcile")
		}
	})
}

func TestIgnoreInsignificantSecretUpdates(t *testing.T) {
	p := IgnoreInsignificantSecretUpdates(klog.Background())

	makeSecret := func(data map[string][]byte, rv string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "hetzner",
				Namespace:       "default",
				ResourceVersion: rv,
			},
			Data: data,
		}
	}

	testCases := []struct {
		name     string
		oldObj   *corev1.Secret
		newObj   *corev1.Secret
		expected bool
	}{
		{
			name:     "Data changed",
			oldObj:   makeSecret(map[string][]byte{"hcloud-token": []byte("old")}, "1"),
			newObj:   makeSecret(map[string][]byte{"hcloud-token": []byte("new")}, "2"),
			expected: true,
		},
		{
			name:     "Only ResourceVersion changed",
			oldObj:   makeSecret(map[string][]byte{"hcloud-token": []byte("same")}, "1"),
			newObj:   makeSecret(map[string][]byte{"hcloud-token": []byte("same")}, "2"),
			expected: false,
		},
		{
			name:     "Unrelated data key added",
			oldObj:   makeSecret(map[string][]byte{"hcloud-token": []byte("same")}, "1"),
			newObj:   makeSecret(map[string][]byte{"hcloud-token": []byte("same"), "other": []byte("x")}, "2"),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := p.Update(event.UpdateEvent{ObjectOld: tc.oldObj, ObjectNew: tc.newObj})
			require.Equal(t, tc.expected, got)
		})
	}

	require.True(t, p.Create(event.CreateEvent{Object: makeSecret(nil, "1")}))
	require.True(t, p.Delete(event.DeleteEvent{Object: makeSecret(nil, "1")}))
	require.False(t, p.Generic(event.GenericEvent{Object: makeSecret(nil, "1")}))
}

func TestHetznerSecretToHCloudMachines(t *testing.T) {
	ctx := context.Background()

	testScheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(testScheme))
	utilruntime.Must(infrav1.AddToScheme(testScheme))
	utilruntime.Must(clusterv1.AddToScheme(testScheme))

	const (
		ns          = "default"
		secretName  = "hetzner"
		clusterName = "cluster-a"
	)

	newCluster := func(name string) *clusterv1.Cluster {
		return &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, UID: types.UID(name + "-uid")},
		}
	}
	newHetznerCluster := func(name, clusterOwner, secret string) *infrav1.HetznerCluster {
		return &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: clusterv1.GroupVersion.String(), Kind: "Cluster", Name: clusterOwner, UID: types.UID(clusterOwner + "-uid")},
				},
			},
			Spec: infrav1.HetznerClusterSpec{
				HetznerSecret: infrav1.HetznerSecretRef{
					Name: secret,
					Key:  infrav1.HetznerSecretKeyRef{HCloudToken: "hcloud-token"},
				},
			},
		}
	}
	newMachine := func(name, clusterOwner, infraName string) *clusterv1.Machine {
		return &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterOwner},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: clusterOwner,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "HCloudMachine",
					Name:     infraName,
				},
			},
		}
	}

	capiClusterA := newCluster(clusterName)
	capiClusterB := newCluster("cluster-b")
	hcA := newHetznerCluster("hc-a", clusterName, secretName)
	hcB := newHetznerCluster("hc-b", "cluster-b", secretName)
	hcUnrelated := newHetznerCluster("hc-u", clusterName, "other-secret")
	hcmA := &infrav1.HCloudMachine{ObjectMeta: metav1.ObjectMeta{Name: "m-a", Namespace: ns}}
	hcmB := &infrav1.HCloudMachine{ObjectMeta: metav1.ObjectMeta{Name: "m-b", Namespace: ns}}
	cmA := newMachine("cm-a", clusterName, hcmA.Name)
	cmB := newMachine("cm-b", "cluster-b", hcmB.Name)
	matchingSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns}}
	otherSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "no-ref", Namespace: ns}}

	c := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(capiClusterA, capiClusterB, hcA, hcB, hcUnrelated, hcmA, hcmB, cmA, cmB).
		Build()

	r := &HCloudMachineReconciler{Client: c}
	mapper := r.HetznerSecretToHCloudMachines(ctx)

	got := mapper(ctx, matchingSecret)
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: client.ObjectKey{Namespace: ns, Name: hcmA.Name}},
		{NamespacedName: client.ObjectKey{Namespace: ns, Name: hcmB.Name}},
	}, got)

	require.Empty(t, mapper(ctx, otherSecret))
}

func TestHetznerSecretToHCloudMachinesRespectsWatchFilter(t *testing.T) {
	ctx := context.Background()

	testScheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(testScheme))
	utilruntime.Must(infrav1.AddToScheme(testScheme))
	utilruntime.Must(clusterv1.AddToScheme(testScheme))

	const (
		ns         = "default"
		secretName = "hetzner"
		filterFoo  = "foo"
		filterBar  = "bar"
	)

	// newObjects returns a CAPI Cluster, a HetznerCluster with the given watch filter label,
	// and a CAPI Machine whose infrastructureRef points to an HCloudMachine. All names
	// start with prefix.
	newObjects := func(prefix, watchFilterLabel string) []client.Object {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: prefix, Namespace: ns, UID: types.UID(prefix + "-uid")},
		}
		hetznerCluster := &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      prefix,
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: clusterv1.GroupVersion.String(), Kind: "Cluster", Name: cluster.Name, UID: cluster.UID},
				},
			},
			Spec: infrav1.HetznerClusterSpec{
				HetznerSecret: infrav1.HetznerSecretRef{Name: secretName},
			},
		}
		if watchFilterLabel != "" {
			hetznerCluster.Labels = map[string]string{clusterv1.WatchLabel: watchFilterLabel}
		}
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      prefix,
				Namespace: ns,
				Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: cluster.Name,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "HCloudMachine",
					Name:     prefix + "-hcloudmachine",
				},
			},
		}
		return []client.Object{cluster, hetznerCluster, machine}
	}

	objects := newObjects("owned-by-foo", filterFoo)
	objects = append(objects, newObjects("owned-by-bar", filterBar)...)
	objects = append(objects, newObjects("unlabelled", "")...)

	c := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(objects...).
		Build()

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns}}

	fooReconciler := &HCloudMachineReconciler{Client: c, WatchFilterValue: filterFoo}
	got := fooReconciler.HetznerSecretToHCloudMachines(ctx)(ctx, secret)
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: client.ObjectKey{Namespace: ns, Name: "owned-by-foo-hcloudmachine"}},
	}, got, "with --watch-filter=foo, only HCloudMachines of HetznerClusters labelled foo are enqueued")

	emptyFilterReconciler := &HCloudMachineReconciler{Client: c}
	got = emptyFilterReconciler.HetznerSecretToHCloudMachines(ctx)(ctx, secret)
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: client.ObjectKey{Namespace: ns, Name: "owned-by-foo-hcloudmachine"}},
		{NamespacedName: client.ObjectKey{Namespace: ns, Name: "owned-by-bar-hcloudmachine"}},
		{NamespacedName: client.ObjectKey{Namespace: ns, Name: "unlabelled-hcloudmachine"}},
	}, got, "without --watch-filter, HCloudMachines of every HetznerCluster that uses the Secret are enqueued")
}

var _ = Describe("HCloudMachineReconciler", func() {
	var (
		capiCluster *clusterv1.Cluster
		capiMachine *clusterv1.Machine

		hetznerCluster *infrav1.HetznerCluster
		hcloudMachine  *infrav1.HCloudMachine

		testNs *corev1.Namespace

		hetznerSecret   *corev1.Secret
		bootstrapSecret *corev1.Secret

		key client.ObjectKey

		hcloudMachineName string

		hcloudClient hcloudclient.Client
	)

	BeforeEach(func() {
		var err error
		var finish func()
		testNs, finish, err = testEnv.ResetAndCreateNamespace(ctx, "hcloudmachine-reconciler")
		defer finish()
		Expect(err).NotTo(HaveOccurred())
		hcloudClient = testEnv.HCloudClientFactory.NewClient("fake-token")

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "HetznerCluster",
					Name:     "hetzner-test1",
				},
			},
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		hcloudMachineName = utils.GenerateName(nil, "hcloud-machine-")
		capiMachineName := utils.GenerateName(nil, "capi-machine-")

		capiMachine = &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       capiMachineName,
				Namespace:  testNs.Name,
				Finalizers: []string{clusterv1.MachineFinalizer},
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: capiCluster.Name,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "HCloudMachine",
					Name:     hcloudMachineName,
				},
				FailureDomain: defaultFailureDomain,
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}

		hetznerCluster = &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hetzner-test1",
				Namespace: testNs.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       capiCluster.Name,
						UID:        capiCluster.UID,
					},
				},
			},
			Spec: getDefaultHetznerClusterSpec(),
		}

		hetznerSecret = getDefaultHetznerSecret(testNs.Name)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		bootstrapSecret = getDefaultBootstrapSecret(testNs.Name)
		Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())

		key = client.ObjectKey{Namespace: testNs.Name, Name: hcloudMachineName}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, hetznerSecret, bootstrapSecret)).To(Succeed())
	})

	Context("Basic hcloudmachine test", func() {
		Context("correct server", func() {
			BeforeEach(func() {
				// clear DataSecretName to simulate "bootstrap not ready"
				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					ConfigRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "bootstrap.cluster.x-k8s.io",
						Kind:     "KubeadmConfig",
						Name:     "my-config",
					},
				}
				Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

				hcloudMachine = &infrav1.HCloudMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      hcloudMachineName,
						Namespace: testNs.Name,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:             capiCluster.Name,
							clusterv1.MachineControlPlaneNameLabel: "",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: clusterv1.GroupVersion.String(),
								Kind:       "Machine",
								Name:       capiMachine.Name,
								UID:        capiMachine.UID,
							},
						},
					},
					Spec: infrav1.HCloudMachineSpec{
						ImageName:          "my-control-plane",
						Type:               "cpx32",
						PlacementGroupName: &defaultPlacementGroupName,
					},
				}
				Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
				Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, capiMachine, hcloudMachine, hetznerCluster)).To(Succeed())
			})

			It("creates the infra machine", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, hcloudMachine); err != nil {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})

			It("creates the HCloud machine in Hetzner 1", func() {
				By("checking that no servers exist")

				Eventually(func() bool {
					servers, err := testEnv.HCloudClientFactory.NewClient("fake-token").ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{hetznerCluster.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return false
					}
					if len(servers) != 0 {
						return false
					}
					return true
				}, timeout, interval).Should(BeTrue())

				By("checking that bootstrap condition is not ready")

				Eventually(func() error {
					err := testEnv.Get(ctx, client.ObjectKeyFromObject(hcloudMachine), hcloudMachine)
					if err != nil {
						return err
					}
					c := deprecatedv1beta1conditions.Get(hcloudMachine, infrav1.BootstrapReadyV1Beta1Condition)
					if c == nil {
						return fmt.Errorf("BootstrapReadyCondition not set")
					}
					if c.Status != corev1.ConditionFalse {
						return fmt.Errorf("BootstrapReadyCondition not false")
					}
					if c.Reason != infrav1.BootstrapNotReadyV1Beta1Reason {
						return fmt.Errorf("BootstrapNotReadyReason not set. Reason: %q", c.Reason)
					}
					if !isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudMachineServerCreatedCondition, infrav1.HCloudMachineServerWaitingForBootstrapDataReason) {
						return fmt.Errorf("ServerCreated condition not false with WaitingForBootstrapData reason")
					}
					return nil
				}, timeout, interval).Should(Succeed())

				By("setting the bootstrap data")

				ph, err := patch.NewHelper(capiMachine, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				}

				Eventually(func() error {
					return ph.Patch(ctx, capiMachine, patch.WithStatusObservedGeneration{})
				}, timeout, interval).Should(BeNil())

				By("checking that bootstrap condition is ready")

				Eventually(func() bool {
					return isPresentAndTrueDeprecatedV1Beta1(key, hcloudMachine, infrav1.BootstrapReadyV1Beta1Condition)
				}, timeout, interval).Should(BeTrue())

				By("listing hcloud servers")

				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{hetznerCluster.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return 0
					}
					return len(servers)
				}, timeout, interval).Should(BeNumerically(">", 0))

				By("checking if server created condition is set")

				Eventually(func() bool {
					return isPresentAndTrueDeprecatedV1Beta1(key, hcloudMachine, infrav1.ServerCreateSucceededV1Beta1Condition) &&
						isPresentAndTrueWithReason(key, hcloudMachine, infrav1.HCloudMachineServerCreatedCondition, infrav1.HCloudMachineServerCreatedReason)
				}, timeout, interval).Should(BeTrue())

				By("checking if server provisioned condition is set")

				Eventually(func() bool {
					return isPresentAndTrueDeprecatedV1Beta1(key, hcloudMachine, infrav1.ServerProvisionedV1Beta1Condition) &&
						isPresentAndTrueWithReason(key, hcloudMachine, infrav1.HCloudMachineServerProvisionedCondition, infrav1.HCloudMachineServerProvisionedReason)
				}, timeout, interval).Should(BeTrue())

				By("checking if server available condition is set")

				Eventually(func() bool {
					return isPresentAndTrueDeprecatedV1Beta1(key, hcloudMachine, infrav1.ServerAvailableV1Beta1Condition) &&
						isPresentAndTrueWithReason(key, hcloudMachine, infrav1.HCloudMachineServerAvailableCondition, infrav1.HCloudMachineServerAvailableReason)
				}, timeout, interval).Should(BeTrue())

				By("checking if the v1beta2 summary condition is set")
				Eventually(func(g Gomega) {
					g.Expect(testEnv.Get(ctx, key, hcloudMachine)).To(Succeed())
					g.Expect(conditions.IsTrue(hcloudMachine, clusterv1.ReadyCondition)).To(BeTrue())
					g.Expect(conditions.Get(hcloudMachine, clusterv1.ReadyCondition).Reason).To(Equal(clusterv1.ReadyReason))
				}, timeout, interval).Should(Succeed())

				By("checking if the BootState is now OperatingSystemRunning")
				Eventually(func() bool {
					if err = testEnv.Get(ctx, key, hcloudMachine); err != nil {
						return false
					}

					return hcloudMachine.Status.BootState == infrav1.HCloudBootStateOperatingSystemRunning && !hcloudMachine.Status.BootStateSince.IsZero()
				}, timeout, interval).Should(BeTrue())

				By("checking if the ssh keys are set in the status")
				Eventually(func() bool {
					if err = testEnv.Get(ctx, key, hcloudMachine); err != nil {
						return false
					}

					return len(hcloudMachine.Status.SSHKeys) == 1 && hcloudMachine.Status.SSHKeys[0].Name == "testsshkey"
				}, timeout, interval).Should(BeTrue())
			})
		})

		Context("wrong server", func() {
			BeforeEach(func() {
				Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

				hcloudMachine = &infrav1.HCloudMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      hcloudMachineName,
						Namespace: testNs.Name,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:             capiCluster.Name,
							clusterv1.MachineControlPlaneNameLabel: "",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: clusterv1.GroupVersion.String(),
								Kind:       "Machine",
								Name:       capiMachine.Name,
								UID:        capiMachine.UID,
							},
						},
					},
					Spec: infrav1.HCloudMachineSpec{
						ImageName:          "my-control-plane-2",
						Type:               "cpx32",
						PlacementGroupName: &defaultPlacementGroupName,
					},
				}
				Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())

				Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, hcloudMachine, hetznerCluster)).To(Succeed())
			})

			It("checks that ImageNotFound is visible in conditions if image does not exist", func() {
				Eventually(func() bool {
					return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, hcloudMachine, infrav1.ServerCreateSucceededV1Beta1Condition, infrav1.ImageNotFoundV1Beta1Reason) &&
						isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudMachineServerCreatedCondition, infrav1.HCloudMachineServerImageNotFoundReason)
				}, timeout, interval).Should(BeTrue())
			})
		})
	})

	Context("various specs", func() {
		BeforeEach(func() {
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			hcloudMachine = &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hcloudMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel:             capiCluster.Name,
						clusterv1.MachineControlPlaneNameLabel: "",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       capiMachine.Name,
							UID:        capiMachine.UID,
						},
					},
				},
				Spec: infrav1.HCloudMachineSpec{
					ImageName:          "my-control-plane",
					Type:               "cpx32",
					PlacementGroupName: &defaultPlacementGroupName,
				},
			}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, capiMachine)).To(Succeed())
		})

		Context("without network", func() {
			BeforeEach(func() {
				hetznerCluster.Spec.HCloudNetwork.Enabled = false
				Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
				Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, hetznerCluster, hcloudMachine)).To(Succeed())
			})

			It("creates the HCloud machine in Hetzner 2", func() {
				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{hetznerCluster.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return 0
					}
					return len(servers)
				}, timeout, interval).Should(BeNumerically(">", 0))
			})
		})

		Context("without placement groups", func() {
			BeforeEach(func() {
				hetznerCluster.Spec.HCloudPlacementGroups = nil
				Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

				hcloudMachine.Spec.PlacementGroupName = nil
				Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, hetznerCluster, hcloudMachine)).To(Succeed())
			})

			It("creates the HCloud machine in Hetzner 3", func() {
				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{hetznerCluster.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return 0
					}

					return len(servers)
				}, timeout, interval).Should(BeNumerically(">", 0))
			})
		})

		Context("without placement groups, but with placement group in hcloudMachine spec", func() {
			BeforeEach(func() {
				hetznerCluster.Spec.HCloudPlacementGroups = nil
				Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
				Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, hetznerCluster, hcloudMachine)).To(Succeed())
			})

			It("should show the expected reason for server not created", func() {
				Eventually(func() bool {
					return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, hcloudMachine, infrav1.ServerCreateSucceededV1Beta1Condition, infrav1.InstanceHasNonExistingPlacementGroupV1Beta1Reason) &&
						isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudMachineServerCreatedCondition, infrav1.HCloudMachineServerPlacementGroupNotFoundReason)
				}, timeout).Should(BeTrue())
			})
		})

		Context("with public network specs", func() {
			BeforeEach(func() {
				hcloudMachine.Spec.PublicNetwork = &infrav1.PublicNetworkSpec{
					EnableIPv4: false,
					EnableIPv6: false,
				}
				Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
				Eventually(func() bool {
					var updatedCluster infrav1.HetznerCluster
					if err := testEnv.Get(ctx, client.ObjectKeyFromObject(hetznerCluster), &updatedCluster); err != nil {
						return false
					}

					if updatedCluster.Spec.ControlPlaneEndpoint.Host == "" {
						return false
					}
					if updatedCluster.Status.ControlPlaneLoadBalancer == nil {
						return false
					}
					if updatedCluster.Status.ControlPlaneLoadBalancer.IPv4 == "" {
						return false
					}

					return updatedCluster.Spec.ControlPlaneEndpoint.Host == updatedCluster.Status.ControlPlaneLoadBalancer.IPv4
				}, timeout, interval).Should(BeTrue())
				Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, hetznerCluster, hcloudMachine)).To(Succeed())
			})

			It("creates the HCloud machine in Hetzner 4", func() {
				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{hetznerCluster.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return 0
					}

					return len(servers)
				}, timeout, interval).Should(BeNumerically(">", 0))
			})
		})
	})
})

var _ = Describe("Hetzner secret", func() {
	var (
		testNs *corev1.Namespace

		hetznerCluster *infrav1.HetznerCluster
		hcloudMachine  *infrav1.HCloudMachine

		capiCluster   *clusterv1.Cluster
		capiMachine   *clusterv1.Machine
		hetznerSecret *corev1.Secret

		key client.ObjectKey

		hetznerClusterName string
	)

	BeforeEach(func() {
		var err error
		var finish func()
		testNs, finish, err = testEnv.ResetAndCreateNamespace(ctx, "hcloudmachine-validation")
		defer finish()
		Expect(err).NotTo(HaveOccurred())

		hetznerClusterName = utils.GenerateName(nil, "hetzner-cluster-test")

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "HetznerCluster",
					Name:     hetznerClusterName,
				},
			},
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		hetznerCluster = &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hetznerClusterName,
				Namespace: testNs.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       capiCluster.Name,
						UID:        capiCluster.UID,
					},
				},
			},
			Spec: getDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

		hcloudMachineName := utils.GenerateName(nil, "hcloud-machine-")

		capiMachine = &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "capi-machine-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.MachineFinalizer},
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: capiCluster.Name,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "HCloudMachine",
					Name:     hcloudMachineName,
				},
				FailureDomain: defaultFailureDomain,
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hcloudMachineName,
				Namespace: testNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel:             capiCluster.Name,
					clusterv1.MachineControlPlaneNameLabel: "",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       capiMachine.Name,
						UID:        capiMachine.UID,
					},
				},
			},
			Spec: infrav1.HCloudMachineSpec{
				ImageName:          "my-control-plane",
				Type:               "cpx32",
				PlacementGroupName: &defaultPlacementGroupName,
			},
		}
		Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
		key = client.ObjectKey{Namespace: testNs.Name, Name: hcloudMachine.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, hetznerCluster, capiCluster, hcloudMachine, capiMachine, hetznerSecret)).To(Succeed())
	})

	DescribeTable("test different hetzner secret",
		func(secretFunc func() *corev1.Secret, expectedDeprecatedV1Beta1Reason string) {
			hetznerSecret = secretFunc()
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			expectedReason := infrav1.HCloudTokenInvalidReason
			if expectedDeprecatedV1Beta1Reason == infrav1.HetznerSecretUnreachableV1Beta1Reason {
				expectedReason = infrav1.HCloudTokenSecretUnreachableReason
			}

			Eventually(func() bool {
				return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, hcloudMachine, infrav1.HCloudTokenAvailableV1Beta1Condition, expectedDeprecatedV1Beta1Reason)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudTokenAvailableCondition, expectedReason)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, hcloudMachine, clusterv1.ReadyCondition, clusterv1.NotReadyReason)
			}, timeout, interval).Should(BeTrue())
			Expect(testEnv.Cleanup(ctx, hetznerSecret)).To(Succeed())
		},
		Entry("no Hetzner secret/wrong reference", func() *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wrong-name",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"hcloud": []byte("my-token"),
				},
			}
		}, infrav1.HetznerSecretUnreachableV1Beta1Reason),
		Entry("empty hcloud token", func() *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hetzner-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"hcloud": []byte(""),
				},
			}
		}, infrav1.HCloudCredentialsInvalidV1Beta1Reason),
		Entry("wrong key in secret", func() *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hetzner-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"wrongkey": []byte("my-token"),
				},
			}
		}, infrav1.HCloudCredentialsInvalidV1Beta1Reason),
	)

	It("finishes deleting an HCloudMachine whose owner Machine is already gone", func() {
		hetznerSecret = getDefaultHetznerSecret(testNs.Name)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		// Keep the HCloudMachine around after it is deleted, the way the controller does.
		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, key, hcloudMachine)).To(Succeed())
			hcloudMachine.Finalizers = append(hcloudMachine.Finalizers, infrav1.HCloudMachineFinalizer)
			g.Expect(testEnv.Update(ctx, hcloudMachine)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Force-delete the owner Machine. This is what leaves the HCloudMachine orphaned:
		// CAPI on its own waits for the infrastructure to go first.
		capiMachineKey := client.ObjectKeyFromObject(capiMachine)
		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, capiMachineKey, capiMachine)).To(Succeed())
			capiMachine.Finalizers = nil
			g.Expect(testEnv.Update(ctx, capiMachine)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(client.IgnoreNotFound(testEnv.Delete(ctx, capiMachine))).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(testEnv.Get(ctx, capiMachineKey, capiMachine))
		}, timeout, interval).Should(BeTrue())

		// Deleting the HCloudMachine now has no owner Machine to look up.
		Expect(testEnv.Delete(ctx, hcloudMachine)).To(Succeed())

		// The controller has to release the finalizer anyway, otherwise the object and
		// the server behind it stay around forever.
		Eventually(func() bool {
			return apierrors.IsNotFound(testEnv.Get(ctx, key, hcloudMachine))
		}, timeout, interval).Should(BeTrue())
	})

	It("sets InstanceState=Deleting and ServerAvailable=False on delete with missing secret", func() {
		// Create a wrong secret so token validation fails.
		hetznerSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wrong-name",
				Namespace: testNs.Name,
			},
			Data: map[string][]byte{
				"hcloud": []byte("my-token"),
			},
		}
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		// Add a finalizer so the object is not immediately removed on delete.
		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, key, hcloudMachine)).To(Succeed())
			hcloudMachine.Finalizers = append(hcloudMachine.Finalizers, infrav1.HCloudMachineFinalizer)
			g.Expect(testEnv.Update(ctx, hcloudMachine)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Delete the machine (sets DeletionTimestamp but keeps it because of the finalizer).
		Expect(testEnv.Delete(ctx, hcloudMachine)).To(Succeed())

		// InstanceState should be set to Deleting even though the secret is missing.
		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, key, hcloudMachine)).To(Succeed())
			g.Expect(hcloudMachine.Status.InstanceState).ToNot(BeEmpty())
			g.Expect(hcloudMachine.Status.InstanceState).To(Equal(infrav1.InstanceStateDeleting))
		}, timeout, interval).Should(Succeed())

		// The ServerAvailable condition should be False with Deleting reason.
		Eventually(func() bool {
			return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudMachineServerAvailableCondition, infrav1.HCloudMachineDeletingReason)
		}, timeout, interval).Should(BeTrue())

		// Token condition should also be False (secret is missing).
		Eventually(func() bool {
			return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, hcloudMachine, infrav1.HCloudTokenAvailableV1Beta1Condition, infrav1.HetznerSecretUnreachableV1Beta1Reason) &&
				isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudTokenSecretUnreachableReason)
		}, timeout, interval).Should(BeTrue())
	})
})

var _ = Describe("Hetzner secret watch with a non-empty watch filter", func() {
	It("reconciles the HCloudMachine when a Secret without the watch filter label is created or updated", func() {
		const watchFilterValue = "hcloudmachine-secret-watch-filter-test"

		// Point the HCloudMachineReconciler of the suite to an unused namespace. It has no
		// watch filter, so it would reconcile the objects of this test. Only the manager
		// configured below should do that.
		_, finish, err := testEnv.ResetAndCreateNamespace(ctx, "watch-filter-unused")
		defer finish()
		Expect(err).NotTo(HaveOccurred())
		testNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "watch-filter-secret-"}}
		Expect(testEnv.Create(ctx, testNs)).To(Succeed())

		// The suite sets secretErrorRetryDelay to 1ms. A requeue would reconcile the
		// HCloudMachine again, and the test would pass even without the Secret event.
		// One hour is much longer than the test, so only the Secret event can do it.
		originalSecretErrorRetryDelay := secretErrorRetryDelay
		secretErrorRetryDelay = time.Hour
		DeferCleanup(func() { secretErrorRetryDelay = originalSecretErrorRetryDelay })

		mgr, err := ctrl.NewManager(testEnv.Config, ctrl.Options{
			Scheme: testEnv.GetScheme(),
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{testNs.Name: {}},
				ByObject:          secretutil.AddSecretSelector(),
			},
			Metrics: metricsserver.Options{BindAddress: "0"},
		})
		Expect(err).NotTo(HaveOccurred())
		filteredReconciler := &HCloudMachineReconciler{
			Client:              mgr.GetClient(),
			APIReader:           mgr.GetAPIReader(),
			WatchFilterValue:    watchFilterValue,
			Namespace:           testNs.Name,
			HCloudClientFactory: testEnv.HCloudClientFactory,
		}
		Expect(filteredReconciler.SetupWithManager(ctx, mgr, controller.Options{
			SkipNameValidation: ptr.To(true),
			// Same reason as secretErrorRetryDelay above, for reconciles that return an error.
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](time.Hour, time.Hour),
		})).To(Succeed())

		hetznerSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hetzner-secret",
				Namespace: testNs.Name,
			},
		}

		hetznerClusterName := utils.GenerateName(nil, "hetzner-cluster-test")
		capiCluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "HetznerCluster",
					Name:     hetznerClusterName,
				},
			},
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		hetznerCluster := &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hetznerClusterName,
				Namespace: testNs.Name,
				Labels:    map[string]string{clusterv1.WatchLabel: watchFilterValue},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       capiCluster.Name,
						UID:        capiCluster.UID,
					},
				},
			},
			Spec: getDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

		hcloudMachineName := utils.GenerateName(nil, "hcloud-machine-")
		capiMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "capi-machine-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.MachineFinalizer},
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: capiCluster.Name,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "HCloudMachine",
					Name:     hcloudMachineName,
				},
				FailureDomain: defaultFailureDomain,
				// No DataSecretName means bootstrap data is not ready. A reconcile
				// with a valid token then stops at the ServerCreated condition, which
				// the test checks below.
				Bootstrap: clusterv1.Bootstrap{
					ConfigRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "bootstrap.cluster.x-k8s.io",
						Kind:     "KubeadmConfig",
						Name:     "my-config",
					},
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		hcloudMachine := &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hcloudMachineName,
				Namespace: testNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
					clusterv1.WatchLabel:       watchFilterValue,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       capiMachine.Name,
						UID:        capiMachine.UID,
					},
				},
			},
			Spec: infrav1.HCloudMachineSpec{
				ImageName:          "my-control-plane",
				Type:               "cpx32",
				PlacementGroupName: &defaultPlacementGroupName,
			},
		}
		Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
		DeferCleanup(func() {
			Expect(testEnv.Cleanup(ctx, hetznerCluster, capiCluster, hcloudMachine, capiMachine, hetznerSecret)).To(Succeed())
		})

		managerCtx, cancelManager := context.WithCancel(ctx)
		managerDone := make(chan error, 1)
		go func() { managerDone <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			cancelManager()
			Eventually(managerDone, timeout).Should(Receive(Succeed()))
		})
		key := client.ObjectKeyFromObject(hcloudMachine)

		By("waiting for the HCloudMachine to report the missing Secret")
		Eventually(func() bool {
			return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudTokenSecretUnreachableReason)
		}, timeout, interval).Should(BeTrue())

		By("creating the Secret without the watch filter label")
		// The manager cache only holds Secrets with the label secretutil.LabelEnvironmentName.
		// AcquireSecret adds it during a reconcile with a valid token. That reconcile does not
		// happen here, so we add the label ourselves.
		hetznerSecret.Labels = map[string]string{secretutil.LabelEnvironmentName: secretutil.LabelEnvironmentValue}
		hetznerSecret.Data = map[string][]byte{"hcloud": []byte("")}
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		By("expecting the Secret create event to reconcile the HCloudMachine")
		Eventually(func() bool {
			return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudTokenInvalidReason)
		}, timeout, interval).Should(BeTrue())

		By("updating the token in the Secret")
		Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(hetznerSecret), hetznerSecret)).To(Succeed())
		hetznerSecret.Data["hcloud"] = []byte("rotated-token")
		Expect(testEnv.Update(ctx, hetznerSecret)).To(Succeed())

		By("expecting the Secret update event to reconcile the HCloudMachine with the new token")
		Eventually(func() bool {
			return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudMachineServerCreatedCondition, infrav1.HCloudMachineServerWaitingForBootstrapDataReason)
		}, timeout, interval).Should(BeTrue())
	})
})

var _ = Describe("HCloudMachine validation", func() {
	var (
		hcloudMachine *infrav1.HCloudMachine
		testNs        *corev1.Namespace
	)

	BeforeEach(func() {
		var err error
		var finish func()
		testNs, finish, err = testEnv.ResetAndCreateNamespace(ctx, "hcloudmachine-validation")
		defer finish()
		Expect(err).NotTo(HaveOccurred())

		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloud-validation-machine",
				Namespace: testNs.Name,
			},
			Spec: infrav1.HCloudMachineSpec{
				ImageName: "my-control-plane",
				Type:      "cpx32",
			},
		}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, hcloudMachine)).To(Succeed())
	})

	It("should fail without imageName", func() {
		hcloudMachine.Spec.ImageName = ""
		Expect(testEnv.Create(ctx, hcloudMachine)).ToNot(Succeed())
	})

	It("should allow valid HCloudMachine creation", func() {
		Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
	})

	It("should prevent updating immutable fields", func() {
		Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())

		Eventually(func() error {
			key := client.ObjectKey{Namespace: testNs.Name, Name: hcloudMachine.Name}
			return testEnv.Client.Get(ctx, key, hcloudMachine)
		}, timeout, interval).Should(BeNil())

		hcloudMachine.Spec.Type = "cpx42"
		hcloudMachine.Spec.ImageName = "my-control-plane"
		Expect(testEnv.Update(ctx, hcloudMachine)).ToNot(Succeed())
	})
})

var _ = Describe("IgnoreInsignificantHetznerClusterUpdates Predicate", func() {
	var (
		predicate predicate.Predicate

		oldCluster *infrav1.HetznerCluster
		newCluster *infrav1.HetznerCluster
	)

	BeforeEach(func() {
		predicate = IgnoreInsignificantHetznerClusterUpdates(klog.Background())

		oldCluster = &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-predicate", ResourceVersion: "1"},
			Spec:       getDefaultHetznerClusterSpec(),
			Status: infrav1.HetznerClusterStatus{
				Conditions: []metav1.Condition{},
			},
		}
		deprecatedv1beta1conditions.MarkTrue(oldCluster, infrav1.CredentialsAvailableV1Beta1Condition)

		newCluster = oldCluster.DeepCopy()
	})

	It("should skip updates to the HetznerCluster conditions", func() {
		// Make change to conditions & other fields that get changed on every update
		deprecatedv1beta1conditions.MarkFalse(newCluster, infrav1.CredentialsAvailableV1Beta1Condition, infrav1.HCloudCredentialsInvalidV1Beta1Reason, clusterv1.ConditionSeverityError, "")
		newCluster.ResourceVersion = "2"
		newCluster.SetManagedFields([]metav1.ManagedFieldsEntry{{
			Manager:   "test",
			Operation: "update",
		}})

		Expect(predicate.Update(event.UpdateEvent{
			ObjectOld: oldCluster,
			ObjectNew: newCluster,
		})).To(BeFalse())
	})

	It("should skip updates to the v1beta2 HetznerCluster conditions", func() {
		conditions.Set(newCluster, metav1.Condition{
			Type:   infrav1.HCloudTokenAvailableCondition,
			Status: metav1.ConditionFalse,
			Reason: infrav1.HCloudTokenInvalidReason,
		})

		Expect(predicate.Update(event.UpdateEvent{
			ObjectOld: oldCluster,
			ObjectNew: newCluster,
		})).To(BeFalse())
	})

	It("should process updates to other fields", func() {
		newCluster.Spec.ControlPlaneRegions = []infrav1.Region{"fsn1", "nbg1", "hel1"}

		Expect(predicate.Update(event.UpdateEvent{
			ObjectOld: oldCluster,
			ObjectNew: newCluster,
		})).To(BeTrue())
	})

	It("should process updates to other resources", func() {
		Expect(predicate.Update(event.UpdateEvent{
			ObjectOld: &infrav1.HCloudMachine{},
			ObjectNew: &infrav1.HCloudMachine{},
		})).To(BeTrue())
	})

	It("should process create events", func() {
		Expect(predicate.Create(event.CreateEvent{
			Object: newCluster,
		})).To(BeTrue())
	})

	It("should process delete events", func() {
		Expect(predicate.Delete(event.DeleteEvent{
			Object: newCluster,
		})).To(BeTrue())
	})

	It("should process generic events", func() {
		Expect(predicate.Generic(event.GenericEvent{
			Object: newCluster,
		})).To(BeTrue())
	})
})
