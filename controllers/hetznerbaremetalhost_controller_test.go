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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hostpkg "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/host"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

const (
	bmMachineName = "bm-machine-host-testing"
	hostName      = "test-host"
)

func verifyError(host *infrav1.HetznerBareMetalHost, errorType infrav1.ErrorType, errorMessage string) bool {
	if host.Spec.Status.ErrorType != errorType {
		return false
	}
	if host.Spec.Status.ErrorMessage != errorMessage {
		return false
	}
	return true
}

var _ = Describe("HetznerBareMetalHostReconciler", func() {
	var (
		host           *infrav1.HetznerBareMetalHost
		bmMachine      *infrav1.HetznerBareMetalMachine
		hetznerCluster *infrav1.HetznerCluster

		capiCluster *clusterv1.Cluster
		capiMachine *clusterv1.Machine

		hetznerClusterName string

		testNs *corev1.Namespace

		hetznerSecret   *corev1.Secret
		rescueSSHSecret *corev1.Secret
		osSSHSecret     *corev1.Secret
		bootstrapSecret *corev1.Secret

		key client.ObjectKey

		robotClient                  *robotmock.Client
		rescueSSHClient              *sshmock.Client
		osSSHClientAfterInstallImage *sshmock.Client
		osSSHClientAfterCloudInit    *sshmock.Client
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "baremetalhost-reconciler")
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
				Labels: map[string]string{clusterv1.ClusterNameLabel: capiCluster.Name},
			},
			Spec: helpers.GetDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

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
		osSSHClientAfterInstallImage.On("CloudInitStatus").Return(sshclient.Output{StdOut: "status: done"})
		osSSHClientAfterInstallImage.On("CheckCloudInitLogsForSigTerm").Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("ResetKubeadm").Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("GetHostName").Return(sshclient.Output{
			StdOut: infrav1.BareMetalHostNamePrefix + bmMachineName,
			StdErr: "",
			Err:    nil,
		})
		osSSHClientAfterInstallImage.On("GetCloudInitOutput").Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})

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
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, hetznerCluster,
			hetznerSecret, rescueSSHSecret, osSSHSecret, bootstrapSecret)).To(Succeed())
	})

	Context("Basic test", func() {
		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithRootDeviceHintWWN(),
				helpers.WithHetznerClusterRef(hetznerClusterName),
			)
			Expect(testEnv.Create(ctx, host)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, host)).To(Succeed())
		})

		It("creates the host machine", func() {
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, host); err != nil {
					return false
				}
				return true
			}, timeout).Should(BeTrue())

			Expect(testEnv.Delete(ctx, host)).To(Succeed())
		})

		It("sets the finalizer", func() {
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, host); err != nil {
					return false
				}
				for _, finalizer := range host.GetFinalizers() {
					if finalizer == infrav1.BareMetalHostFinalizer {
						return true
					}
				}
				return false
			}, timeout).Should(BeTrue())

			Expect(testEnv.Delete(ctx, host)).To(Succeed())
		})

		It("deletes successfully", func() {
			By("deleting the host object")
			Expect(testEnv.Delete(ctx, host)).To(Succeed())

			By("making sure the it has been deleted")
			Eventually(func() bool {
				return apierrors.IsNotFound(testEnv.Get(ctx, key, host))
			}, timeout, time.Second).Should(BeTrue())
		})
	})

	Context("Tests with bm machine", func() {
		BeforeEach(func() {
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
						Kind:       "HetznerBareMetalMachine",
						Name:       bmMachineName,
					},
					FailureDomain: &defaultFailureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bmMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
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
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, capiMachine, bmMachine)).To(Succeed())
		})

		Context("Test root device life cycle", func() {
			BeforeEach(func() {
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
					helpers.WithHetznerClusterRef(hetznerClusterName),
				)
				Expect(testEnv.Create(ctx, host)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}
			})

			AfterEach(func() {
				Expect(testEnv.Delete(ctx, host)).To(Succeed())
			})

			It("gives an error if no root device hints are set", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, host); err != nil {
						return false
					}
					return verifyError(host, infrav1.RegistrationError, infrav1.ErrorMessageMissingRootDeviceHints)
				}, timeout).Should(BeTrue())
			})

			It("reaches the state image installing", func() {
				ph, err := patch.NewHelper(host, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				host.Spec.RootDeviceHints = &infrav1.RootDeviceHints{
					WWN: helpers.DefaultWWN,
				}

				Eventually(func() error {
					return ph.Patch(ctx, host, patch.WithStatusObservedGeneration{})
				}, timeout, time.Second).Should(BeNil())

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, host); err != nil {
						testEnv.GetLogger().Info("reaches the state image installing. Get failed", "err", err)
						return false
					}
					if host.Spec.Status.ProvisioningState == infrav1.StateImageInstalling || host.Spec.Status.ProvisioningState == infrav1.StateProvisioned {
						return true
					}
					testEnv.GetLogger().Info("reaches the state image installing. State",
						"is-state", host.Spec.Status.ProvisioningState,
						"should-state", infrav1.StateImageInstalling)
					return false
				}, 10*time.Second).Should(BeTrue())
			})
		})

		Context("provision host", func() {
			BeforeEach(func() {
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
					helpers.WithRootDeviceHintWWN(),
					helpers.WithHetznerClusterRef(hetznerClusterName),
				)
				Expect(testEnv.Create(ctx, host)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, host)).To(Succeed())
			})

			It("gets selected from a bm machine and provisions (context 'provision host')", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, host); err != nil {
						return false
					}
					if host.Spec.Status.ProvisioningState != infrav1.StateProvisioned {
						return false
					}

					return isPresentAndTrue(key, host, infrav1.ProvisionSucceededCondition)
				}, timeout).Should(BeTrue())
			})
		})

		Context("Test secret owner refs", func() {
			BeforeEach(func() {
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
					helpers.WithHetznerClusterRef(hetznerClusterName),
				)
				Expect(testEnv.Create(ctx, host)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, host)).To(Succeed())
			})

			It("should create an owner ref of the cluster in the rescue ssh secret", func() {
				Eventually(func() bool {
					var secret corev1.Secret
					secretKey := types.NamespacedName{Name: rescueSSHSecret.Name, Namespace: rescueSSHSecret.Namespace}
					if err := testEnv.Get(ctx, secretKey, &secret); err != nil {
						return false
					}

					for _, owner := range secret.GetOwnerReferences() {
						if owner.UID == hetznerCluster.GetUID() {
							return true
						}
					}

					return false
				}, timeout, time.Second).Should(BeTrue())
			})
		})
	})

	Context("Tests with bm machine (with RAID)", func() {
		BeforeEach(func() {
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
						Kind:       "HetznerBareMetalMachine",
						Name:       bmMachineName,
					},
					FailureDomain: &defaultFailureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bmMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
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
			bmMachine.Spec.InstallImage.Swraid = 1
			Expect(testEnv.Create(ctx, bmMachine)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, capiMachine, bmMachine)).To(Succeed())
		})

		Context("provision host with rootDeviceHints RAID", func() {
			BeforeEach(func() {
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
					helpers.WithRootDeviceHintRaid(),
					helpers.WithHetznerClusterRef(hetznerClusterName),
				)
				Expect(testEnv.Create(ctx, host)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, host)).To(Succeed())
			})

			It("gets selected from a bm machine and provisions (rootDeviceHints RAID)", func() {
				Expect(len(host.Spec.RootDeviceHints.Raid.WWN)).To(Equal(2))
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, host); err != nil {
						return false
					}
					if host.Spec.Status.ProvisioningState == infrav1.StateProvisioned {
						return true
					}
					return false
				}, timeout).Should(BeTrue())
			})
		})
	})

	Context("Tests with bm machine and different ports after installImage and cloudInit", func() {
		BeforeEach(func() {
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
						Kind:       "HetznerBareMetalMachine",
						Name:       bmMachineName,
					},
					FailureDomain: &defaultFailureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bmMachineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
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
			bmMachine.Spec.SSHSpec.PortAfterInstallImage = 23
			bmMachine.Spec.SSHSpec.PortAfterCloudInit = 24

			Expect(testEnv.Create(ctx, bmMachine)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, capiMachine, bmMachine)).To(Succeed())
		})

		Context("provision host", func() {
			BeforeEach(func() {
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
					helpers.WithRootDeviceHintWWN(),
					helpers.WithHetznerClusterRef(hetznerClusterName),
				)
				Expect(testEnv.Create(ctx, host)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, host)).To(Succeed())
			})

			It("gets selected from a bm machine and provisions (different ports after installImage)", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, host); err != nil {
						return false
					}
					if host.Spec.Status.ProvisioningState == infrav1.StateProvisioned {
						return true
					}
					return false
				}, timeout).Should(BeTrue())
			})
		})
	})
})

