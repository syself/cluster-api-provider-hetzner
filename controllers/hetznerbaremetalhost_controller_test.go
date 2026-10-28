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
	"errors"
	"sync"
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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	bmclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"
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

type robotFactoryFunc func(robotclient.Credentials) robotclient.Client

func (f robotFactoryFunc) NewClient(credentials robotclient.Credentials) robotclient.Client {
	return f(credentials)
}

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
	host.Finalizers = []string{infrav1.HetznerBareMetalHostFinalizer}
	host.Status.ProvisioningState = infrav1.StatePreparing
	hetznerSecret := getDefaultHetznerSecret(namespace)
	robotFactory := &countingRobotFactory{}

	c := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&infrav1.HetznerBareMetalHost{}).
		WithObjects(cluster, hetznerCluster, host, hetznerSecret).
		Build()

	reconciler := &HetznerBareMetalHostReconciler{
		Client:             c,
		APIReader:          c,
		RobotClientFactory: robotFactory,
		EventRecorder:      record.NewFakeRecorder(100),
	}

	result, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(host),
	})
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, result)

	updatedHost := &infrav1.HetznerBareMetalHost{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(host), updatedHost))
	require.Contains(t, updatedHost.Finalizers, infrav1.HetznerBareMetalHostFinalizer)
	require.Equal(t, infrav1.StatePreparing, updatedHost.Status.ProvisioningState)
	require.Zero(t, robotFactory.calls)
}

var _ = Describe("HetznerBareMetalHostReconciler", func() {
	var (
		host           *infrav1.HetznerBareMetalHost
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
		var finish func()
		testNs, finish, err = testEnv.ResetAndCreateNamespace(ctx, "baremetalhost-reconciler")
		defer finish()
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
					if finalizer == infrav1.HetznerBareMetalHostFinalizer {
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
					c := conditions.Get(host, infrav1.HetznerBareMetalHostRootDeviceHintsValidatedCondition)
					return c != nil && c.Reason == infrav1.HetznerBareMetalHostRootDeviceHintsValidationFailedReason
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
					if host.Status.ProvisioningState == infrav1.StateImageInstalling || host.Status.ProvisioningState == infrav1.StateProvisioned {
						return true
					}
					testEnv.GetLogger().Info("reaches the state image installing. State",
						"is-state", host.Status.ProvisioningState,
						"should-state", infrav1.StateImageInstalling)
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
					if host.Status.ProvisioningState != infrav1.StateProvisioned {
						return false
					}

					return isPresentAndTrueWithReason(key, host, infrav1.HetznerBareMetalHostProvisionSucceededCondition, infrav1.HetznerBareMetalHostProvisionSucceededReason) &&
						isPresentAndTrueDeprecatedV1Beta1(key, host, infrav1.ProvisionSucceededV1Beta1Condition)
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
			bmMachine.Spec.InstallImage.Swraid = ptr.To(int32(1))
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
					if host.Status.ProvisioningState == infrav1.StateProvisioned {
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
			bmMachine.Spec.SSHSpec.PortAfterInstallImage = ptr.To[int32](23)

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
					if host.Status.ProvisioningState == infrav1.StateProvisioned {
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
		var finish func()
		testNs, finish, err = testEnv.ResetAndCreateNamespace(ctx, "baremetalmachine-reconciler")
		defer finish()
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
				return isPresentAndFalseWithReason(key, host, infrav1.HetznerBareMetalHostSSHKeysAvailableCondition, infrav1.HetznerBareMetalHostRescueSSHSecretMissingReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav1.CredentialsAvailableV1Beta1Condition, infrav1.RescueSSHSecretMissingV1Beta1Reason)
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
				return isPresentAndFalseWithReason(key, host, infrav1.HetznerBareMetalHostSSHKeysAvailableCondition, infrav1.HetznerBareMetalHostSSHKeysInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav1.CredentialsAvailableV1Beta1Condition, infrav1.SSHCredentialsInSecretInvalidV1Beta1Reason)
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
				return isPresentAndFalseWithReason(key, host, infrav1.HetznerBareMetalHostSSHKeysAvailableCondition, infrav1.HetznerBareMetalHostOSSSHSecretMissingReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav1.CredentialsAvailableV1Beta1Condition, infrav1.OSSSHSecretMissingV1Beta1Reason)
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
				return isPresentAndFalseWithReason(key, host, infrav1.HetznerBareMetalHostSSHKeysAvailableCondition, infrav1.HetznerBareMetalHostSSHKeysInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav1.CredentialsAvailableV1Beta1Condition, infrav1.SSHCredentialsInSecretInvalidV1Beta1Reason)
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
				return isPresentAndFalseWithReason(key, host, infrav1.HetznerBareMetalHostRobotCredentialsAvailableCondition, infrav1.HetznerBareMetalHostRobotCredentialsInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav1.RobotCredentialsAvailableV1Beta1Condition, infrav1.RobotCredentialsInvalidV1Beta1Reason)
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
				return isPresentAndFalseWithReason(key, host, infrav1.HetznerBareMetalHostRobotCredentialsAvailableCondition, infrav1.HetznerBareMetalHostRobotCredentialsInvalidReason) &&
					isPresentAndFalseWithReasonDeprecatedV1Beta1(key, host, infrav1.RobotCredentialsAvailableV1Beta1Condition, infrav1.RobotCredentialsInvalidV1Beta1Reason)
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
	newHostWithError := func(annotations map[string]string, errorType infrav1.ErrorType) infrav1.HetznerBareMetalHost {
		return infrav1.HetznerBareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: annotations,
			},
			Status: infrav1.HetznerBareMetalHostStatus{
				ErrorType: errorType,
			},
		}
	}
	reconciler := &HetznerBareMetalHostReconciler{EventRecorder: record.NewFakeRecorder(10)}

	// PermanentError with annotation --> Error should not get removed
	bmHost := newHostWithError(map[string]string{infrav1.PermanentErrorAnnotation: ""}, infrav1.ErrorTypePermanent)
	removed := reconciler.removePermanentErrorIfAnnotationIsGone(&bmHost)
	require.False(t, removed)
	require.NotEmpty(t, bmHost.Status.ErrorType)

	// PermanentError without annotation --> Error should get removed
	bmHost = newHostWithError(map[string]string{"other-annotation": "some value"}, infrav1.ErrorTypePermanent)
	removed = reconciler.removePermanentErrorIfAnnotationIsGone(&bmHost)
	require.True(t, removed)
	require.Empty(t, bmHost.Status.ErrorType)
	require.Equal(t, map[string]string{"other-annotation": "some value"}, bmHost.Annotations)

	// Other Error without annotation --> Error should not get removed
	bmHost = newHostWithError(map[string]string{}, infrav1.ErrorTypeFatal)
	removed = reconciler.removePermanentErrorIfAnnotationIsGone(&bmHost)
	require.False(t, removed)
	require.NotEmpty(t, bmHost.Status.ErrorType)
}

// Test_needsProvisioning checks every input that decides whether the host starts provisioning.
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

	// HetznerBareMetalMachine with bootstrap data --> provision
	hbmm, machine := newMachines()
	require.True(t, needsProvisioning(hbmm, machine))

	// no consuming HetznerBareMetalMachine --> do not provision
	require.False(t, needsProvisioning(nil, machine))

	// HetznerBareMetalMachine is being deleted --> do not provision
	hbmm, machine = newMachines()
	now := metav1.Now()
	hbmm.DeletionTimestamp = &now
	require.False(t, needsProvisioning(hbmm, machine))

	// no owning CAPI Machine (e.g. force deleted) --> do not provision
	hbmm, _ = newMachines()
	require.False(t, needsProvisioning(hbmm, nil))

	// owning CAPI Machine is being deleted --> do not provision
	hbmm, machine = newMachines()
	machine.DeletionTimestamp = &now
	require.False(t, needsProvisioning(hbmm, machine))

	// bootstrap data not ready --> do not provision
	hbmm, machine = newMachines()
	machine.Spec.Bootstrap.DataSecretName = nil
	require.False(t, needsProvisioning(hbmm, machine))
}

// Test_hetznerBareMetalMachinePredicate covers the filter of the watch on HetznerBareMetalMachine.
// Only the deletion timestamp and the spec change what the host does, and an update that touches
// neither must not reach the host.
func Test_hetznerBareMetalMachinePredicate(t *testing.T) {
	newMachine := func() *infrav1.HetznerBareMetalMachine {
		return &infrav1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bm-machine",
				Namespace:   "default",
				Annotations: map[string]string{infrav1.HostAnnotation: "default/host"},
			},
			Spec: infrav1.HetznerBareMetalMachineSpec{
				SSHSpec: infrav1.SSHSpec{PortAfterInstallImage: ptr.To[int32](22)},
			},
		}
	}
	update := func(oldObj, newObj client.Object) bool {
		return hetznerBareMetalMachinePredicate().Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
	}

	// status only change --> do not reconcile the host
	oldMachine, newMachine2 := newMachine(), newMachine()
	newMachine2.Status.Phase = "Running"
	require.False(t, update(oldMachine, newMachine2))

	// the HetznerBareMetalMachine is being deleted --> deprovision the host
	oldMachine, newMachine2 = newMachine(), newMachine()
	now := metav1.Now()
	newMachine2.DeletionTimestamp = &now
	require.True(t, update(oldMachine, newMachine2))

	// the spec changed --> the host reads the spec
	oldMachine, newMachine2 = newMachine(), newMachine()
	newMachine2.Spec.SSHSpec.PortAfterInstallImage = ptr.To[int32](2222)
	require.True(t, update(oldMachine, newMachine2))

	// not a HetznerBareMetalMachine --> reconcile, because we cannot tell what changed
	require.True(t, update(&infrav1.HetznerBareMetalHost{}, &infrav1.HetznerBareMetalHost{}))
}

