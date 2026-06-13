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
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hostpkg "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/host"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

const (
	hostName = "test-host"
)

type countingRobotFactory struct {
	calls int
}

func (f *countingRobotFactory) NewClient(robotclient.Credentials) robotclient.Client {
	f.calls++
	return nil
}

// TestHetznerBareMetalHostReconciler_ReconcileSkipsPausedCluster verifies that
// the reconciler returns early when the linked Cluster has Spec.Paused = true.
// The Robot client factory counts NewClient calls and the test asserts the
// count stays at zero, proving the pause guard fired before any host work ran.
func TestHetznerBareMetalHostReconciler_ReconcileSkipsPausedCluster(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(infrav1.AddToScheme(scheme))
	utilruntime.Must(infrav2.AddToScheme(scheme))

	namespace := "default"
	clusterName := "test-cluster"
	hetznerClusterName := "test-hetzner-cluster"

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: ptr.To(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrav1.GroupVersion.Group,
				Kind:     "HetznerCluster",
				Name:     hetznerClusterName,
			},
		},
	}
	hetznerCluster := &infrav1.HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hetznerClusterName,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: helpers.GetDefaultHetznerClusterSpec(),
	}
	host := helpers.BareMetalHost("paused-cluster-host", namespace, helpers.WithClusterNameLabel(clusterName))
	host.Finalizers = []string{infrav2.HetznerBareMetalHostFinalizer}
	host.Status.ProvisioningState = infrav2.StatePreparing
	hetznerSecret := getDefaultHetznerSecret(namespace)
	robotFactory := &countingRobotFactory{}

	c := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&infrav2.HetznerBareMetalHost{}).
		WithObjects(cluster, hetznerCluster, host, hetznerSecret).
		Build()

	reconciler := &HetznerBareMetalHostReconciler{
		Client:             c,
		APIReader:          c,
		RobotClientFactory: robotFactory,
	}

	result, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(host),
	})
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, result)

	updatedHost := &infrav2.HetznerBareMetalHost{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(host), updatedHost))
	require.Contains(t, updatedHost.Finalizers, infrav2.HetznerBareMetalHostFinalizer)
	require.Equal(t, infrav2.StatePreparing, updatedHost.Status.ProvisioningState)
	require.Zero(t, robotFactory.calls)
}

func verifyError(host *infrav2.HetznerBareMetalHost, errorType infrav2.OperationalState, errorMessage string) bool {
	if host.Status.OperationalState != errorType {
		return false
	}
	condition := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
	if condition == nil || condition.Message != errorMessage {
		return false
	}
	return true
}

