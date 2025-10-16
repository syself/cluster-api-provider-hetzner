/*
Copyright 2023 The Kubernetes Authors.

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

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

var _ = Describe("HCloudRemediationReconciler", func() {
	var (
		hcloudRemediation *infrav1.HCloudRemediation
		hcloudMachine     *infrav1.HCloudMachine
		hetznerCluster    *infrav1.HetznerCluster

		capiMachine *clusterv1.Machine
		capiCluster *clusterv1.Cluster

		hetznerSecret   *corev1.Secret
		bootstrapSecret *corev1.Secret

		testNs *corev1.Namespace

		hcloudRemediationkey client.ObjectKey
		capiMachineKey       client.ObjectKey
		hcloudMachineKey     client.ObjectKey
	)

	BeforeEach(func() {
		hcloudClient.Reset()
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "hcloudmachinetemplate-reconciler")
		Expect(err).NotTo(HaveOccurred())

		hetznerSecret = getDefaultHetznerSecret(testNs.Name)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		bootstrapSecret = getDefaultBootstrapSecret(testNs.Name)
		Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())

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

		hcloudMachineName := utils.GenerateName(nil, "hcloud-machine-")
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
					Namespace:  testNs.Name,
				},
				FailureDomain: &defaultFailureDomain,
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		capiMachineKey = client.ObjectKey{Name: capiMachineName, Namespace: testNs.Name}

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
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

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

		hcloudMachineKey = client.ObjectKey{Name: hcloudMachineName, Namespace: testNs.Name}

		hcloudRemediation = &infrav1.HCloudRemediation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloud-remediation",
				Namespace: testNs.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Machine",
						APIVersion: clusterv1.GroupVersion.String(),
						Name:       capiMachine.Name,
						UID:        capiMachine.UID,
					},
				},
			},
			Spec: infrav1.HCloudRemediationSpec{
				Strategy: &infrav1.RemediationStrategy{
					Type:       "Reboot",
					RetryLimit: 1,
					Timeout:    &metav1.Duration{Duration: 1 * time.Second},
				},
			},
		}

		hcloudRemediationkey = client.ObjectKey{Namespace: testNs.Name, Name: "hcloud-remediation"}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, hcloudRemediation, hcloudMachine,
			hetznerSecret, bootstrapSecret, hetznerCluster, capiMachine, capiCluster)).To(Succeed())
	})

	Context("Basic hcloudremediation test", func() {
		It("creates the hcloudRemediation successfully", func() {
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			Eventually(func() error {
				return testEnv.Get(ctx, hcloudRemediationkey, hcloudRemediation)
			}, timeout).Should(BeNil())
		})

		It("checks if hcloudRemediation objects has the HCloudTokenAvailableCondition condition", func() {
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			Eventually(func() bool {
				return isPresentAndTrue(hcloudMachineKey, hcloudRemediation, infrav1.HCloudTokenAvailableCondition)
			})
		})

		It("checks that no remediation is tried if HCloud server does not exist anymore", func() {
			By("ensuring if hcloudMachine is provisioned")
			Eventually(func() bool {
				if err := testEnv.Get(ctx, hcloudMachineKey, hcloudMachine); err != nil {
					return false
				}

				testEnv.GetLogger().Info("Status of the hcloudmachine", "status", hcloudMachine.Status)
				return hcloudMachine.Status.Ready
			}, timeout).Should(BeTrue())

			By("deleting the server associated with the hcloudMachine")
			providerID, err := hcloudutil.ServerIDFromProviderID(hcloudMachine.Spec.ProviderID)
			Expect(err).NotTo(HaveOccurred())

			Expect(hcloudClient.DeleteServer(ctx, &hcloud.Server{ID: providerID})).NotTo(HaveOccurred())

			By("creating the hcloudRemediation")
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			By("checking if hcloudRemediation is in deleting phase and capiMachine has the MachineOwnerRemediatedCondition")
			Eventually(func() bool {
				if err := testEnv.Get(ctx, hcloudRemediationkey, hcloudRemediation); err != nil {
					return false
				}

				return hcloudRemediation.Status.Phase == infrav1.PhaseDeleting &&
					isPresentAndFalseWithReason(capiMachineKey, capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason)
			}, timeout).Should(BeTrue())
		})

		It("checks that, under normal conditions, a reboot is carried out and retryCount and lastRemediated are set", func() {
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			Eventually(func() error {
				if err := testEnv.Get(ctx, hcloudRemediationkey, hcloudRemediation); err != nil {
					return err
				}

				if hcloudRemediation.Status.LastRemediated == nil {
					return fmt.Errorf("hcloudRemediation.Status.LastRemediated == nil")
				}
				if hcloudRemediation.Status.RetryCount != 1 {
					return fmt.Errorf("hcloudRemediation.Status.RetryCount is %d", hcloudRemediation.Status.RetryCount)
				}
				return nil
			}, timeout).ToNot(HaveOccurred())
		})

		It("checks if PhaseWaiting is set when retryLimit is reached", func() {
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, hcloudRemediationkey, hcloudRemediation); err != nil {
					return false
				}

				testEnv.GetLogger().Info("status of hcloudRemediation", "status", hcloudRemediation.Status.Phase)
				return hcloudRemediation.Status.Phase == infrav1.PhaseWaiting
			}, timeout).Should(BeTrue())
		})

		It("should delete machine if retry limit reached and reboot timed out (hcloud)", func() {
			By("creating hcloudRemediation")
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			By("updating the status to waiting and setting the last remediation to past")
			hcloudRemediationPatchHelper, err := patch.NewHelper(hcloudRemediation, testEnv.GetClient())
			Expect(err).NotTo(HaveOccurred())

			hcloudRemediation.Status.Phase = infrav1.PhaseWaiting
			hcloudRemediation.Status.LastRemediated = &metav1.Time{Time: time.Now().Add(-2 * time.Second)}

			Expect(hcloudRemediationPatchHelper.Patch(ctx, hcloudRemediation)).NotTo(HaveOccurred())

			By("checking if hcloudRemediation is in deleting phase and capiMachine has MachineOwnerRemediatedCondition")
			Eventually(func() bool {
				if err := testEnv.Get(ctx, hcloudRemediationkey, hcloudRemediation); err != nil {
					return false
				}

				testEnv.GetLogger().Info("status of hcloudRemediation", "status", hcloudRemediation.Status.Phase)
				return hcloudRemediation.Status.Phase == infrav1.PhaseDeleting &&
					isPresentAndFalseWithReason(capiMachineKey, capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason)
			}, timeout).Should(BeTrue())
		})
	})
})