// Test_hetznerBareMetalMachineToHetznerBareMetalHost covers the mapper of the watch on
// HetznerBareMetalMachine. The host both starts provisioning and deprovisions based on its
// HetznerBareMetalMachine, so those events must enqueue the bound host.
func Test_hetznerBareMetalMachineToHetznerBareMetalHost(t *testing.T) {
	ctx := context.Background()

	// HetznerBareMetalMachine without host annotation --> no request
	hbmm := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bm-machine",
			Namespace: "test-ns",
		},
	}
	require.Nil(t, hetznerBareMetalMachineToHetznerBareMetalHost(ctx, hbmm))

	// HetznerBareMetalMachine with host annotation --> request for the bound host
	hbmm.Annotations = map[string]string{
		infrav1.HostAnnotation: "test-ns/test-host",
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
	hbmm.Annotations[infrav1.HostAnnotation] = "test-host"
	require.Nil(t, hetznerBareMetalMachineToHetznerBareMetalHost(ctx, hbmm))

	// other object type --> no request
	require.Nil(t, hetznerBareMetalMachineToHetznerBareMetalHost(ctx, &infrav1.HetznerBareMetalHost{}))
}

func listFailsFor[L client.ObjectList]() interceptor.Funcs {
	return interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(L); ok {
				return errors.New("list failed")
			}
			return c.List(ctx, list, opts...)
		},
	}
}

func secretMapperMachine(namespace, name, osSSHSecretName, claimedHostName string) *infrav1.HetznerBareMetalMachine {
	machine := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	machine.Spec.SSHSpec.SecretRef.Name = osSSHSecretName
	if claimedHostName != "" {
		machine.Annotations = map[string]string{infrav1.HostAnnotation: namespace + "/" + claimedHostName}
	}
	return machine
}

func secretMapperCluster(namespace, name, hetznerClusterName string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrav1.GroupVersion.Group,
				Kind:     "HetznerCluster",
				Name:     hetznerClusterName,
			},
		},
	}
}

func secretMapperHetznerCluster(namespace, name, hetznerSecretName, rescueSecretName string) *infrav1.HetznerCluster {
	hetznerCluster := &infrav1.HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	hetznerCluster.Spec.HetznerSecret.Name = hetznerSecretName
	if rescueSecretName != "" {
		hetznerCluster.Spec.SSHKeys.RescueSecretRef = &infrav1.SSHSecretRef{Name: rescueSecretName}
	}
	return hetznerCluster
}

func secretMapperRequest(namespace, name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}
}

type secretMapperCase struct {
	name         string
	objects      []client.Object
	secretName   string
	secretNs     string
	watchFilter  string
	interceptors interceptor.Funcs
	want         []reconcile.Request
}

