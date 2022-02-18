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

package controllers_test

import (
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("VsphereMachineReconciler", func() {
	var (
		capiCluster *clusterv1.Cluster
		capiMachine *clusterv1.Machine

		infraCluster *infrav1.HetznerCluster
		infraMachine *infrav1.HCloudMachine

		testNs *corev1.Namespace

		hetznerSecret   *corev1.Secret
		bootstrapSecret *corev1.Secret

		failureDomain = "fsn1"
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
					FailureDomain: &failureDomain,
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			pgName := "control-plane"

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
					ImageName:          "1.23.4-fedora-35-control-plane",
					Type:               "cpx31",
					PlacementGroupName: &pgName,
				},
			}

			Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())
			Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())
		})

		It("creates the infra machine", func() {
			key := client.ObjectKey{Namespace: testNs.Name, Name: infraMachine.Name}
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

			By("setting the bootstrap data")
			Eventually(func() error {
				ph, err := patch.NewHelper(capiMachine, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					DataSecretName: pointer.String("bootstrap-secret"),
				}
				return ph.Patch(ctx, capiMachine, patch.WithStatusObservedGeneration{})
			}, timeout, time.Second).Should(BeNil())

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
					FailureDomain: &failureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.String("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			pgName := "control-plane"

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
					ImageName:          "1.23.4-fedora-35-control-plane",
					Type:               "cpx31",
					PlacementGroupName: &pgName,
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

		Context("without ssh keys", func() {
			BeforeEach(func() {
				infraCluster.Spec.SSHKeys = infrav1.HetznerSSHKeys{}
				Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())
				Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())
			})

			It("should show the expected reason for server not created", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, infraMachine); err != nil {
						return false
					}
					return isPresentAndFalseWithReason(key, infraMachine, infrav1.InstanceReadyCondition, infrav1.InstanceHasNoValidSSHKeyReason)
				}, timeout).Should(BeTrue())
			})
		})
	})
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
				ImageName: "1.23.4-fedora-35-control-plane",
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
