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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/machinetemplate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("HCloudMachineTemplateReconciler", func() {
	var (
		capiCluster  *clusterv1.Cluster
		infraCluster *infrav1.HetznerCluster

		machineTemplate *infrav1.HCloudMachineTemplate

		testNs *corev1.Namespace

		hetznerSecret *corev1.Secret

		key client.ObjectKey
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "hcloudmachinetemplate-reconciler")
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

		Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())

		machineTemplate = &infrav1.HCloudMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "hcloud-machine-template-",
				Namespace:    testNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterLabelName: capiCluster.Name,
				},
			},
			Spec: infrav1.HCloudMachineTemplateSpec{
				Template: infrav1.HCloudMachineTemplateResource{
					Spec: infrav1.HCloudMachineSpec{
						ImageName:          "fedora-control-plane",
						Type:               "cpx31",
						PlacementGroupName: &defaultPlacementGroupName,
					},
				},
			},
		}
		Expect(testEnv.Create(ctx, machineTemplate)).To(Succeed())

		hetznerSecret = getDefaultHetznerSecret(testNs.Name)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		key = client.ObjectKey{Namespace: testNs.Name, Name: machineTemplate.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, infraCluster,
			machineTemplate, hetznerSecret)).To(Succeed())
	})

	Context("Basic test", func() {
		It("sets the capacity in status", func() {
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, machineTemplate); err != nil {
					fmt.Printf("Did not find machine template: %s\n", err)
					return false
				}

				fmt.Printf("Capacity: %v\n", machineTemplate.Status.Capacity)

				// If capacity is not set (yet), there is nothing to compare
				if machineTemplate.Status.Capacity.Cpu() == nil || machineTemplate.Status.Capacity.Memory() == nil {
					fmt.Printf("Capacity not set: %v\n", machineTemplate.Status.Capacity)
					return false
				}

				// Compare CPU
				expectedCPU, err := machinetemplate.GetCPUQuantityFromInt(fake.DefaultCPUCores)
				Expect(err).To(Succeed())
				if !expectedCPU.Equal(*machineTemplate.Status.Capacity.Cpu()) {
					fmt.Printf("CPU did not equal: Expected: %v. Actual: %v\n", expectedCPU, machineTemplate.Status.Capacity.Cpu())
					return false
				}

				// Compare memory
				expectedMemory, err := machinetemplate.GetMemoryQuantityFromFloat32(fake.DefaultMemoryInGB)
				Expect(err).To(Succeed())
				if !expectedMemory.Equal(*machineTemplate.Status.Capacity.Memory()) {
					fmt.Printf("Memory did not equal: Expected: %v. Actual: %v\n", expectedMemory, machineTemplate.Status.Capacity.Memory())
					return false
				}
				return expectedMemory.Equal(*machineTemplate.Status.Capacity.Memory())
			}, 10*time.Second, time.Second).Should(BeTrue())
		})
	})
})