func newSecretMapperReconciler(objects []client.Object, interceptors interceptor.Funcs, watchFilter string) *HetznerBareMetalHostReconciler {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(infrav1.AddToScheme(scheme))

	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(interceptors).Build()
	return &HetznerBareMetalHostReconciler{Client: c, WatchFilterValue: watchFilter}
}

func runSecretMapperCases(t *testing.T, defaultNamespace string, tests []secretMapperCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newSecretMapperReconciler(tt.objects, tt.interceptors, tt.watchFilter)

			secretNs := tt.secretNs
			if secretNs == "" {
				secretNs = defaultNamespace
			}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tt.secretName, Namespace: secretNs}}

			got := r.secretToHetznerBareMetalHosts(context.Background(), secret)
			require.ElementsMatch(t, tt.want, got)
		})
	}
}

func Test_secretToHetznerBareMetalHosts_clusterSecrets(t *testing.T) {
	const ns = "ns-a"

	pausedCluster := secretMapperCluster(ns, "paused-cluster", "hc-paused")
	pausedCluster.Spec.Paused = ptr.To(true)

	pausedHost := helpers.BareMetalHost("paused-host", ns, helpers.WithClusterNameLabel("cluster-a"))
	pausedHost.Annotations = map[string]string{clusterv1.PausedAnnotation: ""}

	watchedHost := helpers.BareMetalHost("watched-host", ns, helpers.WithClusterNameLabel("cluster-a"))
	watchedHost.Labels[clusterv1.WatchLabel] = "foo"
	otherWatchHost := helpers.BareMetalHost("other-watch-host", ns, helpers.WithClusterNameLabel("cluster-a"))
	otherWatchHost.Labels[clusterv1.WatchLabel] = "bar"
	unwatchedHost := helpers.BareMetalHost("unwatched-host", ns, helpers.WithClusterNameLabel("cluster-a"))

	failHostListOfClusterA := interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*infrav1.HetznerBareMetalHostList); ok {
				listOpts := &client.ListOptions{}
				listOpts.ApplyOptions(opts)
				if listOpts.LabelSelector != nil && listOpts.LabelSelector.Matches(labels.Set{clusterv1.ClusterNameLabel: "cluster-a"}) {
					return errors.New("host list failed")
				}
			}
			return c.List(ctx, list, opts...)
		},
	}
	baseObjects := func() []client.Object {
		return []client.Object{
			secretMapperCluster(ns, "cluster-a", "hc-a"),
			secretMapperHetznerCluster(ns, "hc-a", "hetzner", "rescue"),
			helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
			helpers.BareMetalHost("host-2", ns, helpers.WithClusterNameLabel("cluster-a"), helpers.WithConsumerRef()),
		}
	}

	tests := []secretMapperCase{
		{
			name:       "Hetzner Secret reconciles all claimed hosts of the cluster",
			objects:    baseObjects(),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name:       "rescue Secret reconciles claimed hosts with and without a consumerRef",
			objects:    baseObjects(),
			secretName: "rescue",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name: "Secret in both roles of one HetznerCluster returns each host once",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "dup", "dup"),
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
			},
			secretName: "dup",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1")},
		},
		{
			name: "shared Hetzner Secret reconciles the hosts of every referencing cluster",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "shared", ""),
				helpers.BareMetalHost("host-a", ns, helpers.WithClusterNameLabel("cluster-a")),
				secretMapperCluster(ns, "cluster-b", "hc-b"),
				secretMapperHetznerCluster(ns, "hc-b", "shared", ""),
				helpers.BareMetalHost("host-b", ns, helpers.WithClusterNameLabel("cluster-b")),
			},
			secretName: "shared",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-a"), secretMapperRequest(ns, "host-b")},
		},
		{
			name: "HetznerCluster without a rescue Secret does not match another Secret",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "hetzner", ""),
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
			},
			secretName: "unrelated",
		},
		{
			name:       "mapper returns no request for a Secret that nothing references",
			objects:    baseObjects(),
			secretName: "unrelated",
		},
		{
			name:       "Secret in another namespace reconciles nothing",
			objects:    baseObjects(),
			secretName: "hetzner",
			secretNs:   "ns-b",
		},
		{
			name: "same cluster name in another namespace maps only the host in the Secret namespace",
			objects: append(baseObjects(),
				secretMapperCluster("ns-b", "cluster-a", "hc-a"),
				secretMapperHetznerCluster("ns-b", "hc-a", "hetzner", ""),
				helpers.BareMetalHost("host-in-b", "ns-b", helpers.WithClusterNameLabel("cluster-a")),
			),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name: "hosts of a cluster that references other Secrets are not reconciled",
			objects: append(baseObjects(),
				secretMapperCluster(ns, "cluster-b", "hc-b"),
				secretMapperHetznerCluster(ns, "hc-b", "other-hetzner", "other-rescue"),
				helpers.BareMetalHost("host-b", ns, helpers.WithClusterNameLabel("cluster-b")),
			),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name:       "host without the cluster-name label is not reconciled",
			objects:    append(baseObjects(), helpers.BareMetalHost("unclaimed", ns)),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name:       "host with an empty cluster-name label is not reconciled",
			objects:    append(baseObjects(), helpers.BareMetalHost("empty-label", ns, helpers.WithClusterNameLabel(""))),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name: "watch filter set reconciles only hosts with the watch label",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "hetzner", ""),
				watchedHost, otherWatchHost, unwatchedHost,
			},
			secretName:  "hetzner",
			watchFilter: "foo",
			want:        []reconcile.Request{secretMapperRequest(ns, "watched-host")},
		},
		{
			name: "watch filter unset reconciles hosts whatever labels they carry",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "hetzner", ""),
				watchedHost, otherWatchHost, unwatchedHost,
			},
			secretName: "hetzner",
			want: []reconcile.Request{
				secretMapperRequest(ns, "watched-host"),
				secretMapperRequest(ns, "other-watch-host"),
				secretMapperRequest(ns, "unwatched-host"),
			},
		},
		{
			name: "host with the paused annotation is not reconciled",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "hetzner", ""),
				pausedHost,
				helpers.BareMetalHost("running-host", ns, helpers.WithClusterNameLabel("cluster-a")),
			},
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "running-host")},
		},
		{
			name: "hosts of a paused Cluster are not reconciled",
			objects: []client.Object{
				pausedCluster,
				secretMapperHetznerCluster(ns, "hc-paused", "hetzner", ""),
				helpers.BareMetalHost("host-in-paused-cluster", ns, helpers.WithClusterNameLabel("paused-cluster")),
			},
			secretName: "hetzner",
		},
		{
			name: "HetznerCluster that no Cluster references is skipped and others are mapped",
			objects: append(baseObjects(),
				secretMapperHetznerCluster(ns, "hc-orphan", "hetzner", ""),
			),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name: "HetznerCluster owned by a Cluster that does not reference it is skipped",
			objects: func() []client.Object {
				clusterB := secretMapperCluster(ns, "cluster-b", "hc-b")
				clusterB.UID = "cluster-b-uid"
				owned := secretMapperHetznerCluster(ns, "hc-owned", "hetzner", "")
				owned.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       clusterB.Name,
					UID:        clusterB.UID,
				}}
				return append(baseObjects(),
					clusterB,
					owned,
					helpers.BareMetalHost("host-b", ns, helpers.WithClusterNameLabel("cluster-b")),
				)
			}(),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name: "Cluster with an infrastructureRef of another kind does not match",
			objects: func() []client.Object {
				otherKind := secretMapperCluster(ns, "cluster-x", "hc-x")
				otherKind.Spec.InfrastructureRef.Kind = "OtherCluster"
				otherGroup := secretMapperCluster(ns, "cluster-y", "hc-y")
				otherGroup.Spec.InfrastructureRef.APIGroup = "other.example.com"
				return append(baseObjects(),
					otherKind, secretMapperHetznerCluster(ns, "hc-x", "hetzner", ""),
					helpers.BareMetalHost("host-x", ns, helpers.WithClusterNameLabel("cluster-x")),
					otherGroup, secretMapperHetznerCluster(ns, "hc-y", "hetzner", ""),
					helpers.BareMetalHost("host-y", ns, helpers.WithClusterNameLabel("cluster-y")),
				)
			}(),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name: "two Clusters that reference one HetznerCluster map the hosts of both",
			objects: append(baseObjects(),
				secretMapperCluster(ns, "cluster-b", "hc-a"),
				helpers.BareMetalHost("host-b", ns, helpers.WithClusterNameLabel("cluster-b")),
			),
			secretName: "hetzner",
			want: []reconcile.Request{
				secretMapperRequest(ns, "host-1"),
				secretMapperRequest(ns, "host-2"),
				secretMapperRequest(ns, "host-b"),
			},
		},
		{
			name:         "Cluster list failure returns nothing",
			objects:      baseObjects(),
			secretName:   "hetzner",
			interceptors: listFailsFor[*clusterv1.ClusterList](),
		},
		{
			name:         "HetznerCluster list failure returns nothing",
			objects:      baseObjects(),
			secretName:   "hetzner",
			interceptors: listFailsFor[*infrav1.HetznerClusterList](),
		},
		{
			name: "host list failure of one cluster does not hide another cluster",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "shared", ""),
				helpers.BareMetalHost("host-a", ns, helpers.WithClusterNameLabel("cluster-a")),
				secretMapperCluster(ns, "cluster-b", "hc-b"),
				secretMapperHetznerCluster(ns, "hc-b", "shared", ""),
				helpers.BareMetalHost("host-b", ns, helpers.WithClusterNameLabel("cluster-b")),
			},
			secretName:   "shared",
			interceptors: failHostListOfClusterA,
			want:         []reconcile.Request{secretMapperRequest(ns, "host-b")},
		},
	}

	runSecretMapperCases(t, ns, tests)

	t.Run("an object that is not a Secret returns no requests", func(t *testing.T) {
		r := &HetznerBareMetalHostReconciler{Client: fakeclient.NewClientBuilder().Build()}
		require.Empty(t, r.secretToHetznerBareMetalHosts(context.Background(), &infrav1.HetznerBareMetalHost{}))
	})
}

