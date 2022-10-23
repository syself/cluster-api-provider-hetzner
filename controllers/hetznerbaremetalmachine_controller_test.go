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
	"github.com/stretchr/testify/mock"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/baremetal"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("HetznerBareMetalMachineReconciler", func() {
	var (
		host           *infrav1.HetznerBareMetalHost
		bmMachine      *infrav1.HetznerBareMetalMachine
		hetznerCluster *infrav1.HetznerCluster
		capiCluster    *clusterv1.Cluster
		capiMachine    *clusterv1.Machine

		hetznerClusterName string

		testNs *corev1.Namespace

		hetznerSecret   *corev1.Secret
		rescueSSHSecret *corev1.Secret
		osSSHSecret     *corev1.Secret
		bootstrapSecret *corev1.Secret

		key     client.ObjectKey
		hostKey client.ObjectKey

		robotClient     *robotmock.Client
		rescueSSHClient *sshmock.Client
		osSSHClient     *sshmock.Client
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "baremetalmachine-reconciler")
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
			Status: clusterv1.ClusterStatus{
				InfrastructureReady: true,
				Conditions: clusterv1.Conditions{
					{
						Reason:  "reason",
						Message: "message",
					},
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
			Spec: helpers.GetDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

		host = helpers.BareMetalHost(
			hostName,
			testNs.Name,
			helpers.WithRootDeviceHintWWN(),
			helpers.WithHetznerClusterRef(hetznerClusterName),
		)
		Expect(testEnv.Create(ctx, host)).To(Succeed())
		hostKey = client.ObjectKey{Namespace: testNs.Name, Name: hostName}

		hetznerSecret = getDefaultHetznerSecret(testNs.Name)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		rescueSSHSecret = helpers.GetDefaultSSHSecret("rescue-ssh-secret", testNs.Name)
		Expect(testEnv.Create(ctx, rescueSSHSecret)).To(Succeed())

		osSSHSecret = helpers.GetDefaultSSHSecret("os-ssh-secret", testNs.Name)
		Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())

		bootstrapSecret = getDefaultBootstrapSecret(testNs.Name)
		Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())

		robotClient = testEnv.RobotClient
		rescueSSHClient = testEnv.RescueSSHClient
		osSSHClient = testEnv.OSSSHClientAfterInstallImage

		robotClient.On("GetBMServer", 1).Return(&models.Server{
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
		robotClient.On("GetReboot", 1).Return(&models.Reset{Type: []string{"hw", "sw"}}, nil)
		robotClient.On("GetBootRescue", 1).Return(&models.Rescue{Active: true}, nil)
		robotClient.On("SetBootRescue", 1, mock.Anything).Return(&models.Rescue{Active: true}, nil)
		robotClient.On("DeleteBootRescue", 1).Return(&models.Rescue{Active: false}, nil)
		robotClient.On("RebootBMServer", mock.Anything, mock.Anything).Return(&models.ResetPost{}, nil)
		robotClient.On("SetBMServerName", 1, mock.Anything).Return(nil, nil)

		configureRescueSSHClient(rescueSSHClient)

		osSSHClient.On("Reboot").Return(sshclient.Output{})
		osSSHClient.On("CreateNoCloudDirectory").Return(sshclient.Output{})
		osSSHClient.On("CreateMetaData", mock.Anything).Return(sshclient.Output{})
		osSSHClient.On("CreateUserData", mock.Anything).Return(sshclient.Output{})
		osSSHClient.On("EnsureCloudInit").Return(sshclient.Output{StdOut: "cloud-init"})
		osSSHClient.On("CloudInitStatus").Return(sshclient.Output{StdOut: "status: done"})
		osSSHClient.On("CheckCloudInitLogsForSigTerm").Return(sshclient.Output{})
		osSSHClient.On("ResetKubeadm").Return(sshclient.Output{})
		osSSHClient.On("GetHostName").Return(sshclient.Output{
			StdOut: infrav1.BareMetalHostNamePrefix + bmMachineName,
			StdErr: "",
			Err:    nil})
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, hetznerCluster, host,
			hetznerSecret, rescueSSHSecret, osSSHSecret, bootstrapSecret)).To(Succeed())
	})

	Context("Basic test", func() {
		BeforeEach(func() {
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
						Kind:       "HetznerBareMetalMachine",
						Name:       bmMachineName,
					},
					FailureDomain: &defaultFailureDomain,
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bmMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterLabelName: capiCluster.Name,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Machine",
							Name:       capiMachine.Name,
							UID:        capiMachine.UID,
						},
					},
				},
				Spec: getDefaultHetznerBareMetalMachineSpec(),
			}
			Expect(testEnv.Create(ctx, bmMachine)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: bmMachineName}
		})

		It("creates the bare metal machine and provisions host", func() {
			// Check whether bootstrap condition is not ready
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}
				return isPresentAndFalseWithReason(key, bmMachine, infrav1.InstanceBootstrapReadyCondition, infrav1.InstanceBootstrapNotReadyReason)
			}, timeout, time.Second).Should(BeTrue())

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
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}
				return isPresentAndTrue(key, bmMachine, infrav1.InstanceBootstrapReadyCondition)
			}, timeout, time.Second).Should(BeTrue())

			defer func() {
				Expect(testEnv.Delete(ctx, bmMachine)).To(Succeed())
			}()
			Eventually(func() bool {
				if err := testEnv.Get(ctx, hostKey, host); err != nil {
					return false
				}
				if host.Spec.Status.ProvisioningState == infrav1.StateProvisioned {
					return true
				}
				return false
			}, timeout).Should(BeTrue())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}
				return bmMachine.Status.Ready
			}, timeout).Should(BeTrue())
		})
	})

	Context("Basic test", func() {
		BeforeEach(func() {
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
						Kind:       "HetznerBareMetalMachine",
						Name:       bmMachineName,
					},
					FailureDomain: &defaultFailureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.String("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bmMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterLabelName: capiCluster.Name,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Machine",
							Name:       capiMachine.Name,
							UID:        capiMachine.UID,
						},
					},
				},
				Spec: getDefaultHetznerBareMetalMachineSpec(),
			}
			Expect(testEnv.Create(ctx, bmMachine)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: bmMachineName}
		})

		It("creates the bare metal machine", func() {
			defer func() {
				Expect(testEnv.Delete(ctx, bmMachine)).To(Succeed())
			}()
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}
				return true
			}, timeout).Should(BeTrue())
		})

		It("sets the finalizer", func() {
			defer func() {
				Expect(testEnv.Delete(ctx, bmMachine)).To(Succeed())
			}()
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}
				for _, finalizer := range bmMachine.GetFinalizers() {
					if finalizer == infrav1.BareMetalMachineFinalizer {
						return true
					}
				}
				return false
			}, timeout).Should(BeTrue())
		})

		It("reaches state ready and provisions the host", func() {
			defer func() {
				Expect(testEnv.Delete(ctx, bmMachine)).To(Succeed())
			}()
			Eventually(func() bool {
				if err := testEnv.Get(ctx, hostKey, host); err != nil {
					return false
				}
				if host.Spec.Status.ProvisioningState == infrav1.StateProvisioned {
					return true
				}
				return false
			}, timeout).Should(BeTrue())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}
				return bmMachine.Status.Ready
			}, timeout).Should(BeTrue())
		})

		It("deletes successfully", func() {
			// Delete the bmMachine object
			Expect(testEnv.Delete(ctx, bmMachine)).To(Succeed())

			// Make sure the it has been deleted
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return true
				}
				return false
			}, timeout, time.Second).Should(BeTrue())

			// Make sure the host has been deprovisioned.
			Eventually(func() bool {
				if err := testEnv.Get(ctx, hostKey, host); err != nil {
					return false
				}
				return host.Spec.Status.ProvisioningState == infrav1.StateNone
			}, timeout, time.Second).Should(BeTrue())
		})

		It("sets a failure reason when maintenance mode is set on the host", func() {
			defer func() {
				Expect(testEnv.Delete(ctx, bmMachine)).To(Succeed())
			}()

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					fmt.Println("failed to get bm machine. ", err)
					return false
				}
				return bmMachine.Status.Ready
			}, timeout, time.Second).Should(BeTrue())

			By("setting maintenance mode")

			ph, err := patch.NewHelper(host, testEnv)
			Expect(err).ShouldNot(HaveOccurred())
			host.Spec.MaintenanceMode = true
			Expect(ph.Patch(ctx, host, patch.WithStatusObservedGeneration{})).To(Succeed())

			// It sets a failure reason
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}
				return bmMachine.Status.FailureMessage != nil && *bmMachine.Status.FailureMessage == baremetal.FailureMessageMaintenanceMode
			}, timeout).Should(BeTrue())
		})
	})
})