var _ = Describe("HetznerBareMetalHostReconciler - missing secrets", func() {
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

		key client.ObjectKey

		robotClient     *robotmock.Client
		rescueSSHClient *sshmock.Client
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
				Labels: map[string]string{clusterv1.ClusterNameLabel: capiCluster.Name},
			},
			Spec: helpers.GetDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

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
					Kind:       "HetznerBareMetalMachine",
					Name:       bmMachineName,
				},
				FailureDomain: &defaultFailureDomain,
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		bmMachine = &infrav1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bmMachineName,
				Namespace: testNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
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

		bootstrapSecret = getDefaultBootstrapSecret(testNs.Name)
		Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())

		robotClient = testEnv.RobotClient
		rescueSSHClient = testEnv.RescueSSHClient

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
		robotClient.On("SetBootRescue", mock.Anything, mock.Anything).Return(&models.Rescue{Active: true}, nil)
		robotClient.On("DeleteBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
		robotClient.On("RebootBMServer", mock.Anything, mock.Anything).Return(&models.ResetPost{}, nil)

		configureRescueSSHClient(rescueSSHClient)
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, hetznerCluster, capiMachine, bmMachine, bootstrapSecret)).To(Succeed())
	})

	Context("Test missing Rescue SSH secret", func() {
		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithHetznerClusterRef(hetznerClusterName),
			)
			Expect(testEnv.Create(ctx, host)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}

			hetznerSecret = getDefaultHetznerSecret(testNs.Name)
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			osSSHSecret = helpers.GetDefaultSSHSecret("os-ssh-secret", testNs.Name)
			Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, host, hetznerSecret, osSSHSecret)).To(Succeed())
		})

		It("gives an error", func() {
			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, host, infrav1.CredentialsAvailableCondition, infrav1.RescueSSHSecretMissingReason)
			}, timeout).Should(BeTrue())
		})

		It("gives the right error if secret if rescue-ssh is invalid", func() {
			rescueSSHSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rescue-ssh-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"private-key": []byte("rescue-ssh-secret-private-key"),
					"sshkey-name": []byte(""),
					"public-key":  []byte("my-public-key"),
				},
			}
			Expect(testEnv.Create(ctx, rescueSSHSecret)).To(Succeed())
			defer func() {
				Expect(testEnv.Delete(ctx, rescueSSHSecret)).To(Succeed())
			}()

			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, host, infrav1.CredentialsAvailableCondition, infrav1.SSHCredentialsInSecretInvalidReason)
			}, timeout).Should(BeTrue())
		})
	})

	Context("Test missing/wrong OS SSH secret", func() {
		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithHetznerClusterRef(hetznerClusterName),
				helpers.WithRootDeviceHintWWN(),
			)
			Expect(testEnv.Create(ctx, host)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}

			hetznerSecret = getDefaultHetznerSecret(testNs.Name)
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			rescueSSHSecret = helpers.GetDefaultSSHSecret("rescue-ssh-secret", testNs.Name)
			Expect(testEnv.Create(ctx, rescueSSHSecret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, host, hetznerSecret, rescueSSHSecret)).To(Succeed())
		})

		It("gives the right error if secret is missing", func() {
			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, host, infrav1.CredentialsAvailableCondition, infrav1.OSSSHSecretMissingReason)
			}, timeout).Should(BeTrue())
		})

		It("gives the right error if secret is invalid", func() {
			osSSHSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "os-ssh-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"private-key": []byte("os-ssh-secret-private-key"),
					"sshkey-name": []byte(""),
					"public-key":  []byte("my-public-key"),
				},
			}
			Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())
			defer func() {
				Expect(testEnv.Delete(ctx, osSSHSecret)).To(Succeed())
			}()

			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, host, infrav1.CredentialsAvailableCondition, infrav1.SSHCredentialsInSecretInvalidReason)
			}, timeout).Should(BeTrue())
		})
	})
})