func Test_secretToHetznerBareMetalHosts_osSSHSecret(t *testing.T) {
	const ns = "ns-a"

	pausedCluster := secretMapperCluster(ns, "paused-cluster", "hc-paused")
	pausedCluster.Spec.Paused = ptr.To(true)

	pausedHost := helpers.BareMetalHost("paused-host", ns, helpers.WithClusterNameLabel("cluster-a"))
	pausedHost.Annotations = map[string]string{clusterv1.PausedAnnotation: ""}

	watchedHost := helpers.BareMetalHost("watched-host", ns, helpers.WithClusterNameLabel("cluster-a"))
	watchedHost.Labels[clusterv1.WatchLabel] = "foo"

	failGetOfHost1 := interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*infrav1.HetznerBareMetalHost); ok && key.Name == "host-1" {
				return errors.New("get failed")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}

	baseObjects := func() []client.Object {
		return []client.Object{
			secretMapperCluster(ns, "cluster-a", "hc-a"),
			secretMapperHetznerCluster(ns, "hc-a", "hetzner", "rescue"),
			helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
			helpers.BareMetalHost("host-2", ns, helpers.WithClusterNameLabel("cluster-a")),
			secretMapperMachine(ns, "machine-1", "os-1", "host-1"),
			secretMapperMachine(ns, "machine-2", "os-2", "host-2"),
		}
	}

	tests := []secretMapperCase{
		{
			name:       "OS SSH Secret reconciles only the host of the machine that names it",
			objects:    baseObjects(),
			secretName: "os-1",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1")},
		},
		{
			name: "OS SSH Secret shared by two machines reconciles both hosts",
			objects: append(baseObjects(),
				secretMapperMachine(ns, "machine-3", "os-1", "host-2"),
			),
			secretName: "os-1",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name:       "Secret in the roles Hetzner and OS SSH returns each host once",
			objects:    append(baseObjects(), secretMapperMachine(ns, "machine-4", "hetzner", "host-1")),
			secretName: "hetzner",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name: "machine without the host annotation reconciles nothing",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
				secretMapperMachine(ns, "machine-1", "os-1", ""),
			},
			secretName: "os-1",
		},
		{
			name: "host that does not exist reconciles nothing",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperMachine(ns, "machine-1", "os-1", "missing-host"),
			},
			secretName: "os-1",
		},
		{
			name: "OS SSH Secret in another namespace reconciles nothing",
			objects: append(baseObjects(),
				secretMapperMachine("ns-b", "machine-b", "os-1", "host-b"),
			),
			secretName: "os-1",
			secretNs:   "ns-b",
		},
		{
			name: "host with the paused annotation is not reconciled",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				pausedHost,
				secretMapperMachine(ns, "machine-1", "os", "paused-host"),
			},
			secretName: "os",
		},
		{
			name: "host of a paused Cluster is not reconciled",
			objects: []client.Object{
				pausedCluster,
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("paused-cluster")),
				secretMapperMachine(ns, "machine-1", "os", "host-1"),
			},
			secretName: "os",
		},
		{
			name: "host without the cluster-name label is not reconciled",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				helpers.BareMetalHost("unclaimed", ns),
				secretMapperMachine(ns, "machine-1", "os", "unclaimed"),
			},
			secretName: "os",
		},
		{
			name: "host with an empty cluster-name label is not reconciled",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				helpers.BareMetalHost("empty-label", ns, helpers.WithClusterNameLabel("")),
				secretMapperMachine(ns, "machine-1", "os", "empty-label"),
			},
			secretName: "os",
		},
		{
			name: "host whose Cluster does not exist is skipped and the other machine is mapped",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
				helpers.BareMetalHost("host-lost", ns, helpers.WithClusterNameLabel("deleted-cluster")),
				secretMapperMachine(ns, "machine-1", "os", "host-1"),
				secretMapperMachine(ns, "machine-0-lost", "os", "host-lost"),
			},
			secretName: "os",
			want:       []reconcile.Request{secretMapperRequest(ns, "host-1")},
		},
		{
			name: "watch filter set skips a host without the watch label",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				watchedHost,
				helpers.BareMetalHost("unwatched-host", ns, helpers.WithClusterNameLabel("cluster-a")),
				secretMapperMachine(ns, "machine-1", "os", "watched-host"),
				secretMapperMachine(ns, "machine-2", "os", "unwatched-host"),
			},
			secretName:  "os",
			watchFilter: "foo",
			want:        []reconcile.Request{secretMapperRequest(ns, "watched-host")},
		},
		{
			name: "HetznerCluster list failure still returns the OS SSH host",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "hetzner", "os-1"),
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
				helpers.BareMetalHost("host-2", ns, helpers.WithClusterNameLabel("cluster-a")),
				secretMapperMachine(ns, "machine-1", "os-1", "host-1"),
			},
			secretName:   "os-1",
			interceptors: listFailsFor[*infrav1.HetznerClusterList](),
			want:         []reconcile.Request{secretMapperRequest(ns, "host-1")},
		},
		{
			name: "Cluster list failure still returns the OS SSH host",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				secretMapperHetznerCluster(ns, "hc-a", "hetzner", "os-1"),
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
				helpers.BareMetalHost("host-2", ns, helpers.WithClusterNameLabel("cluster-a")),
				secretMapperMachine(ns, "machine-1", "os-1", "host-1"),
			},
			secretName:   "os-1",
			interceptors: listFailsFor[*clusterv1.ClusterList](),
			want:         []reconcile.Request{secretMapperRequest(ns, "host-1")},
		},
		{
			name: "machine list failure still returns the cluster-level hosts",
			objects: append(baseObjects(),
				secretMapperCluster(ns, "cluster-b", "hc-b"),
				helpers.BareMetalHost("host-3", ns, helpers.WithClusterNameLabel("cluster-b")),
				secretMapperMachine(ns, "machine-3", "hetzner", "host-3"),
			),
			secretName:   "hetzner",
			interceptors: listFailsFor[*infrav1.HetznerBareMetalMachineList](),
			want:         []reconcile.Request{secretMapperRequest(ns, "host-1"), secretMapperRequest(ns, "host-2")},
		},
		{
			name:         "machine list failure returns nothing for an OS SSH Secret",
			objects:      baseObjects(),
			secretName:   "os-1",
			interceptors: listFailsFor[*infrav1.HetznerBareMetalMachineList](),
		},
		{
			name: "host Get error skips that host and maps the other machine",
			objects: []client.Object{
				secretMapperCluster(ns, "cluster-a", "hc-a"),
				helpers.BareMetalHost("host-1", ns, helpers.WithClusterNameLabel("cluster-a")),
				helpers.BareMetalHost("host-2", ns, helpers.WithClusterNameLabel("cluster-a")),
				secretMapperMachine(ns, "machine-1", "os", "host-1"),
				secretMapperMachine(ns, "machine-2", "os", "host-2"),
			},
			secretName:   "os",
			interceptors: failGetOfHost1,
			want:         []reconcile.Request{secretMapperRequest(ns, "host-2")},
		},
	}

	runSecretMapperCases(t, ns, tests)
}