var _ = Describe("HetznerBareMetalHostReconciler", func() {
	var (
		host           *infrav2.HetznerBareMetalHost
		bmMachine      *infrav1.HetznerBareMetalMachine
		machineName    string
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
		testNs, err = testEnv.ResetAndCreateNamespace(ctx, "baremetalhost-reconciler")
		Expect(err).NotTo(HaveOccurred())

		hetznerClusterName = utils.GenerateName(nil, "hetzner-cluster-test")

		machineName = utils.GenerateName(nil, "machine")

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "HetznerCluster",
					Name:     hetznerClusterName,
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
						APIVersion: clusterv1.GroupVersion.String(),
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

		osSSHClientAfterInstallImage.On("Reboot", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("CloudInitStatus", mock.Anything).Return(sshclient.Output{StdOut: "status: done"})
		osSSHClientAfterInstallImage.On("CheckCloudInitLogsForSigTerm", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("ResetKubeadm", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterInstallImage.On("GetHostName", mock.Anything).Return(sshclient.Output{
			StdOut: infrav1.BareMetalHostNamePrefix + machineName,
			StdErr: "",
			Err:    nil,
		})
		osSSHClientAfterInstallImage.On("GetCloudInitOutput", mock.Anything).Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})

		osSSHClientAfterCloudInit.On("Reboot", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterCloudInit.On("GetHostName", mock.Anything).Return(sshclient.Output{
			StdOut: infrav1.BareMetalHostNamePrefix + machineName,
			StdErr: "",
			Err:    nil,
		})
		osSSHClientAfterCloudInit.On("CloudInitStatus", mock.Anything).Return(sshclient.Output{StdOut: "status: done"})
		osSSHClientAfterCloudInit.On("CheckCloudInitLogsForSigTerm", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterCloudInit.On("ResetKubeadm", mock.Anything).Return(sshclient.Output{})
		osSSHClientAfterCloudInit.On("GetCloudInitOutput", mock.Anything).Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, hetznerCluster,
			hetznerSecret, rescueSSHSecret, osSSHSecret, bootstrapSecret)).To(Succeed())
	})

	Context("Basic hbmh test", func() {
		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithRootDeviceHintWWN(),
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
					if finalizer == infrav2.HetznerBareMetalHostFinalizer {
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
					Name:       machineName,
					Namespace:  testNs.Name,
					Finalizers: []string{clusterv1.MachineFinalizer},
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
					},
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: capiCluster.Name,
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "infrastructure.cluster.x-k8s.io",
						Kind:     "HetznerBareMetalMachine",
						Name:     machineName,
					},
					FailureDomain: defaultFailureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
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
					return verifyError(host, infrav2.OperationalStateRegistrationError, infrav2.ErrorMessageMissingRootDeviceHints)
				}, timeout).Should(BeTrue())
			})

			It("reaches the state image installing", func() {
				ph, err := patch.NewHelper(host, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				host.Spec.RootDeviceHints = &infrav2.RootDeviceHints{
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
					if host.Status.ProvisioningState == infrav2.StateImageInstalling || host.Status.ProvisioningState == infrav2.StateProvisioned {
						return true
					}
					testEnv.GetLogger().Info("reaches the state image installing. State",
						"is-state", host.Status.ProvisioningState,
						"should-state", infrav2.StateImageInstalling)
					return false
				}, 10*time.Second).Should(BeTrue())

				By("Checking if pre-provision-command was executed")
				rescueSSHClient.AssertCalled(GinkgoT(), "ExecutePreProvisionCommand", mock.Anything, mock.Anything)
			})
		})

		Context("provision host", func() {
			BeforeEach(func() {
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
					helpers.WithRootDeviceHintWWN(),
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
					if host.Status.ProvisioningState != infrav2.StateProvisioned {
						return false
					}

					return isPresentAndTrueWithReason(key, host, infrav2.HetznerBareMetalHostProvisionSucceededCondition, infrav2.HetznerBareMetalHostProvisionSucceededReason) &&
						isPresentAndTrueDeprecatedV1Beta1(key, host, infrav2.ProvisionSucceededV1Beta1Condition)
				}, timeout).Should(BeTrue())
			})
		})

		Context("Test secret owner refs", func() {
			BeforeEach(func() {
				host = helpers.BareMetalHost(
					hostName,
					testNs.Name,
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
					Name:       machineName,
					Namespace:  testNs.Name,
					Finalizers: []string{clusterv1.MachineFinalizer},
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
					},
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: capiCluster.Name,
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "infrastructure.cluster.x-k8s.io",
						Kind:     "HetznerBareMetalMachine",
						Name:     machineName,
					},
					FailureDomain: defaultFailureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
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
					if host.Status.ProvisioningState == infrav2.StateProvisioned {
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
					Name:       machineName,
					Namespace:  testNs.Name,
					Finalizers: []string{clusterv1.MachineFinalizer},
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
					},
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: capiCluster.Name,
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "infrastructure.cluster.x-k8s.io",
						Kind:     "HetznerBareMetalMachine",
						Name:     machineName,
					},
					FailureDomain: defaultFailureDomain,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("bootstrap-secret"),
					},
				},
			}
			Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

			bmMachine = &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineName,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
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
			bmMachine.Spec.SSHSpec.PortAfterInstallImage = 23

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
					if host.Status.ProvisioningState == infrav2.StateProvisioned {
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
		host           *infrav2.HetznerBareMetalHost
		bmMachine      *infrav1.HetznerBareMetalMachine
		machineName    string
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
		testNs, err = testEnv.ResetAndCreateNamespace(ctx, "baremetalmachine-reconciler")
		Expect(err).NotTo(HaveOccurred())

		hetznerClusterName = utils.GenerateName(nil, "hetzner-cluster-test")
		machineName = utils.GenerateName(nil, "machine")

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "HetznerCluster",
					Name:     hetznerClusterName,
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
						APIVersion: clusterv1.GroupVersion.String(),
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
				Name:       machineName,
				Namespace:  testNs.Name,
				Finalizers: []string{clusterv1.MachineFinalizer},
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: capiCluster.Name,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "HetznerBareMetalMachine",
					Name:     machineName,
				},
				FailureDomain: defaultFailureDomain,
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		bmMachine = &infrav1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: testNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
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
				return isPresentAndFalseWithReason(key, host, infrav2.HetznerBareMetalHostSSHKeysAvailableCondition, infrav2.HetznerBareMetalHostRescueSSHSecretMissingReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav2.CredentialsAvailableV1Beta1Condition, infrav2.RescueSSHSecretMissingV1Beta1Reason)
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
				return isPresentAndFalseWithReason(key, host, infrav2.HetznerBareMetalHostSSHKeysAvailableCondition, infrav2.HetznerBareMetalHostSSHKeysInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav2.CredentialsAvailableV1Beta1Condition, infrav2.SSHCredentialsInSecretInvalidV1Beta1Reason)
			}, timeout).Should(BeTrue())
		})
	})

	Context("Test missing/wrong OS SSH secret", func() {
		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
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
				return isPresentAndFalseWithReason(key, host, infrav2.HetznerBareMetalHostSSHKeysAvailableCondition, infrav2.HetznerBareMetalHostOSSSHSecretMissingReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav2.CredentialsAvailableV1Beta1Condition, infrav2.OSSSHSecretMissingV1Beta1Reason)
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
				return isPresentAndFalseWithReason(key, host, infrav2.HetznerBareMetalHostSSHKeysAvailableCondition, infrav2.HetznerBareMetalHostSSHKeysInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav2.CredentialsAvailableV1Beta1Condition, infrav2.SSHCredentialsInSecretInvalidV1Beta1Reason)
			}, timeout).Should(BeTrue())
		})
	})

	Context("Wrong Robot Credentials - unauthorized", func() {
		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithRootDeviceHintWWN(),
			)
			Expect(testEnv.Create(ctx, host)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}

			hetznerSecret = getDefaultHetznerSecret(testNs.Name)
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			rescueSSHSecret = helpers.GetDefaultSSHSecret("rescue-ssh-secret", testNs.Name)
			Expect(testEnv.Create(ctx, rescueSSHSecret)).To(Succeed())

			osSSHSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "os-ssh-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"private-key": []byte("os-ssh-secret-private-key"),
					"sshkey-name": []byte("ssh-key-name"),
					"public-key":  []byte("my-public-key"),
				},
			}
			Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())

			robotClient.ExpectedCalls = nil
			robotClient.On("GetBMServer", mock.Anything).Return(nil, models.Error{
				Code:    models.ErrorCodeUnauthorized,
				Message: "You are not authorized - wrong RobotCredentials",
			})
		})

		It("should set CredentialsAvailable condition to false if Robot API returned unauthorized", func() {
			By("making the Robot client return an unauthorized error")
			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, host, infrav2.HetznerBareMetalHostRobotCredentialsAvailableCondition, infrav2.HetznerBareMetalHostRobotCredentialsInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav2.RobotCredentialsAvailableV1Beta1Condition, infrav2.RobotCredentialsInvalidV1Beta1Reason)
			}, timeout).Should(BeTrue())

			Expect(robotClient.AssertExpectations(GinkgoT())).To(BeTrue())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, host, hetznerSecret, rescueSSHSecret)).To(Succeed())
		})
	})

	Context("Wrong Robot Credentials - missing username in secret", func() {
		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithRootDeviceHintWWN(),
			)
			Expect(testEnv.Create(ctx, host)).To(Succeed())

			hetznerSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hetzner-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"hcloud":         []byte("my-token"),
					"robot-user":     []byte(""),
					"robot-password": []byte("my-password"),
				},
			}
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			key = client.ObjectKey{Namespace: testNs.Name, Name: host.Name}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, host, hetznerSecret)).To(Succeed())
		})

		It("sets RobotCredentialsAvailable to false if robot-user is empty", func() {
			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, host, infrav2.HetznerBareMetalHostRobotCredentialsAvailableCondition, infrav2.HetznerBareMetalHostRobotCredentialsInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav2.RobotCredentialsAvailableV1Beta1Condition, infrav2.RobotCredentialsInvalidV1Beta1Reason)
			}, timeout).Should(BeTrue())
		})
	})
})