func configureRescueSSHClient(sshClient *sshmock.Client) {
	sshClient.On("GetHostName").Return(sshclient.Output{
		StdOut: "rescue",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsRAM").Return(sshclient.Output{
		StdOut: "100000",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsStorage").Return(sshclient.Output{
		StdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsNics").Return(sshclient.Output{
		StdOut: `name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ipv4="23.88.6.239/26" speedMbps="1000"
name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ipv6="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUArch").Return(sshclient.Output{
		StdOut: "myarch",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUModel").Return(sshclient.Output{
		StdOut: "mymodel",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUClockGigahertz").Return(sshclient.Output{
		StdOut: "42654",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUFlags").Return(sshclient.Output{
		StdOut: "flag1 flag2 flag3",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUThreads").Return(sshclient.Output{
		StdOut: "123",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUCores").Return(sshclient.Output{
		StdOut: "12",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsDebug").Return(sshclient.Output{
		StdOut: "Dummy output",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("DownloadImage", mock.Anything, mock.Anything).Return(sshclient.Output{})
	sshClient.On("CreateAutoSetup", mock.Anything).Return(sshclient.Output{})
	sshClient.On("UntarTGZ").Return(sshclient.Output{})
	sshClient.On("CreatePostInstallScript", mock.Anything).Return(sshclient.Output{})
	sshClient.On("ExecuteInstallImage", mock.Anything).Return(sshclient.Output{StdOut: hostpkg.PostInstallScriptFinished})
	sshClient.On("Reboot").Return(sshclient.Output{})
	sshClient.On("GetCloudInitOutput").Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})
	sshClient.On("DetectLinuxOnAnotherDisk", mock.Anything).Return(sshclient.Output{})
	sshClient.On("GetRunningInstallImageProcesses").Return(sshclient.Output{})
}