func Test_hetznerSecretErrorResult_requeue(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want reconcile.Result
	}{
		{
			name: "missing Hetzner Secret is requeued",
			err:  &secretutil.ResolveSecretRefError{Message: "hetzner-secret"},
			want: reconcile.Result{RequeueAfter: 5 * time.Minute},
		},
		{
			name: "credentials validation error is not requeued",
			err:  &bmclient.CredentialsValidationError{Message: "invalid credentials"},
			want: reconcile.Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &HetznerBareMetalHostReconciler{EventRecorder: record.NewFakeRecorder(10)}

			got, err := reconciler.hetznerSecretErrorResult(tt.err, &infrav1.HetznerBareMetalHost{})

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

type hostWatchProbe struct {
	mu                 sync.Mutex
	seq                int
	lastMapperRun      int
	lastReconcileStart int
	mapperRuns         int
	reconcileStarts    int
}

func (p *hostWatchProbe) mapperRan() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seq++
	p.lastMapperRun = p.seq
	p.mapperRuns++
}

func (p *hostWatchProbe) reconcileStarted() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seq++
	p.lastReconcileStart = p.seq
	p.reconcileStarts++
}

func (p *hostWatchProbe) snapshot() (mapperRuns, reconcileStarts int, reconciledAfterLastMapperRun bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mapperRuns, p.reconcileStarts, p.lastMapperRun < p.lastReconcileStart
}

func (p *hostWatchProbe) observations() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seq
}

type hostWatchRecordingClient struct {
	client.Client
	namespace string
	probe     *hostWatchProbe
}

