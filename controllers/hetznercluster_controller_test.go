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
	"context"
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Hetzner ClusterReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an HetznerCluster", func() {
		It("should create a cluster", func() {
			// Create the secret
			hetznerSecret := getDefaultHetznerSecret("default")
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())
			defer func() {
				Expect(testEnv.Delete(ctx, hetznerSecret)).To(Succeed())
			}()

			// Create the HetznerCluster object
			instance := &infrav1.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "hetzner-test1",
					Namespace:    "default",
				},
				Spec: getDefaultHetznerClusterSpec(),
			}
			Expect(testEnv.Create(ctx, instance)).To(Succeed())
			defer func() {
				Expect(testEnv.Delete(ctx, instance)).To(Succeed())
			}()

			key := client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}

			// Create capi cluster
			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test1-",
					Namespace:    "default",
					Finalizers:   []string{clusterv1.ClusterFinalizer},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "HetznerCluster",
						Name:       instance.Name,
					},
				},
			}
			Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())
			defer func() {
				Expect(testEnv.Cleanup(ctx, capiCluster)).To(Succeed())
			}()

			// Make sure the HetznerCluster exists.
			Eventually(func() error {
				return testEnv.Get(ctx, key, instance)
			}, timeout).Should(BeNil())

			By("setting the OwnerRef on the HetznerCluster")
			// Set owner reference to Hetzner cluster
			Eventually(func() error {
				ph, err := patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				instance.OwnerReferences = append(instance.OwnerReferences, metav1.OwnerReference{
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
					Name:       capiCluster.Name,
					UID:        capiCluster.UID,
				})
				return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
			}, timeout).Should(BeNil())

			// Check whether finalizer has been set for HetznerCluster
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					fmt.Println("error while getting instance", err)
					return false
				}
				return len(instance.Finalizers) > 0
			}, timeout, time.Second).Should(BeTrue())
		})
	})

	Context("For HetznerMachines belonging to the cluster", func() {
		var (
			namespace       string
			testNs          *corev1.Namespace
			hetznerSecret   *corev1.Secret
			bootstrapSecret *corev1.Secret
		)

		BeforeEach(func() {
			var err error
			testNs, err = testEnv.CreateNamespace(ctx, "hetzner-owner-ref")
			Expect(err).NotTo(HaveOccurred())
			namespace = testNs.Name

			// Create the secret
			hetznerSecret = getDefaultHetznerSecret(namespace)
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			bootstrapSecret = getDefaultBootstrapSecret(namespace)
			Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Delete(ctx, hetznerSecret)).To(Succeed())
			Expect(testEnv.Delete(ctx, bootstrapSecret)).To(Succeed())
			Expect(testEnv.Delete(ctx, testNs)).To(Succeed())
		})

		It("sets owner references to those machines", func() {
			// Create the HetznerCluster object
			instance := &infrav1.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test1-",
					Namespace:    namespace,
				},
				Spec: getDefaultHetznerClusterSpec(),
			}
			Expect(testEnv.Create(ctx, instance)).To(Succeed())

			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "capi-test1-",
					Namespace:    namespace,
					Finalizers:   []string{clusterv1.ClusterFinalizer},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "HetznerCluster",
						Name:       instance.Name,
						Namespace:  namespace,
					},
				},
			}
			Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

			// Make sure the HCloudCluster exists.
			Eventually(func() error {
				return testEnv.Get(ctx, client.ObjectKey{Namespace: namespace, Name: instance.Name}, instance)
			}, timeout).Should(BeNil())

			// Create machines
			machineCount := 3
			for i := 0; i < machineCount; i++ {
				Expect(createHCloudMachine(ctx, testEnv, namespace, capiCluster.Name)).To(Succeed())
			}

			// Set owner reference to HetznerCluster
			Eventually(func() bool {
				ph, err := patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				instance.OwnerReferences = append(instance.OwnerReferences, metav1.OwnerReference{
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
					Name:       capiCluster.Name,
					UID:        capiCluster.UID,
				})
				Expect(ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})).ShouldNot(HaveOccurred())
				return true
			}, timeout).Should(BeTrue())

			By("checking for presence of HCloudMachine objects")
			hcc := testEnv.HCloudClientFactory.NewClient("")
			// Check if machines have been created
			Eventually(func() int {
				servers, err := hcc.ListServers(ctx, hcloud.ServerListOpts{
					ListOpts: hcloud.ListOpts{
						LabelSelector: utils.LabelsToLabelSelector(map[string]string{infrav1.ClusterTagKey(instance.Name): "owned"}),
					},
				})
				if err != nil {
					return -1
				}
				return len(servers)
			}, timeout).Should(Equal(machineCount))
		})
	})
})

func createHCloudMachine(ctx context.Context, env *helpers.TestEnvironment, namespace, clusterName string) error {
	hcloudMachineName := utils.GenerateName(nil, "hcloud-machine")
	failureDomain := "fsn1"
	capiMachine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "capi-machine-",
			Namespace:    namespace,
			Finalizers:   []string{clusterv1.MachineFinalizer},
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: clusterName,
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
	if err := env.Create(ctx, capiMachine); err != nil {
		return err
	}

	hcloudMachine := &infrav1.HCloudMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcloudMachineName,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterLabelName: clusterName},
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
			ImageName: "1.23.3-fedora-35-control-plane",
			Type:      "cpx31",
		},
	}

	return env.Create(ctx, hcloudMachine)
}
