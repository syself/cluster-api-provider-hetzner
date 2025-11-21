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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/machinetemplate"
)

var _ = Describe("HCloudMachineTemplateReconciler", func() {
	var (
		machineTemplate *infrav1.HCloudMachineTemplate
		testNs          *corev1.Namespace
		hetznerSecret   *corev1.Secret
		key             client.ObjectKey
	)

	BeforeEach(func() {
		hcloudClient.Reset()
		var err error
		testNs, err = testEnv.ResetAndCreateNamespace(ctx, "hcloudmachinetemplate-reconciler")
		Expect(err).NotTo(HaveOccurred())

		hetznerSecret = getDefaultHetznerSecret(testNs.Name)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		key = client.ObjectKey{Namespace: testNs.Name, Name: "hcloud-machine-template"}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, hetznerSecret)).To(Succeed())
	})

	Context("Basic hcloudmachinetemplate test", func() {
		Context("ClusterClass test", func() {
			var capiClusterClass *clusterv1.ClusterClass

			BeforeEach(func() {
				capiClusterClass = &clusterv1.ClusterClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-class",
						Namespace: testNs.Name,
					},
					Spec: clusterv1.ClusterClassSpec{
						ControlPlane: clusterv1.ControlPlaneClass{
							MachineInfrastructure: &clusterv1.LocalObjectTemplate{
								Ref: &corev1.ObjectReference{
									APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
									Kind:       "HCloudMachineTemplate",
									Name:       "hcloud-machine-template",
									Namespace:  testNs.Name,
								},
							},
							LocalObjectTemplate: clusterv1.LocalObjectTemplate{
								Ref: &corev1.ObjectReference{
									APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
									Kind:       "KubeadmControlPlaneTemplate",
									Name:       "quick-start-control-plane",
									Namespace:  testNs.Name,
								},
							},
						},
						Infrastructure: clusterv1.LocalObjectTemplate{
							Ref: &corev1.ObjectReference{
								APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
								Kind:       "HetznerClusterTemplate",
								Name:       "hcloud-cluster-template",
								Namespace:  testNs.Name,
							},
						},
					},
				}
				Expect(testEnv.Create(ctx, capiClusterClass)).To(Succeed())

				machineTemplate = &infrav1.HCloudMachineTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hcloud-machine-template",
						Namespace: testNs.Name,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "cluster.x-k8s.io/v1beta1",
								Kind:       "ClusterClass",
								Name:       capiClusterClass.Name,
								UID:        capiClusterClass.UID,
							},
						},
					},
					Spec: infrav1.HCloudMachineTemplateSpec{
						Template: infrav1.HCloudMachineTemplateResource{
							Spec: infrav1.HCloudMachineSpec{
								ImageName:          "my-control-plane",
								Type:               "cpx31",
								PlacementGroupName: &defaultPlacementGroupName,
							},
						},
					},
				}
				Expect(testEnv.Create(ctx, machineTemplate)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, capiClusterClass, machineTemplate)).To(Succeed())
			})

			It("checks clusterClass ownership", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, machineTemplate); err != nil {
						testEnv.GetLogger().Error(err, "did not find machine template")
						return false
					}

					testEnv.GetLogger().Info("found the machine template", "OwnerType", machineTemplate.Status.OwnerType)
					return machineTemplate.Status.OwnerType == "ClusterClass"
				}, timeout).Should(BeTrue())
			})
		})

		Context("Cluster test", func() {
			var (
				capiCluster    *clusterv1.Cluster
				hetznerCluster *infrav1.HetznerCluster
			)

			BeforeEach(func() {
				hcloudClient.Reset()

				capiCluster = &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-",
						Namespace:    testNs.Name,
						Finalizers:   []string{clusterv1.ClusterFinalizer},
					},
					Spec: clusterv1.ClusterSpec{
						InfrastructureRef: &corev1.ObjectReference{
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
							Kind:       "HetznerCluster",
							Name:       "hetzner-test",
							Namespace:  testNs.Name,
						},
					},
				}
				Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

				hetznerCluster = &infrav1.HetznerCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hetzner-test",
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

				machineTemplate = &infrav1.HCloudMachineTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "hcloud-machine-template",
						Namespace:       testNs.Name,
						OwnerReferences: hetznerCluster.OwnerReferences,
					},
					Spec: infrav1.HCloudMachineTemplateSpec{
						Template: infrav1.HCloudMachineTemplateResource{
							Spec: infrav1.HCloudMachineSpec{
								ImageName:          "my-control-plane",
								Type:               "cpx31",
								PlacementGroupName: &defaultPlacementGroupName,
							},
						},
					},
				}
				Expect(testEnv.Create(ctx, machineTemplate)).To(Succeed())
			})

			It("checks the Cluster ownership", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, machineTemplate); err != nil {
						testEnv.GetLogger().Error(err, "did not find machine template")
						return false
					}

					testEnv.GetLogger().Info("found the machine template", "OwnerType", machineTemplate.Status.OwnerType)
					return machineTemplate.Status.OwnerType == "Cluster"
				}, timeout).Should(BeTrue())
			})

			It("sets the capacity in status", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, machineTemplate); err != nil {
						testEnv.GetLogger().Error(err, "did not find machine template")
						return false
					}

					// If capacity is not set (yet), there is nothing to compare
					if machineTemplate.Status.Capacity.Cpu() == nil || machineTemplate.Status.Capacity.Memory() == nil {
						testEnv.GetLogger().Info("capacity not set", "capacity", machineTemplate.Status.Capacity)
						return false
					}

					// compare CPU
					expectedCPU, err := machinetemplate.GetCPUQuantityFromInt(fake.DefaultCPUCores)
					Expect(err).To(Succeed())

					if !expectedCPU.Equal(*machineTemplate.Status.Capacity.Cpu()) {
						testEnv.GetLogger().Info("cpu did not equal", "expected", expectedCPU, "actual", machineTemplate.Status.Capacity.Cpu())
						return false
					}

					// compare memory
					expectedMemory, err := machinetemplate.GetMemoryQuantityFromFloat32(fake.DefaultMemoryInGB)
					Expect(err).To(Succeed())

					if !expectedMemory.Equal(*machineTemplate.Status.Capacity.Memory()) {
						testEnv.GetLogger().Info("memory did not equal", "expected", expectedMemory, "actual", machineTemplate.Status.Capacity.Memory())
						return false
					}

					return true
				}, timeout, interval).Should(BeTrue())
			})
		})
		Context("HCloudMachineTemplate Webhook Validation", func() {
			var (
				hcloudMachineTemplate *infrav1.HCloudMachineTemplate
				testNs                *corev1.Namespace
			)
			BeforeEach(func() {
				var err error
				testNs, err = testEnv.ResetAndCreateNamespace(ctx, "hcloudmachine-validation")
				Expect(err).NotTo(HaveOccurred())

				hcloudMachineTemplate = &infrav1.HCloudMachineTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hcloud-validation-machine",
						Namespace: testNs.Name,
					},
					Spec: infrav1.HCloudMachineTemplateSpec{
						Template: infrav1.HCloudMachineTemplateResource{
							Spec: infrav1.HCloudMachineSpec{
								Type:      "cx41",
								ImageName: "my-hcloud-image",
							},
						},
					},
				}
				Expect(testEnv.Client.Create(ctx, hcloudMachineTemplate)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: "hcloud-validation-machine"}
				Eventually(func() error {
					return testEnv.Client.Get(ctx, key, hcloudMachineTemplate)
				}, timeout, interval).Should(BeNil())
			})
			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, testNs, hcloudMachineTemplate)).To(Succeed())
			})

			It("should prevent updating type", func() {
				Expect(testEnv.Get(ctx, key, machineTemplate)).To(Succeed())

				hcloudMachineTemplate.Spec.Template.Spec.Type = "cpx32"
				Expect(testEnv.Client.Update(ctx, hcloudMachineTemplate)).ToNot(Succeed())
			})

			It("should prevent updating Image name", func() {
				Expect(testEnv.Get(ctx, key, machineTemplate)).To(Succeed())

				hcloudMachineTemplate.Spec.Template.Spec.ImageName = "my-control-plane"
				Expect(testEnv.Client.Update(ctx, hcloudMachineTemplate)).ToNot(Succeed())
			})

			It("should prevent updating SSHKey", func() {
				Expect(testEnv.Get(ctx, key, machineTemplate)).To(Succeed())

				hcloudMachineTemplate.Spec.Template.Spec.SSHKeys = []infrav1.SSHKey{{Name: "ssh-key-1"}}
				Expect(testEnv.Client.Update(ctx, hcloudMachineTemplate)).ToNot(Succeed())
			})

			It("should prevent updating PlacementGroups", func() {
				Expect(testEnv.Get(ctx, key, machineTemplate)).To(Succeed())

				hcloudMachineTemplate.Spec.Template.Spec.PlacementGroupName = createPlacementGroupName("placement-group-1")
				Expect(testEnv.Client.Update(ctx, hcloudMachineTemplate)).ToNot(Succeed())
			})

			It("should succeed for mutable fields", func() {
				Expect(testEnv.Get(ctx, key, machineTemplate)).To(Succeed())

				hcloudMachineTemplate.Status.Conditions = clusterv1.Conditions{
					{
						Type:    "TestSuccessful",
						Status:  corev1.ConditionTrue,
						Reason:  "TestPassed",
						Message: "The test was successful",
					},
				}
				Expect(testEnv.Client.Update(ctx, hcloudMachineTemplate)).To(Succeed())
			})
		})
	})
})

func createPlacementGroupName(name string) *string {
	return &name
}
