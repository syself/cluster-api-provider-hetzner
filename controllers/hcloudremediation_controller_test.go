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
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
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
		var err error
		testNs, err = testEnv.ResetAndCreateNamespace(ctx, "hcloudmachinetemplate-reconciler")
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
				ImageName: "my-control-plane",
				Type:      "cpx32",
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
			Eventually(func() error {
				if err := testEnv.Get(ctx, hcloudMachineKey, hcloudMachine); err != nil {
					return err
				}

				if !hcloudMachine.Status.Ready {
					return fmt.Errorf("hcloudMachine.Status.Ready is not true (yet)")
				}
				return nil
			}, timeout).ShouldNot(HaveOccurred())

			By("deleting the server associated with the hcloudMachine")
			providerID, err := hcloudutil.ServerIDFromProviderID(hcloudMachine.Spec.ProviderID)
			Expect(err).NotTo(HaveOccurred())

			hcloudClient := testEnv.HCloudClientFactory.NewClient("fake-token")
			Expect(hcloudClient.DeleteServer(ctx, &hcloud.Server{ID: providerID})).NotTo(HaveOccurred())

			By("creating the hcloudRemediation")
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			By("checking if hcloudRemediation is in deleting phase and capiMachine has the MachineOwnerRemediatedCondition")
			Eventually(func() error {
				if err := testEnv.Get(ctx, hcloudRemediationkey, hcloudRemediation); err != nil {
					return err
				}
				if hcloudRemediation.Status.Phase != infrav1.PhaseDeleting {
					return fmt.Errorf("hcloudRemediation.Status.Phase is not infrav1.PhaseDeleting")
				}
				if !isPresentAndFalseWithReason(capiMachineKey, capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason) {
					return fmt.Errorf("MachineOwnerRemediatedCondition not set")
				}
				return nil
			}, timeout).Should(Succeed())
		})

		It("checks that, under normal conditions, a reboot is carried out and retryCount and lastRemediated are set", func() {
			// Wait until machine has a ProviderID
			Eventually(func() error {
				err := testEnv.Client.Get(ctx, hcloudMachineKey, hcloudMachine)
				if err != nil {
					return err
				}
				if hcloudMachine.Spec.ProviderID == nil {
					return fmt.Errorf("hcloudMachine.Spec.ProviderID is still nil")
				}
				return nil
			}).NotTo(HaveOccurred())

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
			}, timeout).ShouldNot(HaveOccurred())
		})

		It("checks if PhaseWaiting is set when retryLimit is reached", func() {
			// Wait until machine has a ProviderID
			Eventually(func() error {
				err := testEnv.Client.Get(ctx, hcloudMachineKey, hcloudMachine)
				if err != nil {
					return err
				}
				if hcloudMachine.Spec.ProviderID == nil {
					return fmt.Errorf("hcloudMachine.Spec.ProviderID is still nil")
				}
				return nil
			}).NotTo(HaveOccurred())

			hcloudRemediation.Status.RetryCount = hcloudRemediation.Spec.Strategy.RetryLimit
			Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())

			Eventually(func() error {
				if err := testEnv.Get(ctx, hcloudRemediationkey, hcloudRemediation); err != nil {
					return err
				}
				if hcloudRemediation.Status.Phase != infrav1.PhaseWaiting {
					return fmt.Errorf("hcloudRemediation.Status.Phase != infrav1.PhaseWaiting (phase is %s)", hcloudRemediation.Status.Phase)
				}
				return nil
			}, timeout).ShouldNot(HaveOccurred())
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
		It("should delete machine if SetErrorAndRemediate() was called", func() {
			By("Checking Environment")
			capiMachineAgain, err := util.GetOwnerMachine(ctx, testEnv, hcloudMachine.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(capiMachineAgain).ToNot(BeNil())
			Expect(capiMachine.UID).To(Equal(capiMachineAgain.UID))
			hcloudClient := testEnv.HCloudClientFactory.NewClient("dummy-token")

			server, err := hcloudClient.CreateServer(ctx, hcloud.ServerCreateOpts{
				Name: "myserver",
			})
			Expect(err).ShouldNot(HaveOccurred())
			providerID := hcloudutil.ProviderIDFromServerID(int(server.ID))

			Eventually(func() error {
				err := testEnv.Get(ctx, client.ObjectKeyFromObject(hcloudMachine), hcloudMachine)
				if err != nil {
					return err
				}
				hcloudMachine.Spec.ProviderID = &providerID
				err = testEnv.Update(ctx, hcloudMachine)
				if err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			By("Call SetErrorAndRemediateMachine")
			Eventually(func() error {
				err = testEnv.Get(ctx, client.ObjectKeyFromObject(hcloudMachine), hcloudMachine)
				if err != nil {
					return err
				}
				err = scope.SetErrorAndRemediateMachine(ctx, testEnv, capiMachine, hcloudMachine, "test-of-set-error-and-remediate")
				if err != nil {
					return err
				}
				err = testEnv.Status().Update(ctx, hcloudMachine)
				if err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			By("Wait until hcloud has condition set.")
			Eventually(func() error {
				err := testEnv.Get(ctx, client.ObjectKeyFromObject(hcloudMachine), hcloudMachine)
				if err != nil {
					return err
				}
				c := conditions.Get(hcloudMachine, infrav1.DeleteMachineSucceededCondition)
				if c == nil {
					return fmt.Errorf("not set: DeleteMachineSucceededCondition")
				}
				if c.Status != corev1.ConditionFalse {
					return fmt.Errorf("status not set yet")
				}
				return nil
			}).Should(Succeed())

			By("Do the job of CAPI: Create a HCloudRemediation")
			rem := &infrav1.HCloudRemediation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hcloudMachine.Name,
					Namespace: hcloudMachine.Namespace,
				},
				Spec: infrav1.HCloudRemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       infrav1.RemediationTypeReboot,
						RetryLimit: 5,
						Timeout: &metav1.Duration{
							Duration: time.Minute,
						},
					},
				},
			}

			err = controllerutil.SetOwnerReference(capiMachine, rem, testEnv.GetScheme())
			Expect(err).Should(Succeed())

			err = testEnv.Create(ctx, rem)
			Expect(err).ShouldNot(HaveOccurred())

			By("Wait until remediation is done")
			Eventually(func() error {
				err := testEnv.Get(ctx, client.ObjectKeyFromObject(capiMachine), capiMachine)
				if err != nil {
					return err
				}

				c := conditions.Get(capiMachine, clusterv1.MachineOwnerRemediatedCondition)
				if c == nil {
					return fmt.Errorf("not set: MachineOwnerRemediatedCondition")
				}
				if c.Status != corev1.ConditionFalse {
					return fmt.Errorf("status not set yet")
				}
				if c.Message != "Remediation finished (machine will be deleted): exit remediation because infra machine has condition set: DeleteMachineInProgress: test-of-set-error-and-remediate" {
					return fmt.Errorf("Message is not as expected: %q", c.Message)
				}
				return nil
			}).Should(Succeed())
		})
	})
})
