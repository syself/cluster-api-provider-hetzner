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
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

var _ = Describe("HetznerBareMetalMachineReconciler", func() {
	var (
		bmMachine      *infrav1.HetznerBareMetalMachine
		hetznerCluster *infrav1.HetznerCluster

		capiCluster *clusterv1.Cluster

		hetznerClusterName string

		testNs *corev1.Namespace

		hetznerSecret   *corev1.Secret
		rescueSSHSecret *corev1.Secret
		bootstrapSecret *corev1.Secret

		key client.ObjectKey

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
				Labels: map[string]string{clusterv1.ClusterNameLabel: capiCluster.Name},
			},
			Spec: helpers.GetDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

		hetznerSecret = getDefaultHetznerSecret(testNs.Name)
		Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

		rescueSSHSecret = helpers.GetDefaultSSHSecret("rescue-ssh-secret", testNs.Name)
		Expect(testEnv.Create(ctx, rescueSSHSecret)).To(Succeed())

		bootstrapSecret = getDefaultBootstrapSecret(testNs.Name)
		Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())

		robotClient = testEnv.RobotClient
		rescueSSHClient = testEnv.RescueSSHClient
		osSSHClient = testEnv.OSSSHClientAfterInstallImage

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

		osSSHClient.On("Reboot").Return(sshclient.Output{})
		osSSHClient.On("CloudInitStatus").Return(sshclient.Output{StdOut: "status: done"})
		osSSHClient.On("CheckCloudInitLogsForSigTerm").Return(sshclient.Output{})
		osSSHClient.On("ResetKubeadm").Return(sshclient.Output{})
		osSSHClient.On("GetHostName").Return(sshclient.Output{
			StdOut: infrav1.BareMetalHostNamePrefix + bmMachineName,
			StdErr: "",
			Err:    nil,
		})
		osSSHClient.On("GetCloudInitOutput").Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, hetznerCluster,
			hetznerSecret, rescueSSHSecret, bootstrapSecret)).To(Succeed())
	})

	Context("Tests with host", func() {
		var (
			host    *infrav1.HetznerBareMetalHost
			hostKey client.ObjectKey

			capiMachine *clusterv1.Machine
		)

		BeforeEach(func() {
			host = helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithRootDeviceHintWWN(),
				helpers.WithHetznerClusterRef(hetznerClusterName),
			)
			Expect(testEnv.Create(ctx, host)).To(Succeed())

			hostKey = client.ObjectKey{Namespace: testNs.Name, Name: hostName}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, host)).To(Succeed())
		})

		Context("Test bootstrap", func() {
			BeforeEach(func() {
				capiMachineName := utils.GenerateName(nil, "capimachine-name-")
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

				key = client.ObjectKey{Namespace: testNs.Name, Name: bmMachineName}
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, capiMachine, bmMachine)).To(Succeed())
			})

			It("sets bootstrap condition on false if no bootstrap available", func() {
				Eventually(func() bool {
					return isPresentAndFalseWithReason(key, bmMachine, infrav1.BootstrapReadyCondition, infrav1.BootstrapNotReadyReason)
				}, timeout, time.Second).Should(BeTrue())
			})

			It("sets bootstrap condition on true if bootstrap available", func() {
				By("setting bootstrap information in capi machine")

				ph, err := patch.NewHelper(capiMachine, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				}
				Eventually(func() error {
					return ph.Patch(ctx, capiMachine, patch.WithStatusObservedGeneration{})
				}, timeout, time.Second).Should(BeNil())

				By("checking that bootstrap condition is set on true")

				Eventually(func() bool {
					return isPresentAndTrue(key, bmMachine, infrav1.BootstrapReadyCondition)
				}, timeout, time.Second).Should(BeTrue())
			})
		})

		Context("Basic hbmm test", func() {
			var osSSHSecret *corev1.Secret

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

				osSSHSecret = helpers.GetDefaultSSHSecret("os-ssh-secret", testNs.Name)
				Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: bmMachineName}
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, capiMachine, bmMachine, osSSHSecret)).To(Succeed())
			})

			It("creates the bare metal machine", func() {
				Eventually(func() error {
					return testEnv.Get(ctx, key, bmMachine)
				}, timeout).Should(BeNil())
			})

			It("sets the finalizer", func() {
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, bmMachine); err != nil {
						return false
					}
					for _, finalizer := range bmMachine.GetFinalizers() {
						if finalizer == infrav1.HetznerBareMetalMachineFinalizer {
							return true
						}
					}
					return false
				}, timeout).Should(BeTrue())
			})

			It("reaches state ready and provisions the host", func() {
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

			It("sets the appropriate conditions that a host is associated and later provisioned", func() {
				By("checking that the host is associated")
				Eventually(func() bool {
					return isPresentAndTrue(key, bmMachine, infrav1.HostAssociateSucceededCondition)
				}, timeout).Should(BeTrue())

				By("checking that the host is ready")
				Eventually(func() bool {
					return isPresentAndTrue(key, bmMachine, infrav1.HostReadyCondition)
				}, timeout).Should(BeTrue())

				By("checking that the bare metal machine is ready")
				Eventually(func() bool {
					return isPresentAndTrue(key, bmMachine, clusterv1.ReadyCondition)
				}, timeout).Should(BeTrue())
			})

			It("deletes successfully", func() {
				By("deleting bm machine")

				Expect(testEnv.Delete(ctx, bmMachine)).To(Succeed())

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, bmMachine); apierrors.IsNotFound(err) {
						return true
					}
					return false
				}, timeout, time.Second).Should(BeTrue())

				By("making sure the host has been deprovisioned")

				Eventually(func() bool {
					if err := testEnv.Get(ctx, hostKey, host); err != nil {
						return false
					}
					return host.Spec.Status.ProvisioningState == infrav1.StateNone
				}, timeout, time.Second).Should(BeTrue())
			})

			It("sets a failure reason when maintenance mode is set on the host", func() {
				By("making sure that machine is ready")

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, bmMachine); err != nil {
						return false
					}
					return bmMachine.Status.Ready
				}, timeout, time.Second).Should(BeTrue())

				By("setting maintenance mode on host")

				ph, err := patch.NewHelper(host, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				maintenanceMode := true
				host.Spec.MaintenanceMode = &maintenanceMode

				Expect(ph.Patch(ctx, host, patch.WithStatusObservedGeneration{})).To(Succeed())

				By("checking that failure message is set on machine")

				Eventually(func() error {
					if err := testEnv.Get(ctx, key, bmMachine); err != nil {
						return err
					}

					capiMachine, err := util.GetOwnerMachine(ctx, testEnv, bmMachine.ObjectMeta)
					if err != nil {
						return err
					}

					_, exists := capiMachine.Annotations[clusterv1.RemediateMachineAnnotation]
					if !exists {
						return fmt.Errorf("RemediateMachineAnnotation not set on capi machine")
					}

					c := conditions.Get(bmMachine, infrav1.NoRemediateMachineAnnotationCondition)
					if c == nil {
						return fmt.Errorf("Condition NoRemediateMachineAnnotationCondition does not exist.")
					}

					if c.Status != corev1.ConditionFalse {
						return fmt.Errorf("Condition NoRemediateMachineAnnotationCondition should be False")
					}

					return nil
				}, timeout).Should(Succeed())
			})

			It("checks the hetznerBareMetalMachine status running phase", func() {
				By("making sure that machine is in running state")
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, bmMachine); err != nil {
						return false
					}

					testEnv.GetLogger().Info("status of host and hetznerBareMetalMachine", "hetznerBareMetalMachine phase", bmMachine.Status.Phase, "host state", host.Spec.Status.ProvisioningState)
					return bmMachine.Status.Phase == clusterv1.MachinePhaseRunning
				}, timeout, time.Second).Should(BeTrue())
			})

			It("checks that HostReady condition is True for hetznerBareMetalMachine", func() {
				Eventually(func() bool {
					return isPresentAndTrue(key, bmMachine, infrav1.HostReadyCondition)
				}, timeout, time.Second).Should(BeTrue())
			})
		})

		Context("Test wrong Host", func() {
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

				key = client.ObjectKey{Namespace: testNs.Name, Name: bmMachineName}
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, capiMachine, bmMachine)).To(Succeed())
			})

			It("checks for HostReadyCondition False for hetznerBareMetalMachine with HetznerSecretUnreachableReason", func() {
				Eventually(func() bool {
					return isPresentAndFalseWithReason(key, bmMachine, infrav1.HostReadyCondition, infrav1.OSSSHSecretMissingReason)
				}, timeout, time.Second).Should(BeTrue())
			})
		})
	})

	Context("Tests without host", func() {
		var capiMachine *clusterv1.Machine

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

			key = client.ObjectKey{Namespace: testNs.Name, Name: bmMachineName}
		})

		It("creates the bare metal machine and sets condition that no host is available", func() {
			Eventually(func() bool {
				return isPresentAndFalseWithReason(key, bmMachine, infrav1.HostAssociateSucceededCondition, infrav1.NoAvailableHostReason)
			}, timeout).Should(BeTrue())
		})

		It("checks the hetznerBareMetalMachine status pending phase", func() {
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, bmMachine); err != nil {
					return false
				}

				testEnv.GetLogger().Info("phase of hetznerBareMetalMachine", "phase", bmMachine.Status.Phase)
				return bmMachine.Status.Phase == clusterv1.MachinePhasePending
			}, timeout, time.Second).Should(BeTrue())
		})
	})

	Context("hetznerBareMetalMachine validation", func() {
		var capiMachine *clusterv1.Machine

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
			bmMachine.Spec.HostSelector.MatchLabels = map[string]string{
				"hcloud": "baremetalmachine-test",
			}

			key = client.ObjectKey{Namespace: testNs.Name, Name: bmMachineName}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, capiMachine, bmMachine)).To(Succeed())
		})

		Context("validate create", func() {
			It("should create the hetznerBareMetalMachine", func() {
				Expect(testEnv.Create(ctx, bmMachine)).To(Succeed())
			})

			It("should fail with a empty image name and path", func() {
				bmMachine.Spec.InstallImage.Image.Name = ""
				bmMachine.Spec.InstallImage.Image.Path = ""
				Expect(testEnv.Create(ctx, bmMachine)).NotTo(Succeed())
			})

			It("should fail with a empty image url and path", func() {
				bmMachine.Spec.InstallImage.Image.URL = ""
				bmMachine.Spec.InstallImage.Image.Path = ""
				Expect(testEnv.Create(ctx, bmMachine)).NotTo(Succeed())
			})

			It("should fail if image url has no suffix", func() {
				bmMachine.Spec.InstallImage.Image.URL = "https://hcloud.com/image/1111"
				Expect(testEnv.Create(ctx, bmMachine)).NotTo(Succeed())
			})

			It("should fail with a wrong host selector label", func() {
				bmMachine.Spec.HostSelector.MatchLabels = map[string]string{
					"cluster": "!test",
				}
				Expect(testEnv.Create(ctx, bmMachine)).NotTo(Succeed())
			})

			It("should fail with a wrong host selector label", func() {
				bmMachine.Spec.HostSelector.MatchExpressions = []infrav1.HostSelectorRequirement{
					{
						Key:      "Cluster",
						Operator: selection.Operator("WrongOperator"), // Invalid operator, should be one of In, NotIn, Exists, DoesNotExist
						Values:   []string{"test"},
					},
				}
				Expect(testEnv.Create(ctx, bmMachine)).NotTo(Succeed())
			})
		})

		Context("validate update", func() {
			BeforeEach(func() {
				Expect(testEnv.Create(ctx, bmMachine)).To(Succeed())

				Eventually(func() error {
					return testEnv.Get(ctx, key, bmMachine)
				}, timeout).Should(BeNil())
			})

			It("should fail updating installImage", func() {
				bmMachine.Spec.InstallImage.Image.Name = "ubuntu-2404-noble"
				Expect(testEnv.Update(ctx, bmMachine)).NotTo(Succeed())
			})

			It("should fail updating ssh spec", func() {
				bmMachine.Spec.SSHSpec.SecretRef.Name = "test-secret-ref-2"
				Expect(testEnv.Update(ctx, bmMachine)).NotTo(Succeed())
			})

			It("should fail updating matchLabels", func() {
				bmMachine.Spec.HostSelector.MatchLabels = map[string]string{
					"hcloud": "baremetalmachine-test-update",
				}
				Expect(testEnv.Update(ctx, bmMachine)).NotTo(Succeed())
			})
		})
		Context("validate update", func() {
			var (
				hbmmt  *infrav1.HetznerBareMetalMachineTemplate
				testNs *corev1.Namespace
			)
			BeforeEach(func() {
				var err error
				testNs, err = testEnv.CreateNamespace(ctx, "hcloudmachine-validation")
				Expect(err).NotTo(HaveOccurred())

				hbmmt = &infrav1.HetznerBareMetalMachineTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bmMachineName,
						Namespace: testNs.Name,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel: capiCluster.Name,
						},
					},
					Spec: infrav1.HetznerBareMetalMachineTemplateSpec{
						Template: infrav1.HetznerBareMetalMachineTemplateResource{
							Spec: getDefaultHetznerBareMetalMachineSpec(),
						},
					},
				}
				Expect(testEnv.Client.Create(ctx, hbmmt)).To(Succeed())

				key = client.ObjectKey{Namespace: testNs.Name, Name: hbmmt.Name}
				Eventually(func() error {
					return testEnv.Client.Get(ctx, key, hbmmt)
				}, timeout, time.Second).Should(BeNil())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, testNs, hbmmt)).To(Succeed())
			})

			It("should not allow update of InstallImage", func() {
				Expect(testEnv.Get(ctx, key, hbmmt)).To(Succeed())

				hbmmt.Spec = infrav1.HetznerBareMetalMachineTemplateSpec{
					Template: infrav1.HetznerBareMetalMachineTemplateResource{
						Spec: infrav1.HetznerBareMetalMachineSpec{
							InstallImage: infrav1.InstallImage{
								Image: infrav1.Image{
									Name: "ubuntu-24.04",
									URL:  "https://example.com/ubuntu-24.04.tar.gz",
								},
								Partitions: []infrav1.Partition{
									{
										Mount:      "/",
										FileSystem: "ext4",
										Size:       "10GiB",
									},
								},
							},
						},
					},
				}
				Expect(testEnv.Client.Update(ctx, hbmmt)).ToNot(Succeed())
			})

			It("should not allow update of SSHSpec", func() {
				Expect(testEnv.Get(ctx, key, hbmmt)).To(Succeed())

				hbmmt.Spec = infrav1.HetznerBareMetalMachineTemplateSpec{
					Template: infrav1.HetznerBareMetalMachineTemplateResource{
						Spec: infrav1.HetznerBareMetalMachineSpec{
							SSHSpec: infrav1.SSHSpec{
								SecretRef: infrav1.SSHSecretRef{
									Name: "ssh-secret-new",
									Key: infrav1.SSHSecretKeyRef{
										Name:       "ssh-key-name-new",
										PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
										PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
									},
								},
								PortAfterInstallImage: 2222,
							},
						},
					},
				}
				Expect(testEnv.Client.Update(ctx, hbmmt)).ToNot(Succeed())
			})

			It("should not allow update of Host Selectors", func() {
				Expect(testEnv.Get(ctx, key, hbmmt)).To(Succeed())

				hbmmt.Spec = infrav1.HetznerBareMetalMachineTemplateSpec{
					Template: infrav1.HetznerBareMetalMachineTemplateResource{
						Spec: infrav1.HetznerBareMetalMachineSpec{
							HostSelector: infrav1.HostSelector{
								MatchLabels: map[string]string{
									"key3": "value3",
								},
								MatchExpressions: []infrav1.HostSelectorRequirement{
									{
										Key:      "key4",
										Operator: selection.In,
										Values:   []string{"value4"},
									},
								},
							},
						},
					},
				}
				Expect(testEnv.Client.Update(ctx, hbmmt)).ToNot(Succeed())
			})

			It("should allow update of mutable fields", func() {
				Expect(testEnv.Get(ctx, key, hbmmt)).To(Succeed())

				if hbmmt.ObjectMeta.Annotations == nil {
					hbmmt.ObjectMeta.Annotations = make(map[string]string)
				}
				hbmmt.ObjectMeta.Annotations["test"] = "should_succeed"
				Expect(testEnv.Client.Update(ctx, hbmmt)).To(Succeed())
			})
		})
	})

	Context("Test EnqueueRequestsFromMapFunc BareMetalHostToBareMetalMachines", func() {
		It("should enqueue the second HetznerBareMetalMachine, when the first gets deleted", func() {
			// We test this part in BareMetalHostToBareMetalMachines:
			// We have a free host. Trigger a matching HetznerBareMetalMachine to be reconciled.
			capiMachine := &clusterv1.Machine{
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

			// Create a second Capi- and HetznerBareMetalMachine
			bmMachineName2 := "hbmm2"
			capiMachine2 := &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       utils.GenerateName(nil, "capimachine-name-"),
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
						Name:       bmMachineName2,
					},
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("bootstrap-secret"),
					},
					FailureDomain: &defaultFailureDomain,
				},
			}
			Expect(testEnv.Create(ctx, capiMachine2)).To(Succeed())

			bmMachine2 := &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bmMachineName2,
					Namespace: testNs.Name,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: capiCluster.Name,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Machine",
							Name:       capiMachine2.Name,
							UID:        capiMachine2.UID,
						},
					},
				},
				Spec: getDefaultHetznerBareMetalMachineSpec(),
			}
			Expect(testEnv.Create(ctx, bmMachine2)).To(Succeed())

			osSSHSecret := helpers.GetDefaultSSHSecret("os-ssh-secret", testNs.Name)
			Expect(testEnv.Create(ctx, osSSHSecret)).To(Succeed())

			host := helpers.BareMetalHost(
				hostName,
				testNs.Name,
				helpers.WithRootDeviceHintWWN(),
				helpers.WithHetznerClusterRef(hetznerClusterName),
			)
			Expect(testEnv.Create(ctx, host)).To(Succeed())

			// Wait until bmMachine is provisioned.
			var pickedMachine *infrav1.HetznerBareMetalMachine
			var waitingMachine *infrav1.HetznerBareMetalMachine

			// The mock should wait until we know which machine is picked.
			w := make(chan time.Time)
			osSSHClient.On("GetHostName").WaitUntil(w)

			Eventually(func() bool {
				if err := testEnv.Get(ctx, client.ObjectKeyFromObject(host), host); err != nil {
					return false
				}
				if host.Spec.ConsumerRef == nil {
					return false
				}

				if host.Spec.ConsumerRef.Name == bmMachineName {
					pickedMachine = bmMachine
					waitingMachine = bmMachine2
				} else {
					pickedMachine = bmMachine2
					waitingMachine = bmMachine
				}

				if w != nil {
					// We need to change the mock on-the-fly. There are two
					// machines, and we need to adapt the mock to the one that is
					// picked.
					osSSHClient.On("GetHostName").Return(sshclient.Output{
						StdOut: infrav1.BareMetalHostNamePrefix + pickedMachine.Name,
						StdErr: "",
						Err:    nil,
					})
					// Close the channel, so that mock "GetHostName" does no longer block.
					close(w)

					// We updated the mock, and closed the channel. Set channel to nil,
					// so that we don't mock and close again.
					// Closing a closed channel would panic.
					w = nil
				}

				return host.Spec.Status.ProvisioningState != infrav1.StateProvisioned
			}, timeout).Should(BeTrue())

			// Ensure the second machine is not ready (waiting for a hbmh)
			Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(waitingMachine),
				waitingMachine)).To(Succeed())
			Expect(waitingMachine.Status.Ready).To(BeFalse())

			// Delete the first machine
			Expect(testEnv.Delete(ctx, pickedMachine)).To(Succeed())

			// Wait until the waiting machine is ready
			Eventually(func() bool {
				Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(waitingMachine),
					waitingMachine)).To(Succeed())
				if waitingMachine.Status.Ready {
					return true
				}
				testEnv.GetLogger().Info("Waiting for machine to be ready", "status", waitingMachine.Status)
				return false
			}, timeout, interval).Should(BeTrue())
		})
	})
})

