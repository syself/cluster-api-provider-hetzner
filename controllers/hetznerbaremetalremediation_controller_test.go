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
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

var _ = Describe("HetznerBareMetalRemediationReconciler", func() {
	var (
		host                        *infrav1.HetznerBareMetalHost
		hetznerBareMetalRemediation *infrav1.HetznerBareMetalRemediation
		hetznerBaremetalMachine     *infrav1.HetznerBareMetalMachine
		hetznerCluster              *infrav1.HetznerCluster

		capiMachine *clusterv1.Machine
		capiCluster *clusterv1.Cluster

		hetznerSecret   *corev1.Secret
		rescueSSHSecret *corev1.Secret
		osSSHSecret     *corev1.Secret
		bootstrapSecret *corev1.Secret

		testNs *corev1.Namespace

		hetznerBaremetalRemediationkey client.ObjectKey
		hostKey                        client.ObjectKey
		capiMachineKey                 client.ObjectKey

		robotClient                  *robotmock.Client
		rescueSSHClient              *sshmock.Client
		osSSHClientAfterInstallImage *sshmock.Client
		osSSHClientAfterCloudInit    *sshmock.Client
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
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

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
					Kind:       "HetznerBareMetalMachine",
					Name:       bmMachineName,
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

		hetznerBaremetalMachine = &infrav1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bmMachineName,
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
			Spec: getDefaultHetznerBareMetalMachineSpec(),
		}

		hetznerBareMetalRemediation = &infrav1.HetznerBareMetalRemediation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hetzner-baremetal-remediation",
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
			Spec: infrav1.HetznerBareMetalRemediationSpec{
				Strategy: &infrav1.RemediationStrategy{
					Type:       "Reboot",
					RetryLimit: 1,
					Timeout:    &metav1.Duration{Duration: 1 * time.Second},
				},
			},
		}

		hetznerBaremetalRemediationkey = client.ObjectKey{Namespace: testNs.Name, Name: "hetzner-baremetal-remediation"}

		robotClient = testEnv.RobotClient
		rescueSSHClient = testEnv.RescueSSHClient
		osSSHClientAfterInstallImage = testEnv.OSSSHClientAfterInstallImage
		osSSHClientAfterCloudInit = testEnv.OSSSHClientAfterCloudInit

		robotClient.On("GetBMServer", mock.Anything).Return(&models.Server{
			ServerNumber: 1,
			ServerIP:     "1.2.3.4",
			Rescue:       true,
		}, nil)
		robotClient.On("ListSSHKeys").Return([]models.Key{
			{
				Name:        "my-name",
				Fingerprint: "my-fingerprint",
				Data:        "my-public-key",
			},
		}, nil)
		robotClient.On("GetReboot", mock.Anything).Return(&models.Reset{Type: []string{"hw", "sw"}}, nil)
		robotClient.On("GetBootRescue", 1).Return(&models.Rescue{Active: true}, nil)
		robotClient.On("SetBootRescue", mock.Anything, mock.Anything).Return(&models.Rescue{Active: true}, nil)
		robotClient.On("DeleteBootRescue", mock.Anything).Return(&models.Rescue{Active: false}, nil)
		robotClient.On("RebootBMServer", mock.Anything, mock.Anything).Return(&models.ResetPost{}, nil)
		robotClient.On("SetBMServerName", mock.Anything, mock.Anything).Return(nil, nil)

		configureRescueSSHClient(rescueSSHClient)

		osSSHClientAfterInstallImage.On("Reboot").Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("CreateNoCloudDirectory").Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("CreateMetaData", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("CreateUserData", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("EnsureCloudInit").Return(sshclient.Output{StdOut: "cloud-init"})
		osSSHClientAfterInstallImage.On("CloudInitStatus").Return(sshclient.Output{StdOut: "status: done"})
		osSSHClientAfterInstallImage.On("CheckCloudInitLogsForSigTerm").Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("ResetKubeadm").Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("GetCloudInitOutput").Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})
		osSSHClientAfterInstallImage.On("GetHostName").Return(sshclient.Output{
			StdOut: infrav1.BareMetalHostNamePrefix + bmMachineName,
			StdErr: "",
			Err:    nil,
		})
		osSSHClientAfterCloudInit.On("Reboot").Return(sshclient.Output{})
		osSSHClientAfterCloudInit.On("GetHostName").Return(sshclient.Output{
			StdOut: infrav1.BareMetalHostNamePrefix + bmMachineName,
			StdErr: "",
			Err:    nil,
		})
		osSSHClientAfterCloudInit.On("CloudInitStatus").Return(sshclient.Output{StdOut: "status: done"})
		osSSHClientAfterCloudInit.On("CheckCloudInitLogsForSigTerm").Return(sshclient.Output{})
		osSSHClientAfterCloudInit.On("ResetKubeadm").Return(sshclient.Output{})
		osSSHClientAfterCloudInit.On("GetCloudInitOutput").Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, capiMachine, hetznerCluster)).To(Succeed())
	})

	Context("Basic test", func() {
		Context("HetznerBareMetalHost will get provisioned", func() {
			BeforeEach(func() {
				hetznerSecret = getDefaultHetznerSecret(testNs.Name)
				Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

				rescueSSHSecret = helpers.GetDefaultSSHSecret("rescue-ssh-secret", testNs.Name)
				Expect(testEnv.Create(ctx, rescueSSHSecret)).To(Succeed())

				osSSHSecret = helpers.GetDefaultSSHSecret("os-ssh-secret", testNs.Name)
				Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())

				bootstrapSecret = getDefaultBootstrapSecret(testNs.Name)
				Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, testNs, hetznerSecret, osSSHSecret, rescueSSHSecret, bootstrapSecret)).To(Succeed())
			})

			Context("HetznerBaremetalHost doesn't exist", func() {
				AfterEach(func() {
					Expect(testEnv.Cleanup(ctx, hetznerBareMetalRemediation, hetznerBaremetalMachine)).To(Succeed())
				})

				It("should not remediate if no annotations is present in the hetznerBaremetalMachine", func() {
					Expect(testEnv.Create(ctx, hetznerBaremetalMachine)).To(Succeed())
					Expect(testEnv.Create(ctx, hetznerBareMetalRemediation)).To(Succeed())

					Eventually(func() bool {
						if err := testEnv.Get(ctx, capiMachineKey, capiMachine); err != nil {
							return false
						}

						return isPresentAndFalseWithReason(capiMachineKey, capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason)
					}, timeout).Should(BeTrue())
				})

				It("should not remediate if HetznerBareMetalHost does not exist anymore", func() {
					hetznerBaremetalMachine.Annotations = map[string]string{
						infrav1.HostAnnotation: fmt.Sprintf("%s/%s", testNs.Name, hostName),
					}
					Expect(testEnv.Create(ctx, hetznerBaremetalMachine)).To(Succeed())
					Expect(testEnv.Create(ctx, hetznerBareMetalRemediation)).To(Succeed())

					Eventually(func() bool {
						if err := testEnv.Get(ctx, capiMachineKey, capiMachine); err != nil {
							return false
						}

						return isPresentAndFalseWithReason(capiMachineKey, capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason)
					}, timeout).Should(BeTrue())
				})
			})

			Context("HetznerBaremetalHost exist", func() {
				BeforeEach(func() {
					hostKey = client.ObjectKey{Name: hostName, Namespace: testNs.Name}

					By("creating HetznerBareMetalHost")
					host = helpers.BareMetalHost(
						hostName,
						testNs.Name,
						helpers.WithRootDeviceHintRaid(),
						helpers.WithHetznerClusterRef(hetznerCluster.Name),
					)
					Expect(testEnv.Create(ctx, host)).To(Succeed())

					By("creating hetznerBaremetalMachine")
					hetznerBaremetalMachine.Annotations = map[string]string{
						infrav1.HostAnnotation: fmt.Sprintf("%s/%s", testNs.Name, hostName),
					}
					Expect(testEnv.Create(ctx, hetznerBaremetalMachine)).To(Succeed())

					By("ensuring host is provisioned")
					Eventually(func() bool {
						if err := testEnv.Get(ctx, hostKey, host); err != nil {
							return false
						}

						testEnv.GetLogger().Info("Provisioning state of host", "state", host.Spec.Status.ProvisioningState, "conditions", host.Spec.Status.Conditions)
						return host.Spec.Status.ProvisioningState == infrav1.StateProvisioned
					}, timeout).Should(BeTrue())
				})

				AfterEach(func() {
					Expect(testEnv.Cleanup(ctx, host, hetznerBareMetalRemediation, hetznerBaremetalMachine)).To(Succeed())
				})

				It("should create hetznerBareMetalRemediation object successfully", func() {
					Expect(testEnv.Create(ctx, hetznerBareMetalRemediation)).To(Succeed())

					Eventually(func() error {
						return testEnv.Get(ctx, hetznerBaremetalRemediationkey, hetznerBareMetalRemediation)
					}, timeout).Should(BeNil())
				})

				It("checks that, under normal conditions, reboot annotation, retryCount and lastRemediated are set", func() {
					By("creating hetznerBareMetalRemediation object")
					Expect(testEnv.Create(ctx, hetznerBareMetalRemediation)).To(Succeed())

					By("checking if host remediation occurred")
					Eventually(func() bool {
						if err := testEnv.Get(ctx, hetznerBaremetalRemediationkey, hetznerBareMetalRemediation); err != nil {
							return false
						}

						if err := testEnv.Get(ctx, hostKey, host); err != nil {
							return false
						}

						rebootAnnotationArguments := infrav1.RebootAnnotationArguments{Type: infrav1.RebootTypeHardware}

						b, err := json.Marshal(rebootAnnotationArguments)
						Expect(err).NotTo(HaveOccurred())

						val, ok := host.Annotations[infrav1.RebootAnnotation]
						if !ok {
							return false
						}

						testEnv.GetLogger().Info("host annotations", "hostAnnotation", host.Annotations)
						return hetznerBareMetalRemediation.Status.LastRemediated != nil && hetznerBareMetalRemediation.Status.RetryCount == 1 && val == string(b)
					}, timeout).Should(BeTrue())
				})

				It("should delete machine if retry limit reached and reboot timed out", func() {
					By("creating hetznerBareMetalRemediation object")
					Expect(testEnv.Create(ctx, hetznerBareMetalRemediation)).To(Succeed())

					By("updating the status to waiting and setting the last remediation to past")
					hetznerBaremetalRemediationPatchHelper, err := patch.NewHelper(hetznerBareMetalRemediation, testEnv.GetClient())
					Expect(err).NotTo(HaveOccurred())

					hetznerBareMetalRemediation.Status.Phase = infrav1.PhaseWaiting
					hetznerBareMetalRemediation.Status.LastRemediated = &metav1.Time{Time: time.Now().Add(-2 * time.Second)}

					Expect(hetznerBaremetalRemediationPatchHelper.Patch(ctx, hetznerBareMetalRemediation)).NotTo(HaveOccurred())

					By("checking if hcloudRemediation is in deleting phase and capiMachine has MachineOwnerRemediatedCondition")
					Eventually(func() bool {
						if err := testEnv.Get(ctx, hetznerBaremetalRemediationkey, hetznerBareMetalRemediation); err != nil {
							return false
						}

						return hetznerBareMetalRemediation.Status.Phase == infrav1.PhaseDeleting &&
							isPresentAndFalseWithReason(capiMachineKey, capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason)
					}, timeout).Should(BeTrue())
				})
			})
		})

		Context("HetznerBareMetalHost will not get provisioned", func() {
			BeforeEach(func() {
				hostKey = client.ObjectKey{Name: hostName, Namespace: testNs.Name}

				By("creating HetznerBareMetalHost")
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
					helpers.WithRootDeviceHintRaid(),
					helpers.WithHetznerClusterRef(hetznerCluster.Name),
				)
				Expect(testEnv.Create(ctx, host)).To(Succeed())

				By("creating hetznerBaremetalMachine")
				hetznerBaremetalMachine.Annotations = map[string]string{
					infrav1.HostAnnotation: fmt.Sprintf("%s/%s", testNs.Name, hostName),
				}
				Expect(testEnv.Create(ctx, hetznerBaremetalMachine)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, host, hetznerBareMetalRemediation, hetznerBaremetalMachine)).To(Succeed())
			})

			It("should not try to remediate if HetznerBareMetalHost is not provisioned", func() {
				By("creating hetznerBareMetalRemediation object")
				Expect(testEnv.Create(ctx, hetznerBareMetalRemediation)).To(Succeed())

				By("checking if capiMachine has the MachineOwnerRemediatedCondition")
				Eventually(func() bool {
					if err := testEnv.Get(ctx, capiMachineKey, capiMachine); err != nil {
						return false
					}

					return isPresentAndFalseWithReason(capiMachineKey, capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason)
				}, timeout).Should(BeTrue())
			})
		})
	})
})