func configureRescueSSHClient(sshClient *sshmock.Client) {
	sshClient.On("GetHostName", mock.Anything).Return(sshclient.Output{
		StdOut: "rescue",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsRAM", mock.Anything).Return(sshclient.Output{
		StdOut: "100000",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsStorage", mock.Anything).Return(sshclient.Output{
		StdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsNics", mock.Anything).Return(sshclient.Output{
		StdOut: `name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ip="23.88.6.239/26" speedMbps="1000"
name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ip="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUArch", mock.Anything).Return(sshclient.Output{
		StdOut: "myarch",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUModel", mock.Anything).Return(sshclient.Output{
		StdOut: "mymodel",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUClockGigahertz", mock.Anything).Return(sshclient.Output{
		StdOut: "42654",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUFlags", mock.Anything).Return(sshclient.Output{
		StdOut: "flag1 flag2 flag3",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUThreads", mock.Anything).Return(sshclient.Output{
		StdOut: "123",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsCPUCores", mock.Anything).Return(sshclient.Output{
		StdOut: "12",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("GetHardwareDetailsDebug", mock.Anything).Return(sshclient.Output{
		StdOut: "Dummy output",
		StdErr: "",
		Err:    nil,
	})
	sshClient.On("DownloadImage", mock.Anything, mock.Anything).Return(sshclient.Output{})
	sshClient.On("CreateAutoSetup", mock.Anything).Return(sshclient.Output{})
	sshClient.On("UntarTGZ", mock.Anything).Return(sshclient.Output{})
	sshClient.On("CreatePostInstallScript", mock.Anything).Return(sshclient.Output{})
	sshClient.On("ExecuteInstallImage", mock.Anything).Return(sshclient.Output{StdOut: hostpkg.PostInstallScriptFinished})
	sshClient.On("Reboot", mock.Anything).Return(sshclient.Output{})
	sshClient.On("GetCloudInitOutput", mock.Anything).Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})
	sshClient.On("DetectLinuxOnAnotherDisk", mock.Anything).Return(sshclient.Output{})
	sshClient.On("ExecutePreProvisionCommand", mock.Anything, mock.Anything).Return(0, "", nil)
	sshClient.On("GetInstallImageState", mock.Anything).Return(sshclient.InstallImageStateFinished, nil)
	sshClient.On("GetResultOfInstallImage", mock.Anything).Return(hostpkg.PostInstallScriptFinished, nil)
}

func Test_removePermanentErrorIfAnnotationIsGone(t *testing.T) {
	newHostWithError := func(annotations map[string]string, errorType infrav2.OperationalState) infrav2.HetznerBareMetalHost {
		bmHost := infrav2.HetznerBareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: annotations,
			},
			Status: infrav2.HetznerBareMetalHostStatus{
				OperationalState: errorType,
			},
		}
		conditions.Set(&bmHost, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostActionCompletedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostPermanentErrorReason,
			Message: "my err",
		})
		return bmHost
	}

	// OperationalStatePermanentError with annotation --> Error should not get removed
	bmHost := newHostWithError(map[string]string{infrav2.PermanentErrorAnnotation: ""}, infrav2.OperationalStatePermanentError)
	removed := removePermanentErrorIfAnnotationIsGone(&bmHost)
	require.False(t, removed)
	require.NotEmpty(t, bmHost.Status.OperationalState)
	require.NotNil(t, conditions.Get(&bmHost, infrav2.HetznerBareMetalHostActionCompletedCondition))

	// OperationalStatePermanentError without annotation --> Error should get removed
	bmHost = newHostWithError(map[string]string{"other-annotation": "some value"}, infrav2.OperationalStatePermanentError)
	removed = removePermanentErrorIfAnnotationIsGone(&bmHost)
	require.True(t, removed)
	require.Empty(t, bmHost.Status.OperationalState)
	require.Nil(t, conditions.Get(&bmHost, infrav2.HetznerBareMetalHostActionCompletedCondition))
	require.Equal(t, map[string]string{"other-annotation": "some value"}, bmHost.Annotations)

	// Other Error without annotation --> Error should not get removed
	bmHost = newHostWithError(map[string]string{}, infrav2.OperationalStateProvisioningError)
	removed = removePermanentErrorIfAnnotationIsGone(&bmHost)
	require.False(t, removed)
	require.NotEmpty(t, bmHost.Status.OperationalState)
	require.NotNil(t, conditions.Get(&bmHost, infrav2.HetznerBareMetalHostActionCompletedCondition))
}

// Test_needsProvisioning covers the provision trigger: the host starts provisioning when its
// machine exists, is not being deleted, and the bootstrap data of the owning CAPI Machine is
// available. The machine used to trigger this by copying its installImage onto the host.
func Test_needsProvisioning(t *testing.T) {
	newMachines := func() (*infrav1.HetznerBareMetalMachine, *clusterv1.Machine) {
		hbmm := &infrav1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bm-machine",
				Namespace: "default",
			},
		}
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: "default",
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}
		return hbmm, machine
	}

	// machine with bootstrap data --> provision
	hbmm, machine := newMachines()
	require.True(t, needsProvisioning(hbmm, machine))

	// no consuming machine --> do not provision
	require.False(t, needsProvisioning(nil, machine))

	// machine is being deleted --> do not provision
	hbmm, machine = newMachines()
	now := metav1.Now()
	hbmm.DeletionTimestamp = &now
	require.False(t, needsProvisioning(hbmm, machine))

	// no owning CAPI machine (e.g. force deleted) --> do not provision
	hbmm, _ = newMachines()
	require.False(t, needsProvisioning(hbmm, nil))

	// owning CAPI machine is being deleted --> do not provision
	hbmm, machine = newMachines()
	machine.DeletionTimestamp = &now
	require.False(t, needsProvisioning(hbmm, machine))

	// bootstrap data not ready --> do not provision
	hbmm, machine = newMachines()
	machine.Spec.Bootstrap.DataSecretName = nil
	require.False(t, needsProvisioning(hbmm, machine))
}

// Test_hetznerBareMetalMachineToHetznerBareMetalHost covers the mapper of the watch on
// HetznerBareMetalMachine. The host both starts provisioning and deprovisions based on its
// machine, so machine events must enqueue the bound host.
func Test_hetznerBareMetalMachineToHetznerBareMetalHost(t *testing.T) {
	ctx := context.Background()

	// machine without host annotation --> no request
	hbmm := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bm-machine",
			Namespace: "test-ns",
		},
	}
	require.Nil(t, hetznerBareMetalMachineToHetznerBareMetalHost(ctx, hbmm))

	// machine with host annotation --> request for the bound host
	hbmm.Annotations = map[string]string{
		infrav2.HostAnnotation: "test-ns/test-host",
	}
	require.Equal(t, []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: "test-ns",
				Name:      "test-host",
			},
		},
	}, hetznerBareMetalMachineToHetznerBareMetalHost(ctx, hbmm))

	// malformed annotation --> no request
	hbmm.Annotations[infrav2.HostAnnotation] = "test-host"
	require.Nil(t, hetznerBareMetalMachineToHetznerBareMetalHost(ctx, hbmm))

	// other object type --> no request
	require.Nil(t, hetznerBareMetalMachineToHetznerBareMetalHost(ctx, &infrav2.HetznerBareMetalHost{}))
}