func Test_BareMetalHostToBareMetalMachines(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	utilruntime.Must(infrav1.AddToScheme(scheme))
	host := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-host",
			Namespace: "test-ns",
		},
	}
	hbmm := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine-with-label-foo",
			Namespace: "test-ns",
		},
		Spec: infrav1.HetznerBareMetalMachineSpec{
			HostSelector: infrav1.HostSelector{
				MatchLabels: map[string]string{
					"key": "foo",
				},
			},
		},
	}
	hbmmWithHostAnnotation := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine-with-host-annotation",
			Namespace: "test-ns",
			Annotations: map[string]string{
				infrav1.HostAnnotation: "test-host",
			},
		},
		Spec: infrav1.HetznerBareMetalMachineSpec{
			HostSelector: infrav1.HostSelector{
				MatchLabels: map[string]string{
					"key": "foo",
				},
			},
		},
	}

	hbmmWithoutLabel := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine-with-label-bar",
			Namespace: "test-ns",
		},
		Spec: infrav1.HetznerBareMetalMachineSpec{
			HostSelector: infrav1.HostSelector{
				MatchLabels: map[string]string{
					"key": "bar",
				},
			},
		},
	}
	hbmmOtherNS := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine-other-namespace",
			Namespace: "test-other-namespace",
		},
		Spec: infrav1.HetznerBareMetalMachineSpec{
			HostSelector: infrav1.HostSelector{
				MatchLabels: map[string]string{
					"key": "foo",
				},
			},
		},
	}
	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(
		host, hbmm, hbmmOtherNS, hbmmWithoutLabel, hbmmWithHostAnnotation).Build()

	// host does not have a label.
	f := BareMetalHostToBareMetalMachines(c, logr.Logger{})
	result := f(ctx, host)
	require.Nil(t, result)
	host.Labels = map[string]string{
		"key": "foo",
	}

	// host has label "foo"
	err := c.Update(ctx, host)
	require.NoError(t, err)
	result = f(ctx, host)
	require.Equal(t, []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "test-machine-with-label-foo",
				Namespace: "test-ns",
			},
		},
	}, result)
}
