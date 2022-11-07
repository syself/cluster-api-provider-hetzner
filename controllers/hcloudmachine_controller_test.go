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
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2/extensions/table"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("HCloudMachineReconciler", func() {
	var (
		capiCluster *clusterv1.Cluster
		capiMachine *clusterv1.Machine

		infraCluster *infrav1.HetznerCluster
		infraMachine *infrav1.HCloudMachine

		testNs *corev1.Namespace

		hetznerSecret   *corev1.Secret
		bootstrapSecret *corev1.Secret

		key client.ObjectKey
	)

	BeforeEach(func() {
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
			Status: clusterv1.ClusterStatus{
				InfrastructureReady: true,
			},
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		infraCluster = &infrav1.HetznerCluster{
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
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, infraCluster, capiMachine,
			infraMachine, hetznerSecret, bootstrapSecret)).To(Succeed())
	})

	Context("Basic test", func() {
		BeforeEach(func() {
			hcloudMachineName := utils.GenerateName(nil, "hcloud-machine-")

			capiMachine = &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "capi-machine-",
					Namespace:    testNs.Name,
					Finalizers:   []string{clusterv1.MachineFinalizer},
					Labels: map[string]string{
						clusterv1.ClusterLabelName: capiCluster.Name,
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
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			infraMachine = &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hcloudMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterLabelName:             capiCluster.Name,
						clusterv1.MachineControlPlaneLabelName: "",
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
					ImageName:          "fedora-control-plane",
					Type:               "cpx31",
					PlacementGroupName: &defaultPlacementGroupName,
				},
			}

			Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())
			Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: infraMachine.Name}
		})

		It("creates the infra machine", func() {
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, infraMachine); err != nil {
					return false
				}
				return true
			}, timeout).Should(BeTrue())
		})

		It("creates the HCloud machine in Hetzner", func() {
			Eventually(func() bool {
				servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
					ListOpts: hcloud.ListOpts{
						LabelSelector: utils.LabelsToLabelSelector(map[string]string{infrav1.ClusterTagKey(infraCluster.Name): "owned"}),
					},
				})
				if err != nil {
					return false
				}
				return len(servers) == 0
			}).Should(BeTrue())

			// Check whether bootstrap condition is not ready
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, infraMachine); err != nil {
					return false
				}
				return isPresentAndFalseWithReason(key, infraMachine, infrav1.InstanceBootstrapReadyCondition, infrav1.InstanceBootstrapNotReadyReason)
			}, timeout, time.Second).Should(BeTrue())

			By("setting the bootstrap data")
			Eventually(func() error {
				ph, err := patch.NewHelper(capiMachine, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					DataSecretName: pointer.String("bootstrap-secret"),
				}
				return ph.Patch(ctx, capiMachine, patch.WithStatusObservedGeneration{})
			}, timeout, time.Second).Should(BeNil())

			// Check whether bootstrap condition is ready
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, infraMachine); err != nil {
					return false
				}
				objectCondition := conditions.Get(infraMachine, infrav1.InstanceBootstrapReadyCondition)
				fmt.Println(objectCondition)
				return isPresentAndTrue(key, infraMachine, infrav1.InstanceBootstrapReadyCondition)
			}, timeout, time.Second).Should(BeTrue())

			Eventually(func() int {
				servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
					ListOpts: hcloud.ListOpts{
						LabelSelector: utils.LabelsToLabelSelector(map[string]string{infrav1.ClusterTagKey(infraCluster.Name): "owned"}),
					},
				})
				if err != nil {
					return 0
				}
				return len(servers)
			}, timeout, time.Second).Should(BeNumerically(">", 0))
		})
	})

	Context("various specs", func() {
		isPresentAndFalseWithReason := func(key types.NamespacedName, getter conditions.Getter, condition clusterv1.ConditionType, reason string) bool {
			ExpectWithOffset(1, testEnv.Get(ctx, key, getter)).To(Succeed())
			if !conditions.Has(getter, condition) {
				return false
			}
			objectCondition := conditions.Get(getter, condition)
			return objectCondition.Status == corev1.ConditionFalse &&
				objectCondition.Reason == reason
		}

		var key client.ObjectKey

		BeforeEach(func() {
			hcloudMachineName := utils.GenerateName(nil, "hcloud-machine-")

			capiMachine = &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "capi-machine-",
					Namespace:    testNs.Name,
					Finalizers:   []string{clusterv1.MachineFinalizer},
					Labels: map[string]string{
						clusterv1.ClusterLabelName: capiCluster.Name,
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
						DataSecretName: pointer.String("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			infraMachine = &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hcloudMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterLabelName:             capiCluster.Name,
						clusterv1.MachineControlPlaneLabelName: "",
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
					ImageName:          "fedora-control-plane",
					Type:               "cpx31",
					PlacementGroupName: &defaultPlacementGroupName,
				},
			}

			key = client.ObjectKey{Namespace: testNs.Name, Name: infraMachine.Name}
		})

		Context("without network", func() {
			BeforeEach(func() {
				infraCluster.Spec.HCloudNetwork.Enabled = false
				Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())
				Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())
			})

			It("creates the HCloud machine in Hetzner", func() {
				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{infrav1.ClusterTagKey(infraCluster.Name): "owned"}),
						},
					})
					if err != nil {
						return 0
					}
					return len(servers)
				}, timeout, time.Second).Should(BeNumerically(">", 0))
			})
		})

		Context("without placement groups", func() {
			BeforeEach(func() {
				infraCluster.Spec.HCloudPlacementGroup = nil
				Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())
				infraMachine.Spec.PlacementGroupName = nil
				Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())
			})

			It("creates the HCloud machine in Hetzner", func() {
				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{infrav1.ClusterTagKey(infraCluster.Name): "owned"}),
						},
					})
					if err != nil {
						return 0
					}
					return len(servers)
				}, timeout, time.Second).Should(BeNumerically(">", 0))
			})
		})

		Context("without placement groups, but with placement group in hcloudMachine spec", func() {
			BeforeEach(func() {
				infraCluster.Spec.HCloudPlacementGroup = nil
				Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())
				Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())
			})

			It("should show the expected reason for server not created", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, infraMachine); err != nil {
						return false
					}
					return isPresentAndFalseWithReason(key, infraMachine, infrav1.InstanceReadyCondition, infrav1.InstanceHasNonExistingPlacementGroupReason)
				}, timeout).Should(BeTrue())
			})
		})

		Context("with public network specs", func() {
			BeforeEach(func() {
				infraMachine.Spec.PublicNetwork = &infrav1.PublicNetworkSpec{
					EnableIPv4: false,
					EnableIPv6: false,
				}
				Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())
				Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())
			})

			It("creates the HCloud machine in Hetzner", func() {
				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{infrav1.ClusterTagKey(infraCluster.Name): "owned"}),
						},
					})
					if err != nil {
						return 0
					}
					return len(servers)
				}, timeout, time.Second).Should(BeNumerically(">", 0))
			})
		})
	})
})