func (c hostWatchRecordingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Namespace); ok && key.Name == c.namespace {
		c.probe.reconcileStarted()
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c hostWatchRecordingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*infrav1.HetznerBareMetalMachineList); ok {
		c.probe.mapperRan()
	}
	return c.Client.List(ctx, list, opts...)
}

var _ = Describe("HetznerBareMetalHostReconciler - Hetzner Secret watch", func() {
	const (
		watchFilterValue       = "baremetalhost-secret-watch-filter-test"
		secretEventQuietPeriod = 300 * time.Millisecond
	)

	var (
		fixtureNs         *corev1.Namespace
		host              *infrav1.HetznerBareMetalHost
		hetznerSecret     *corev1.Secret
		hostKey           client.ObjectKey
		hostProbe         *hostWatchProbe
		hostReconcileGate *sync.RWMutex
	)

	startDedicatedHostManager := func(robotPassword string, robotClientFactory robotclient.Factory) {
		_, finish, err := testEnv.ResetAndCreateNamespace(ctx, "host-watch-unused")
		Expect(err).NotTo(HaveOccurred())
		if robotClientFactory == nil {
			robotClientFactory = testEnv.RobotClientFactory
		}

		// finish() has to run before the manager starts, because otherwise a host reconcile can get
		// the SSH mocks of the previous test.
		func() {
			defer finish()
			robotClient := testEnv.RobotClient
			robotClient.On("GetBMServer", mock.Anything).Return(func(int) (*models.Server, error) {
				return &models.Server{ServerNumber: 1, ServerIP: "1.2.3.4", Rescue: true}, nil
			})
			robotClient.On("ListSSHKeys").Return([]models.Key{{Name: "my-name", Fingerprint: "my-fingerprint", Data: "my-public-key"}}, nil)
			robotClient.On("GetReboot", mock.Anything).Return(&models.Reset{Type: []string{"hw", "sw"}}, nil)
			robotClient.On("GetBootRescue", 1).Return(&models.Rescue{Active: true}, nil)
			robotClient.On("SetBootRescue", mock.Anything, mock.Anything).Return(&models.Rescue{Active: true}, nil)
			robotClient.On("DeleteBootRescue", mock.Anything).Return(&models.Rescue{Active: false}, nil)
			robotClient.On("RebootBMServer", mock.Anything, mock.Anything).Return(&models.ResetPost{}, nil)
			robotClient.On("SetBMServerName", mock.Anything, mock.Anything).Return(nil, nil)

			configureRescueSSHClient(testEnv.RescueSSHClient)

			osSSHClient := testEnv.OSSSHClientAfterInstallImage
			osSSHClient.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + "machine"})
			osSSHClient.On("Reboot", mock.Anything).Return(sshclient.Output{})
			osSSHClient.On("CloudInitStatus", mock.Anything).Return(sshclient.Output{StdOut: "status: done"})
			osSSHClient.On("CheckCloudInitLogsForSigTerm", mock.Anything).Return(sshclient.Output{})
			osSSHClient.On("ResetKubeadm", mock.Anything).Return(sshclient.Output{})
		}()

		fixtureNs = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "host-secret-watch-"}}
		Expect(testEnv.Create(ctx, fixtureNs)).To(Succeed())

		mgr, err := ctrl.NewManager(testEnv.Config, ctrl.Options{
			Scheme: testEnv.GetScheme(),
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{fixtureNs.Name: {}},
				ByObject:          secretutil.AddSecretSelector(),
			},
			Metrics: metricsserver.Options{BindAddress: "0"},
		})
		Expect(err).NotTo(HaveOccurred())
		hostProbe = &hostWatchProbe{}
		hostReconcileGate = &sync.RWMutex{}
		hostReconciler := &HetznerBareMetalHostReconciler{
			Client:              hostWatchRecordingClient{Client: mgr.GetClient(), namespace: fixtureNs.Name, probe: hostProbe},
			APIReader:           mgr.GetAPIReader(),
			RobotClientFactory:  robotClientFactory,
			SSHClientFactory:    testEnv.BaremetalSSHClientFactory,
			WatchFilterValue:    watchFilterValue,
			PreProvisionCommand: "dummy-pre-provision-command",
			Namespace:           fixtureNs.Name,
			ReconcileGate:       hostReconcileGate,
		}
		Expect(hostReconciler.SetupWithManager(ctx, mgr, controller.Options{
			SkipNameValidation: ptr.To(true),
			RateLimiter:        workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](time.Hour, time.Hour),
		})).To(Succeed())

		hetznerClusterName := utils.GenerateName(nil, "hetzner-cluster-test")
		machineName := utils.GenerateName(nil, "machine")

		capiCluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    fixtureNs.Name,
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

		hetznerCluster := &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hetznerClusterName,
				Namespace: fixtureNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
					clusterv1.WatchLabel:       watchFilterValue,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       capiCluster.Name,
						UID:        capiCluster.UID,
					},
				},
			},
			Spec: helpers.GetDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

		capiMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       machineName,
				Namespace:  fixtureNs.Name,
				Finalizers: []string{clusterv1.MachineFinalizer},
				Labels:     map[string]string{clusterv1.ClusterNameLabel: capiCluster.Name},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: capiCluster.Name,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "HetznerBareMetalMachine",
					Name:     machineName,
				},
				FailureDomain: defaultFailureDomain,
				Bootstrap:     clusterv1.Bootstrap{DataSecretName: ptr.To("bootstrap-secret")},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		bmMachine := &infrav1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        machineName,
				Namespace:   fixtureNs.Name,
				Labels:      map[string]string{clusterv1.ClusterNameLabel: capiCluster.Name},
				Annotations: map[string]string{infrav1.HostAnnotation: fixtureNs.Name + "/" + hostName},
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

		bootstrapSecret := getDefaultBootstrapSecret(fixtureNs.Name)
		Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())

		host = helpers.BareMetalHost(
			hostName,
			fixtureNs.Name,
			helpers.WithRootDeviceHintWWN(),
			helpers.WithClusterNameLabel(capiCluster.Name),
		)
		host.Labels[clusterv1.WatchLabel] = watchFilterValue
		host.Spec.ConsumerRef = infrav1.HetznerBareMetalHostConsumerReference{
			Name:     machineName,
			Kind:     "HetznerBareMetalMachine",
			APIGroup: infrav1.GroupVersion.Group,
		}
		Expect(testEnv.Create(ctx, host)).To(Succeed())
		hostKey = client.ObjectKeyFromObject(host)

		hetznerSecret = getDefaultHetznerSecret(fixtureNs.Name)
		hetznerSecret.Data["robot-password"] = []byte(robotPassword)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())
		rescueSSHSecret := helpers.GetDefaultSSHSecret("rescue-ssh-secret", fixtureNs.Name)
		rescueSSHSecret.Labels = map[string]string{secretutil.LabelEnvironmentName: secretutil.LabelEnvironmentValue}
		Expect(testEnv.Create(ctx, rescueSSHSecret)).To(Succeed())
		osSSHSecret := helpers.GetDefaultSSHSecret("os-ssh-secret", fixtureNs.Name)
		osSSHSecret.Labels = map[string]string{secretutil.LabelEnvironmentName: secretutil.LabelEnvironmentValue}
		Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())

		DeferCleanup(func() {
			Expect(testEnv.Cleanup(ctx, fixtureNs, host, hetznerSecret, rescueSSHSecret, osSSHSecret, bootstrapSecret, bmMachine, capiMachine, hetznerCluster, capiCluster)).To(Succeed())
		})

		managerCtx, cancelManager := context.WithCancel(ctx)
		managerDone := make(chan error, 1)
		go func() { managerDone <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			cancelManager()
			Eventually(managerDone, timeout).Should(Receive(Succeed()))
		})
	}

	robotCredentialsInvalid := func() bool {
		return isPresentAndFalseWithReason(hostKey, host, infrav1.HetznerBareMetalHostRobotCredentialsAvailableCondition, infrav1.HetznerBareMetalHostRobotCredentialsInvalidReason)
	}

	newRejectingRobotClient := func() *robotmock.Client {
		rejectingRobotClient := robotmock.NewClient(GinkgoT())
		rejectingRobotClient.On("GetBMServer", mock.Anything).Return(nil, models.Error{
			Code:    models.ErrorCodeUnauthorized,
			Message: "You are not authorized - wrong RobotCredentials",
		})
		return rejectingRobotClient
	}

	robotCredentialsAvailable := func() bool {
		return isPresentAndTrueWithReason(hostKey, host, infrav1.HetznerBareMetalHostRobotCredentialsAvailableCondition, infrav1.HetznerBareMetalHostRobotCredentialsAvailableReason)
	}

	reconciledAfterEachLabelledSecret := func(labelledSecretCount int) (int, bool) {
		if !hostReconcileGate.TryLock() {
			return 0, false
		}
		defer hostReconcileGate.Unlock()
		mapperRuns, reconcileStarts, reconciledAfterLastMapperRun := hostProbe.snapshot()
		if labelledSecretCount == 0 || mapperRuns < labelledSecretCount || !reconciledAfterLastMapperRun {
			return 0, false
		}
		return reconcileStarts, true
	}

	waitUntilSecretEventsAreReconciled := func() int {
		var reconcileStarts int
		Eventually(func() bool {
			labelled := &corev1.SecretList{}
			listCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if err := testEnv.GetAPIReader().List(listCtx, labelled, client.InNamespace(fixtureNs.Name), client.MatchingLabels{secretutil.LabelEnvironmentName: secretutil.LabelEnvironmentValue}); err != nil {
				return false
			}
			if _, ok := reconciledAfterEachLabelledSecret(len(labelled.Items)); !ok {
				return false
			}
			settled := hostProbe.observations()
			time.Sleep(secretEventQuietPeriod)
			if hostProbe.observations() != settled {
				return false
			}
			starts, ok := reconciledAfterEachLabelledSecret(len(labelled.Items))
			if !ok {
				return false
			}
			reconcileStarts = starts
			return true
		}, timeout, interval).Should(BeTrue(), "no quiet reconcile followed the Secret mapper run of each labelled Secret; the probe counts the List of HetznerBareMetalMachines in hostsOfOSSSHSecret as a mapper run and the Get of the fixture Namespace in shouldSkipReconciliationForNamespace as a reconcile start")
		return reconcileStarts
	}

	It("reconciles the host when the Robot password is added to the Hetzner Secret", func() {
		startDedicatedHostManager("", nil)

		By("waiting for the host to be stuck on the empty Robot password")
		Eventually(robotCredentialsInvalid, timeout, interval).Should(BeTrue(), "RobotCredentialsAvailable is not False with reason Invalid")

		By("waiting for the labelled Secret mapper runs and a later reconcile")
		waitUntilSecretEventsAreReconciled()
		Expect(robotCredentialsInvalid()).To(BeTrue())
		mapperRunsBefore, _, _ := hostProbe.snapshot()

		By("adding the Robot password to the Secret data, which carries no watch-filter label")
		Expect(testEnv.GetAPIReader().Get(ctx, client.ObjectKeyFromObject(hetznerSecret), hetznerSecret)).To(Succeed())
		hetznerSecret.Data["robot-password"] = []byte("my-password")
		Expect(testEnv.Update(ctx, hetznerSecret)).To(Succeed())

		Eventually(func() int {
			mapperRuns, _, _ := hostProbe.snapshot()
			return mapperRuns
		}, timeout, interval).Should(BeNumerically(">", mapperRunsBefore), "the Secret change did not reach the Secret mapper")

		By("expecting the host to be reconciled again and to accept the credentials")
		Eventually(robotCredentialsAvailable, timeout, interval).Should(BeTrue(), "RobotCredentialsAvailable did not turn True after the Secret change")
	})

	It("reconciles the host when the Hetzner Secret is deleted", func() {
		rejectingRobotClient := newRejectingRobotClient()
		startDedicatedHostManager("wrong-password", robotFactoryFunc(func(robotclient.Credentials) robotclient.Client {
			return rejectingRobotClient
		}))

		By("waiting for the host to be stuck on the rejected password")
		Eventually(robotCredentialsInvalid, timeout, interval).Should(BeTrue(), "RobotCredentialsAvailable is not False with reason Invalid")

		By("waiting for the labelled Secret mapper runs and a later reconcile")
		waitUntilSecretEventsAreReconciled()
		Expect(robotCredentialsInvalid()).To(BeTrue())
		mapperRunsBefore, _, _ := hostProbe.snapshot()

		By("deleting the Hetzner Secret")
		Expect(testEnv.Delete(ctx, hetznerSecret)).To(Succeed())

		Eventually(func() int {
			mapperRuns, _, _ := hostProbe.snapshot()
			return mapperRuns
		}, timeout, interval).Should(BeNumerically(">", mapperRunsBefore), "the Secret change did not reach the Secret mapper")

		By("expecting the host to be reconciled again and to report the missing Secret")
		Eventually(func() bool {
			return isPresentAndFalseWithReason(hostKey, host, infrav1.HetznerBareMetalHostRobotCredentialsAvailableCondition, infrav1.HetznerBareMetalHostSecretUnreachableReason)
		}, timeout, interval).Should(BeTrue(), "RobotCredentialsAvailable is not False with reason SecretUnreachable")
	})

	It("reconciles the host when the Robot API rejects the password until the Secret changes", func() {
		rejectingRobotClient := newRejectingRobotClient()
		startDedicatedHostManager("wrong-password", robotFactoryFunc(func(credentials robotclient.Credentials) robotclient.Client {
			if credentials.Password == "my-password" {
				return testEnv.RobotClient
			}
			return rejectingRobotClient
		}))

		By("waiting for the host to be stuck on the rejected password")
		Eventually(robotCredentialsInvalid, timeout, interval).Should(BeTrue(), "RobotCredentialsAvailable is not False with reason Invalid")

		By("waiting for the labelled Secret mapper runs and a later reconcile")
		reconcileStarts := waitUntilSecretEventsAreReconciled()
		Expect(robotCredentialsInvalid()).To(BeTrue())
		mapperRunsBefore, _, _ := hostProbe.snapshot()

		By("checking that the host is not reconciled again before the Secret changes")
		Consistently(func() int { _, n, _ := hostProbe.snapshot(); return n }, time.Second, interval).
			Should(Equal(reconcileStarts), "the host was reconciled again before the Secret change")

		By("changing the Robot password in the Secret data")
		Expect(testEnv.GetAPIReader().Get(ctx, client.ObjectKeyFromObject(hetznerSecret), hetznerSecret)).To(Succeed())
		hetznerSecret.Data["robot-password"] = []byte("my-password")
		Expect(testEnv.Update(ctx, hetznerSecret)).To(Succeed())

		Eventually(func() int {
			mapperRuns, _, _ := hostProbe.snapshot()
			return mapperRuns
		}, timeout, interval).Should(BeNumerically(">", mapperRunsBefore), "the Secret change did not reach the Secret mapper")

		By("expecting the host to be reconciled again and to accept the credentials")
		Eventually(robotCredentialsAvailable, timeout, interval).Should(BeTrue(), "RobotCredentialsAvailable did not turn True after the Secret change")
	})
})

