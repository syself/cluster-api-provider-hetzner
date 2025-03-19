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
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
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
					Ready: true,
				},
			},
			newObj: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-machine",
					Namespace:       "default",
					ResourceVersion: "2",
				},
				Status: infrav1.HCloudMachineStatus{
					Ready: true,
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
					Type: "cx21",
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
					InstanceState: stPtr(hcloud.ServerStatusRunning),
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

func stPtr(h hcloud.ServerStatus) *hcloud.ServerStatus {
	return &h
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
	)

	BeforeEach(func() {
		hcloudClient.Reset()

		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "hcloudmachine-reconciler")
		Expect(err).NotTo(HaveOccurred())

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "HetznerCluster",
					Name:       "hetzner-test1",
					Namespace:  testNs.Name,
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
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "HCloudMachine",
					Name:       hcloudMachineName,
				},
				FailureDomain: &defaultFailureDomain,
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
						APIVersion: "cluster.x-k8s.io/v1beta1",
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

	Context("Basic test", func() {
		Context("correct server", func() {
			BeforeEach(func() {
				// remove bootstrap infos
				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{}
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
						Type:               "cpx31",
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
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
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

				Eventually(func() bool {
					return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.BootstrapReadyCondition, infrav1.BootstrapNotReadyReason)
				}, timeout, interval).Should(BeTrue())

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
					return isPresentAndTrue(key, hcloudMachine, infrav1.BootstrapReadyCondition)
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
					return isPresentAndTrue(key, hcloudMachine, infrav1.ServerCreateSucceededCondition)
				}, timeout, interval).Should(BeTrue())

				By("checking if server available condition is set")

				Eventually(func() bool {
					return isPresentAndTrue(key, hcloudMachine, infrav1.ServerAvailableCondition)
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
						Type:               "cpx31",
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
					return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.ServerCreateSucceededCondition, infrav1.ImageNotFoundReason)
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
					Type:               "cpx31",
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
					return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.ServerCreateSucceededCondition, infrav1.InstanceHasNonExistingPlacementGroupReason)
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
		hcloudClient.Reset()
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "hcloudmachine-validation")
		Expect(err).NotTo(HaveOccurred())

		hetznerClusterName = utils.GenerateName(nil, "hetzner-cluster-test")

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "HetznerCluster",
					Name:       hetznerClusterName,
					Namespace:  testNs.Name,
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
						APIVersion: "cluster.x-k8s.io/v1beta1",
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
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "HCloudMachine",
					Name:       hcloudMachineName,
				},
				FailureDomain: &defaultFailureDomain,
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
				Type:               "cpx31",
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
		func(secretFunc func() *corev1.Secret, expectedReason string) {
			hetznerSecret = secretFunc()
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.HCloudTokenAvailableCondition, expectedReason)
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
		}, infrav1.HetznerSecretUnreachableReason),
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
		}, infrav1.HCloudCredentialsInvalidReason),
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
		}, infrav1.HCloudCredentialsInvalidReason),
	)
})

var _ = Describe("HCloudMachine validation", func() {
	var (
		hcloudMachine *infrav1.HCloudMachine
		testNs        *corev1.Namespace
	)

	BeforeEach(func() {
		hcloudClient.Reset()

		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "hcloudmachine-validation")
		Expect(err).NotTo(HaveOccurred())

		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloud-validation-machine",
				Namespace: testNs.Name,
			},
			Spec: infrav1.HCloudMachineSpec{
				ImageName: "my-control-plane",
				Type:      "cpx31",
			},
		}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, hcloudMachine)).To(Succeed())
	})

	It("should fail with wrong type", func() {
		hcloudMachine.Spec.Type = "wrong-type"
		Expect(testEnv.Create(ctx, hcloudMachine)).ToNot(Succeed())
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

		hcloudMachine.Spec.Type = "cpx32"
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
		hcloudClient.Reset()

		predicate = IgnoreInsignificantHetznerClusterUpdates(klog.Background())

		oldCluster = &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-predicate", ResourceVersion: "1"},
			Spec:       getDefaultHetznerClusterSpec(),
			Status: infrav1.HetznerClusterStatus{
				Conditions: []clusterv1.Condition{},
			},
		}
		conditions.MarkTrue(oldCluster, infrav1.CredentialsAvailableCondition)

		newCluster = oldCluster.DeepCopy()
	})

	It("should skip updates to the HetznerCluster conditions", func() {
		// Make change to conditions & other fields that get changed on every update
		conditions.MarkFalse(newCluster, infrav1.CredentialsAvailableCondition, infrav1.HCloudCredentialsInvalidReason, clusterv1.ConditionSeverityError, "")
		newCluster.ObjectMeta.ResourceVersion = "2"
		newCluster.ObjectMeta.SetManagedFields([]metav1.ManagedFieldsEntry{{
			Manager:   "test",
			Operation: "update",
		}})

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