var _ = Describe("Hetzner secret", func() {
	var (
		hetznerCluster     *infrav1.HetznerCluster
		capiCluster        *clusterv1.Cluster
		hcloudMachine      *infrav1.HCloudMachine
		capiMachine        *clusterv1.Machine
		key                client.ObjectKey
		hetznerSecret      *corev1.Secret
		hetznerClusterName string
	)

	BeforeEach(func() {
		var err error
		Expect(err).NotTo(HaveOccurred())

		hetznerClusterName = utils.GenerateName(nil, "hetzner-cluster-test")
		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    "default",
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "HetznerCluster",
					Name:       hetznerClusterName,
					Namespace:  "default",
				},
			},
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		hetznerCluster = &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hetznerClusterName,
				Namespace: "default",
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
				Namespace:    "default",
				Finalizers:   []string{clusterv1.MachineFinalizer},
				Labels: map[string]string{
					clusterv1.ClusterLabelName: capiCluster.Name,
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
					DataSecretName: pointer.String("bootstrap-secret"),
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hcloudMachineName,
				Namespace: "default",
				Labels: map[string]string{
					clusterv1.ClusterLabelName:             capiCluster.Name,
					clusterv1.MachineControlPlaneLabelName: "",
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
				ImageName:          "fedora-control-plane",
				Type:               "cpx31",
				PlacementGroupName: &defaultPlacementGroupName,
			},
		}
		Expect(testEnv.Create(ctx, hcloudMachine)).To(Succeed())
		key = client.ObjectKey{Namespace: "default", Name: hcloudMachine.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, hetznerCluster, capiCluster, hcloudMachine, capiMachine, hetznerSecret)).To(Succeed())

		Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKey{Namespace: hetznerSecret.Namespace, Name: hetznerSecret.Name}, hetznerSecret); err != nil && apierrors.IsNotFound(err) {
				return true
			} else if err != nil {
				return false
			}
			// Secret still there, so the finalizers have not been removed. Patch to remove them.
			ph, err := patch.NewHelper(hetznerSecret, testEnv)
			Expect(err).ShouldNot(HaveOccurred())
			hetznerSecret.Finalizers = nil
			Expect(ph.Patch(ctx, hetznerSecret, patch.WithStatusObservedGeneration{})).To(Succeed())
			// Should delete secret
			if err := testEnv.Delete(ctx, hetznerSecret); err != nil && apierrors.IsNotFound(err) {
				// Has been deleted already
				return true
			}
			return false
		}, time.Second, time.Second).Should(BeTrue())
	})

	DescribeTable("test different hetzner secret",
		func(secret corev1.Secret, expectedReason string) {
			hetznerSecret = &secret
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, hcloudMachine); err != nil {
					return false
				}
				return isPresentAndFalseWithReason(key, hcloudMachine, infrav1.InstanceReadyCondition, expectedReason)
			}, timeout, time.Second).Should(BeTrue())
		},
		Entry("no Hetzner secret/wrong reference", corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wrong-name",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"hcloud": []byte("my-token"),
			},
		}, infrav1.HetznerSecretUnreachableReason),
		Entry("empty hcloud token", corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hetzner-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"hcloud": []byte(""),
			},
		}, infrav1.HCloudCredentialsInvalidReason),
		Entry("wrong key in secret", corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hetzner-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"wrongkey": []byte("my-token"),
			},
		}, infrav1.HCloudCredentialsInvalidReason),
	)
})

var _ = Describe("HCloudMachine validation", func() {
	var (
		infraMachine *infrav1.HCloudMachine
		testNs       *corev1.Namespace
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "hcloudmachine-validation")
		Expect(err).NotTo(HaveOccurred())

		infraMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloud-validation-machine",
				Namespace: testNs.Name,
			},
			Spec: infrav1.HCloudMachineSpec{
				ImageName: "fedora-control-plane",
				Type:      "cpx31",
			},
		}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, infraMachine)).To(Succeed())
	})

	It("should fail with wrong type", func() {
		infraMachine.Spec.Type = "wrong-type"
		Expect(testEnv.Create(ctx, infraMachine)).ToNot(Succeed())
	})

	It("should fail without imageName", func() {
		infraMachine.Spec.ImageName = ""
		Expect(testEnv.Create(ctx, infraMachine)).ToNot(Succeed())
	})
})