var _ = Describe("reconcileRobotRateLimit", func() {
	var host *infrav1.HetznerBareMetalHost
	BeforeEach(func() {
		host = helpers.BareMetalHost("rate-limit-host", "default")
	})

	It("returns wait==true if the robot rate limit is exceeded and the wait time is not over", func() {
		deprecatedv1beta1conditions.MarkFalse(host, infrav1.HetznerAPIReachableV1Beta1Condition,
			infrav1.RateLimitExceededV1Beta1Reason, clusterv1.ConditionSeverityWarning, "")
		conditions.Set(host, metav1.Condition{
			Type:               infrav1.HetznerBareMetalHostRobotRateLimitExceededCondition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav1.HetznerBareMetalHostRobotRateLimitExceededReason,
			LastTransitionTime: metav1.Now(),
		})
		Expect(reconcileRobotRateLimit(host, testEnv.RateLimitWaitTime)).To(BeTrue())
		Expect(conditions.Has(host, infrav1.HetznerBareMetalHostRobotRateLimitExceededCondition)).To(BeTrue())
		reachable := deprecatedv1beta1conditions.Get(host, infrav1.HetznerAPIReachableV1Beta1Condition)
		Expect(reachable).ToNot(BeNil())
		Expect(reachable.Status).To(Equal(corev1.ConditionFalse))
	})

	It("returns wait==false and clears both conditions when the wait time is over", func() {
		deprecatedv1beta1conditions.MarkFalse(host, infrav1.HetznerAPIReachableV1Beta1Condition,
			infrav1.RateLimitExceededV1Beta1Reason, clusterv1.ConditionSeverityWarning, "")
		conditions.Set(host, metav1.Condition{
			Type:               infrav1.HetznerBareMetalHostRobotRateLimitExceededCondition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav1.HetznerBareMetalHostRobotRateLimitExceededReason,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
		})

		Expect(reconcileRobotRateLimit(host, testEnv.RateLimitWaitTime)).To(BeFalse())

		// The condition is deleted rather than set to False, so the next Robot API call determines
		// the real rate-limit status instead of assuming the limit is gone.
		Expect(conditions.Has(host, infrav1.HetznerBareMetalHostRobotRateLimitExceededCondition)).To(BeFalse())
		reachable := deprecatedv1beta1conditions.Get(host, infrav1.HetznerAPIReachableV1Beta1Condition)
		Expect(reachable).ToNot(BeNil())
		Expect(reachable.Status).To(Equal(corev1.ConditionTrue))
	})

	It("returns wait==false if the robot rate limit condition is present but not exceeded", func() {
		conditions.Set(host, metav1.Condition{
			Type:               infrav1.HetznerBareMetalHostRobotRateLimitExceededCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "NotExceeded",
			LastTransitionTime: metav1.Now(),
		})
		Expect(reconcileRobotRateLimit(host, testEnv.RateLimitWaitTime)).To(BeFalse())
		Expect(conditions.IsFalse(host, infrav1.HetznerBareMetalHostRobotRateLimitExceededCondition)).To(BeTrue())
		Expect(deprecatedv1beta1conditions.Get(host, infrav1.HetznerAPIReachableV1Beta1Condition)).To(BeNil())
	})

	It("returns wait==false if the robot rate limit condition is not set", func() {
		Expect(reconcileRobotRateLimit(host, testEnv.RateLimitWaitTime)).To(BeFalse())
		Expect(conditions.Has(host, infrav1.HetznerBareMetalHostRobotRateLimitExceededCondition)).To(BeFalse())
		Expect(deprecatedv1beta1conditions.Get(host, infrav1.HetznerAPIReachableV1Beta1Condition)).To(BeNil())
	})
})
