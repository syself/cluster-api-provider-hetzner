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

package baremetal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
)

var _ = Describe("chooseHost", func() {
	const defaultNamespace = "default"

	bmMachine := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bm-machine",
			Namespace: defaultNamespace,
		},
	}

	hostWithCorrectConsumerRef := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithCorrectConsumerRef",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			ConsumerRef: &corev1.ObjectReference{
				Name:      "bm-machine",
				Namespace: defaultNamespace,
			},
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	hostWithIncorrectConsumerRef := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithIncorrectConsumerRef",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			ConsumerRef: &corev1.ObjectReference{
				Name:       "bm-machine-other",
				Namespace:  defaultNamespace,
				Kind:       "HetznerBareMetalMachine",
				APIVersion: infrav1.GroupVersion.String(),
			},
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	maintenanceMode := true
	hostInMaintenanceMode := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostInMaintenanceMode",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			MaintenanceMode: &maintenanceMode,
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	now := metav1.Now()
	hostWithDeletionTimeStamp := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "hostWithDeletionTimeStamp",
			Namespace:         defaultNamespace,
			DeletionTimestamp: &now,
			Finalizers:        []string{"finalizer"},
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	hostWithErrorMessage := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithErrorMessage",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				ErrorMessage:      "some error",
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	hostWithStateRegistering := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithStateRegistering",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				ErrorMessage:      "some error",
				ProvisioningState: infrav1.StateRegistering,
			},
		},
	}

	hostWithOtherLabel := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithOtherLabel",
			Namespace: defaultNamespace,
			Labels:    map[string]string{"wrong": "label"},
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	hostWithOtherNamespace := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithOtherNamespace",
			Namespace: "other-ns",
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	host := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	hostWithRaidWwnConfig := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithRaidWwnConfig",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			RootDeviceHints: &infrav1.RootDeviceHints{
				WWN: "",
				Raid: infrav1.Raid{
					WWN: []string{"wwnRaid1", "wwnRaid2"},
				},
			},
		},
	}
	hostWithNonRaidWwnConfig := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithNonRaidWwnConfig",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			RootDeviceHints: &infrav1.RootDeviceHints{
				WWN: "wwnNoRaid",
			},
		},
	}

	hostWithLabel := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithLabel",
			Namespace: defaultNamespace,
			Labels:    map[string]string{"key": "value"},
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	hostWithLabelAndMaintenanceMode := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithLabelAndMaintenanceMode",
			Namespace: defaultNamespace,
			Labels:    map[string]string{"key": "value"},
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			MaintenanceMode: &maintenanceMode,
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	type testCaseChooseHost struct {
		Hosts            []client.Object
		HostSelector     infrav1.HostSelector
		ExpectedHostName string
		RootDeviceHints  infrav1.RootDeviceHints
	}
	DescribeTable("chooseHost",
		func(tc testCaseChooseHost) {
			scheme := runtime.NewScheme()
			utilruntime.Must(infrav1.AddToScheme(scheme))
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(tc.Hosts...).Build()
			bmMachine.Spec.HostSelector = tc.HostSelector
			service := newTestService(bmMachine, c)

			hosts := &infrav1.HetznerBareMetalHostList{}
			err := service.scope.Client.List(context.TODO(), hosts,
				client.InNamespace(service.scope.BareMetalMachine.Namespace))
			Expect(err).To(Succeed())

			host, reason, err := ChooseHost(bmMachine, hosts.Items)
			Expect(err).To(Succeed())
			Expect(reason).To(Equal(""))
			if tc.ExpectedHostName == "" {
				Expect(host).To(BeNil())
			} else {
				Expect(host).ToNot(BeNil())
				Expect(host.Name).To(Equal(tc.ExpectedHostName))
			}
		},
		Entry("No host in maintenance mode",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostInMaintenanceMode, &host},
				ExpectedHostName: "host",
			}),
		Entry("No host with deletion timestamp",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithDeletionTimeStamp, &host},
				ExpectedHostName: "host",
			}),
		Entry("No host with error message",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithErrorMessage, &host},
				ExpectedHostName: "host",
			}),
		Entry("No host with incorrect consumer ref",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithIncorrectConsumerRef, &host},
				ExpectedHostName: "host",
			}),
		Entry("No host with other namespace",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithOtherNamespace, &host},
				ExpectedHostName: "host",
			}),
		Entry("No host with state other than available",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithStateRegistering, &host},
				ExpectedHostName: "host",
			}),
		Entry("Choosing host with consumer ref",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithCorrectConsumerRef, &host},
				ExpectedHostName: "hostWithCorrectConsumerRef",
			}),
		Entry("Choosing host with right label",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithLabel, &hostWithOtherLabel, &hostWithLabelAndMaintenanceMode, &host},
				HostSelector:     infrav1.HostSelector{MatchLabels: map[string]string{"key": "value"}},
				ExpectedHostName: "hostWithLabel",
			}),
		Entry("Choosing host with right label through MatchExpressions",
			testCaseChooseHost{
				Hosts: []client.Object{&hostWithLabel, &hostWithOtherLabel, &hostWithLabelAndMaintenanceMode, &host},
				HostSelector: infrav1.HostSelector{MatchExpressions: []infrav1.HostSelectorRequirement{
					{Key: "key", Operator: selection.In, Values: []string{"value", "value2"}},
				}},
				ExpectedHostName: "hostWithLabel",
			}),
	)

	type testCaseChooseHostWithReason struct {
		hosts            []client.Object
		expectedHostName string
		expectedReason   string
		swraid           int
	}

	DescribeTable("chooseHost(): Test with reason, because RAID config does not match.",
		func(tc testCaseChooseHostWithReason) {
			scheme := runtime.NewScheme()
			utilruntime.Must(infrav1.AddToScheme(scheme))
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(tc.hosts...).Build()
			bmMachine := &infrav1.HetznerBareMetalMachine{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{Name: "bmMachine", Namespace: defaultNamespace},
				Spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Swraid: tc.swraid,
					},
				},
			}
			service := newTestService(bmMachine, c)

			hosts := &infrav1.HetznerBareMetalHostList{}
			err := service.scope.Client.List(context.TODO(), hosts,
				client.InNamespace(service.scope.BareMetalMachine.Namespace))
			Expect(err).To(Succeed())
			host, reason, err := ChooseHost(bmMachine, hosts.Items)

			Expect(err).To(Succeed())
			Expect(reason).To(Equal(tc.expectedReason))
			Expect(host).To(BeNil())
		},
		Entry("No host, because invalid RAID config (want RAID)",
			testCaseChooseHostWithReason{
				hosts:            []client.Object{&hostWithNonRaidWwnConfig},
				expectedHostName: "",
				expectedReason:   "No available host of 1 found: machine-should-use-swraid-but-not-enough-RAID-WWNs-in-hbmh: 1",
				swraid:           1,
			}),
		Entry("No host, because invalid RAID config (want no RAID)",
			testCaseChooseHostWithReason{
				hosts:            []client.Object{&hostWithRaidWwnConfig},
				expectedHostName: "",
				expectedReason:   "No available host of 1 found: machine-should-use-no-swraid-and-no-non-raid-WWN-in-hbmh: 1",
				swraid:           0,
			}),
	)
})

var _ = Describe("Test NodeAddresses", func() {
	nic1 := infrav1.NIC{
		IP: "192.168.1.1",
	}

	nic2 := infrav1.NIC{
		IP: "172.16.20.2",
	}

	nic3 := infrav1.NIC{
		IP: "203.0.113.5",
	}

	addr1 := clusterv1beta1.MachineAddress{
		Type:    clusterv1beta1.MachineInternalIP,
		Address: "192.168.1.1",
	}

	addr2 := clusterv1beta1.MachineAddress{
		Type:    clusterv1beta1.MachineInternalIP,
		Address: "172.16.20.2",
	}

	addr3 := clusterv1beta1.MachineAddress{
		Type:    clusterv1beta1.MachineHostName,
		Address: "bm-machine",
	}

	addr4 := clusterv1beta1.MachineAddress{
		Type:    clusterv1beta1.MachineInternalDNS,
		Address: "bm-machine",
	}

	addr5 := clusterv1beta1.MachineAddress{
		Type:    clusterv1beta1.MachineExternalIP,
		Address: "203.0.113.5",
	}

	type testCaseNodeAddress struct {
		Machine               clusterv1.Machine
		BareMetalMachine      infrav1.HetznerBareMetalMachine
		Host                  *infrav1.HetznerBareMetalHost
		ExpectedNodeAddresses []clusterv1beta1.MachineAddress
	}

	DescribeTable("Test NodeAddress",
		func(tc testCaseNodeAddress) {
			nodeAddresses := nodeAddresses(tc.Host, "bm-machine")
			for i, address := range tc.ExpectedNodeAddresses {
				Expect(nodeAddresses[i]).To(Equal(address))
			}
		},
		Entry("One NIC", testCaseNodeAddress{
			Host: &infrav1.HetznerBareMetalHost{
				Spec: infrav1.HetznerBareMetalHostSpec{
					Status: infrav1.ControllerGeneratedStatus{
						HardwareDetails: &infrav1.HardwareDetails{
							NIC: []infrav1.NIC{nic1},
						},
					},
				},
			},
			ExpectedNodeAddresses: []clusterv1beta1.MachineAddress{addr1, addr3, addr4},
		}),
		Entry("Two NICs", testCaseNodeAddress{
			Host: &infrav1.HetznerBareMetalHost{
				Spec: infrav1.HetznerBareMetalHostSpec{
					Status: infrav1.ControllerGeneratedStatus{
						HardwareDetails: &infrav1.HardwareDetails{
							NIC: []infrav1.NIC{nic1, nic2},
						},
					},
				},
			},
			ExpectedNodeAddresses: []clusterv1beta1.MachineAddress{addr1, addr2, addr3, addr4},
		}),
		Entry("Public NIC IP is reported as ExternalIP", testCaseNodeAddress{
			Host: &infrav1.HetznerBareMetalHost{
				Spec: infrav1.HetznerBareMetalHostSpec{
					Status: infrav1.ControllerGeneratedStatus{
						HardwareDetails: &infrav1.HardwareDetails{
							NIC: []infrav1.NIC{nic3},
						},
					},
				},
			},
			ExpectedNodeAddresses: []clusterv1beta1.MachineAddress{addr5, addr3, addr4},
		}),
	)
})

var _ = Describe("Test consumerRefMatches", func() {
	type testCaseConsumerRefMatches struct {
		Consumer       *corev1.ObjectReference
		ExpectedResult bool
	}

	bmMachine := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bm-machine",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "HetznerBareMetalMachine",
			APIVersion: "v1beta1",
		},
	}
	DescribeTable("Test consumerRefMatches",
		func(tc testCaseConsumerRefMatches) {
			Expect(consumerRefMatches(tc.Consumer, bmMachine)).To(Equal(tc.ExpectedResult))
		},
		Entry("Matching consumer", testCaseConsumerRefMatches{
			Consumer: &corev1.ObjectReference{
				Name:       "bm-machine",
				Namespace:  "default",
				Kind:       "HetznerBareMetalMachine",
				APIVersion: "v1beta1",
			},
			ExpectedResult: true,
		}),
		Entry("No matching name", testCaseConsumerRefMatches{
			Consumer: &corev1.ObjectReference{
				Name:       "other-bm-machine",
				Namespace:  "default",
				Kind:       "HetznerBareMetalMachine",
				APIVersion: "v1beta1",
			},
			ExpectedResult: false,
		}),
		Entry("No matching namespace", testCaseConsumerRefMatches{
			Consumer: &corev1.ObjectReference{
				Name:       "bm-machine",
				Namespace:  "other",
				Kind:       "HetznerBareMetalMachine",
				APIVersion: "v1beta1",
			},
			ExpectedResult: false,
		}),
		Entry("No matching kind", testCaseConsumerRefMatches{
			Consumer: &corev1.ObjectReference{
				Name:       "bm-machine",
				Namespace:  "default",
				Kind:       "OtherBareMetalMachine",
				APIVersion: "v1beta1",
			},
			ExpectedResult: false,
		}),
		Entry("No matching apiversion", testCaseConsumerRefMatches{
			Consumer: &corev1.ObjectReference{
				Name:       "bm-machine",
				Namespace:  "default",
				Kind:       "HetznerBareMetalMachine",
				APIVersion: "hetzner/v1beta",
			},
			ExpectedResult: false,
		}),
	)
})

var _ = Describe("Test setOwnerRefInList", func() {
	type testCaseSetOwnerRefInList struct {
		RefList         []metav1.OwnerReference
		ExpectedRefList []metav1.OwnerReference
	}

	objectMeta := metav1.ObjectMeta{
		Name:      "bm-machine",
		Namespace: "default",
	}
	objectType := metav1.TypeMeta{
		Kind:       "HetznerBareMetalMachine",
		APIVersion: "v1beta1",
	}

	DescribeTable("Test setOwnerRefInList",
		func(tc testCaseSetOwnerRefInList) {
			refList := setOwnerRefInList(tc.RefList, objectType, objectMeta)
			Expect(refList).To(Equal(tc.ExpectedRefList))
		},
		Entry("List of one non-matching entry", testCaseSetOwnerRefInList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedRefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
					Controller: ptr.To(true),
				},
			},
		}),
		Entry("List of one matching entry", testCaseSetOwnerRefInList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedRefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
					Controller: ptr.To(true),
				},
			},
		}),
		Entry("List of two non-matching entries", testCaseSetOwnerRefInList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "new-bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta2",
				},
			},
			ExpectedRefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "new-bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta2",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
					Controller: ptr.To(true),
				},
			},
		}),
	)
})

var _ = Describe("Test ensureMachineAnnotation", func() {
	type testCaseEnsureMachineyyAnnotation struct {
		Annotations         map[string]string
		ExpectedAnnotations map[string]string
	}

	DescribeTable("Test ensureMachineAnnotation",
		func(tc testCaseEnsureMachineyyAnnotation) {
			bmMachine := &infrav1.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "bm-machine",
					Namespace:   "default",
					Annotations: tc.Annotations,
				},
			}
			service := newTestService(bmMachine, nil)

			host := infrav1.HetznerBareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hostName",
					Namespace: "default",
				},
			}

			service.ensureMachineAnnotation(&host)
			Expect(bmMachine.GetAnnotations()).Should(Equal(tc.ExpectedAnnotations))
		},
		Entry("List of one non-matching entry", testCaseEnsureMachineyyAnnotation{
			Annotations:         map[string]string{"key1": "val1"},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav1.HostAnnotation: "default/hostName"},
		}),
		Entry("Empty list", testCaseEnsureMachineyyAnnotation{
			Annotations:         map[string]string{},
			ExpectedAnnotations: map[string]string{infrav1.HostAnnotation: "default/hostName"},
		}),
		Entry("Nil list", testCaseEnsureMachineyyAnnotation{
			Annotations:         nil,
			ExpectedAnnotations: map[string]string{infrav1.HostAnnotation: "default/hostName"},
		}),
		Entry("List of one non-matching and one matching entry", testCaseEnsureMachineyyAnnotation{
			Annotations:         map[string]string{"key1": "val1", infrav1.HostAnnotation: "default/hostName"},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav1.HostAnnotation: "default/hostName"},
		}),
	)
})

var _ = Describe("Test updateHostAnnotation", func() {
	type testCaseUpdateHostAnnotation struct {
		Annotations         map[string]string
		ExpectedAnnotations map[string]string
	}

	const hostKey = "default/hostName"

	DescribeTable("Test updateHostAnnotation",
		func(tc testCaseUpdateHostAnnotation) {
			updatedAnnotations := updateHostAnnotation(tc.Annotations, hostKey, logr.Discard())
			Expect(updatedAnnotations).Should(Equal(tc.ExpectedAnnotations))
		},
		Entry("List of one non-matching entry", testCaseUpdateHostAnnotation{
			Annotations:         map[string]string{"key1": "val1"},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav1.HostAnnotation: hostKey},
		}),
		Entry("Empty list", testCaseUpdateHostAnnotation{
			Annotations:         map[string]string{},
			ExpectedAnnotations: map[string]string{infrav1.HostAnnotation: hostKey},
		}),
		Entry("Nil list", testCaseUpdateHostAnnotation{
			Annotations:         nil,
			ExpectedAnnotations: map[string]string{infrav1.HostAnnotation: hostKey},
		}),
		Entry("List of one non-matching and one matching entry", testCaseUpdateHostAnnotation{
			Annotations:         map[string]string{"key1": "val1", infrav1.HostAnnotation: hostKey},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav1.HostAnnotation: hostKey},
		}),
	)
})

var _ = Describe("Test ensureClusterLabel", func() {
	type testCaseEnsureClusterLabel struct {
		labels         map[string]string
		expectedLabels map[string]string
	}

	const clusterName = "clusterName"

	DescribeTable("Test ensureClusterLabel",
		func(tc testCaseEnsureClusterLabel) {
			host := &infrav1.HetznerBareMetalHost{}
			host.Labels = tc.labels

			ensureClusterLabel(host, clusterName)

			Expect(host.Labels).Should(Equal(tc.expectedLabels))
		},
		Entry("Existing labels", testCaseEnsureClusterLabel{
			labels:         map[string]string{"key1": "val1"},
			expectedLabels: map[string]string{"key1": "val1", clusterv1.ClusterNameLabel: clusterName},
		}),
		Entry("Empty labels", testCaseEnsureClusterLabel{
			labels:         map[string]string{},
			expectedLabels: map[string]string{clusterv1.ClusterNameLabel: clusterName},
		}),
		Entry("Nil labels", testCaseEnsureClusterLabel{
			labels:         nil,
			expectedLabels: map[string]string{clusterv1.ClusterNameLabel: clusterName},
		}),
	)
})

var _ = Describe("Test hostKey", func() {
	host := &infrav1.HetznerBareMetalHost{}
	host.Namespace = "namespace"
	host.Name = "name"

	Expect(hostKey(host)).To(Equal("namespace/name"))
})

var _ = Describe("Test checkForRequeueError", func() {
	type testCaseCheckForRequeueError struct {
		err            error
		expectedResult reconcile.Result
		expectedErrMsg string
	}

	DescribeTable("Test ensureClusterLabel",
		func(tc testCaseCheckForRequeueError) {
			errMsg := "test message"
			res, err := checkForRequeueError(tc.err, errMsg)

			if tc.expectedErrMsg == "" {
				Expect(err).To(BeNil())
			} else {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(tc.expectedErrMsg))
			}

			Expect(res).To(Equal(tc.expectedResult))
		},
		Entry("Nil error", testCaseCheckForRequeueError{
			err:            nil,
			expectedResult: reconcile.Result{},
			expectedErrMsg: "",
		}),
		Entry("Requeue error", testCaseCheckForRequeueError{
			err:            &scope.RequeueAfterError{RequeueAfter: 30 * time.Second},
			expectedResult: reconcile.Result{Requeue: true, RequeueAfter: 30 * time.Second},
			expectedErrMsg: "",
		}),
		Entry("Other error", testCaseCheckForRequeueError{
			err:            fmt.Errorf("other error"),
			expectedResult: reconcile.Result{},
			expectedErrMsg: "test message: other error",
		}),
	)
})

var _ = Describe("Test analyzePatchError", func() {
	type testCaseAnalyzePatchError struct {
		ignoreNotFound bool
		err            error
		expectedErr    error
	}

	groupResource := schema.GroupResource{Group: "testgroup", Resource: "testresource"}

	DescribeTable("Test analyzePatchError",
		func(tc testCaseAnalyzePatchError) {
			err := analyzePatchError(tc.err, tc.ignoreNotFound)

			// must not compare nil with nil
			if tc.expectedErr == nil {
				Expect(err).To(BeNil())
			} else {
				Expect(err).To(Equal(tc.expectedErr))
			}
		},
		Entry("Nil error", testCaseAnalyzePatchError{
			ignoreNotFound: false,
			err:            nil,
			expectedErr:    nil,
		}),
		Entry("Not found", testCaseAnalyzePatchError{
			ignoreNotFound: true,
			err:            apierrors.NewNotFound(groupResource, "groupResource"),
			expectedErr:    nil,
		}),
		Entry("Not found but do not ignore it", testCaseAnalyzePatchError{
			ignoreNotFound: false,
			err:            apierrors.NewNotFound(groupResource, "groupResource"),
			expectedErr:    apierrors.NewNotFound(groupResource, "groupResource"),
		}),
		Entry("Conflict error", testCaseAnalyzePatchError{
			ignoreNotFound: true,
			err:            apierrors.NewConflict(groupResource, "groupResource", fmt.Errorf("conflict error")),
			expectedErr:    &scope.RequeueAfterError{},
		}),
		Entry("Conflict error without ignoring not found", testCaseAnalyzePatchError{
			ignoreNotFound: false,
			err:            apierrors.NewConflict(groupResource, "groupResource", fmt.Errorf("conflict error")),
			expectedErr:    &scope.RequeueAfterError{},
		}),
	)
})

var _ = Describe("Test GenerateProviderID", func() {
	type testCaseGenerateProviderID struct {
		hetznerCluster     *infrav1.HetznerCluster
		serverNumber       int
		expectedProviderID string
	}

	newHetznerCluster := func() *infrav1.HetznerCluster {
		return &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-hetzner-cluster",
				Annotations: map[string]string{},
			},
		}
	}

	DescribeTable("GenerateProviderID",
		func(tc testCaseGenerateProviderID) {
			providerID := generateProviderID(tc.hetznerCluster, tc.serverNumber)
			Expect(providerID).To(Equal(tc.expectedProviderID))
		},

		Entry("Defaults to legacy prefix", testCaseGenerateProviderID{
			hetznerCluster:     newHetznerCluster(),
			serverNumber:       7,
			expectedProviderID: "hcloud://bm-7",
		}),
		Entry("Uses annotation prefix", testCaseGenerateProviderID{
			hetznerCluster: func() *infrav1.HetznerCluster {
				hetznerCluster := newHetznerCluster()
				hetznerCluster.Annotations = map[string]string{
					infrav1.UseHrobotProviderIDForBaremetalAnnotation: "true",
				}
				return hetznerCluster
			}(),
			serverNumber:       11,
			expectedProviderID: "hrobot://11",
		}),
		Entry("Uses legacy prefix for non-true annotation value", testCaseGenerateProviderID{
			hetznerCluster: func() *infrav1.HetznerCluster {
				hetznerCluster := newHetznerCluster()
				hetznerCluster.Annotations = map[string]string{
					infrav1.UseHrobotProviderIDForBaremetalAnnotation: "invalid",
				}
				return hetznerCluster
			}(),
			serverNumber:       5,
			expectedProviderID: "hcloud://bm-5",
		}),
	)
})

var _ = Describe("reconcileLoadBalancerAttachment", func() {
	newServiceForLoadBalancerAttachment := func(
		machine *clusterv1.Machine,
		bareMetalMachine *infrav1.HetznerBareMetalMachine,
		cluster *clusterv1.Cluster,
		hetznerCluster *infrav1.HetznerCluster,
		hcloudClient *mocks.Client,
	) *Service {
		return &Service{
			scope: &scope.BareMetalMachineScope{
				Machine:          machine,
				BareMetalMachine: bareMetalMachine,
				Cluster:          cluster,
				HetznerCluster:   hetznerCluster,
				HCloudClient:     hcloudClient,
			},
		}
	}

	newControlPlaneCluster := func() *clusterv1.Cluster {
		return &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: clusterv1.ContractVersionedObjectReference{Kind: "KubeadmControlPlane"},
			},
		}
	}

	newHost := func(ipv4 string) *infrav1.HetznerBareMetalHost {
		return &infrav1.HetznerBareMetalHost{
			Spec: infrav1.HetznerBareMetalHostSpec{
				ServerID: 42,
				Status: infrav1.ControllerGeneratedStatus{
					IPv4: ipv4,
				},
			},
		}
	}

	It("requeues when another control-plane target exists and the api server pod is not healthy", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		machine := &clusterv1.Machine{}
		conditions.Set(machine, metav1.Condition{
			Type:    controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "PodNotHealthy",
			Message: "kube-apiserver is still starting",
		})

		bareMetalMachine := &infrav1.HetznerBareMetalMachine{}
		hetznerCluster := &infrav1.HetznerCluster{
			Status: infrav1.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav1.LoadBalancerStatus{
					ID: 123,
					Target: []infrav1.LoadBalancerTarget{
						{Type: infrav1.LoadBalancerTargetTypeIP, IP: "192.0.2.9"},
					},
				},
			},
		}

		hcloudClient.On("ListLoadBalancers", mock.Anything, mock.Anything).Return([]*hcloud.LoadBalancer{
			{
				ID: 123,
				Targets: []hcloud.LoadBalancerTarget{
					{
						Type: hcloud.LoadBalancerTargetTypeIP,
						IP:   &hcloud.LoadBalancerTargetIP{IP: "192.0.2.9"},
					},
				},
			},
		}, nil).Once()

		service := newServiceForLoadBalancerAttachment(machine, bareMetalMachine, newControlPlaneCluster(), hetznerCluster, hcloudClient)

		err := service.reconcileLoadBalancerAttachment(context.Background(), newHost("192.0.2.10"))
		var requeueErr *scope.RequeueAfterError
		Expect(errors.As(err, &requeueErr)).To(BeTrue())
		Expect(requeueErr.GetRequeueAfter()).To(Equal(requeueAfter))
		Expect(v1beta1conditions.IsFalse(bareMetalMachine, infrav1.ServerAvailableCondition)).To(BeTrue())
		Expect(v1beta1conditions.GetReason(bareMetalMachine, infrav1.ServerAvailableCondition)).To(Equal("WaitingForAPIServer"))
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "AddIPTargetToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
	})

	It("allows the first control-plane target even if the api server pod is not healthy yet", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		machine := &clusterv1.Machine{}
		conditions.Set(machine, metav1.Condition{
			Type:    controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "PodNotHealthy",
			Message: "kube-apiserver is still starting",
		})

		bareMetalMachine := &infrav1.HetznerBareMetalMachine{}
		hetznerCluster := &infrav1.HetznerCluster{
			Status: infrav1.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav1.LoadBalancerStatus{
					ID: 123,
				},
			},
		}

		hcloudClient.On("ListLoadBalancers", mock.Anything, mock.Anything).Return([]*hcloud.LoadBalancer{
			{
				ID: 123,
			},
		}, nil).Once()

		hcloudClient.On(
			"AddIPTargetToLoadBalancer",
			mock.Anything,
			mock.MatchedBy(func(opts hcloud.LoadBalancerAddIPTargetOpts) bool {
				return opts.IP.String() == "192.0.2.10"
			}),
			mock.MatchedBy(func(lb *hcloud.LoadBalancer) bool {
				return lb.ID == 123
			}),
		).Return(nil).Once()

		service := newServiceForLoadBalancerAttachment(machine, bareMetalMachine, newControlPlaneCluster(), hetznerCluster, hcloudClient)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHost("192.0.2.10"))).To(Succeed())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
	})

	It("does not list LoadBalancers via Hetzner API, when ServerAvailable condition is marked true", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		machine := &clusterv1.Machine{}
		conditions.Set(machine, metav1.Condition{
			Type:    controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "PodNotHealthy",
			Message: "kube-apiserver is still starting",
		})

		bareMetalMachine := &infrav1.HetznerBareMetalMachine{}
		v1beta1conditions.MarkTrue(bareMetalMachine, infrav1.ServerAvailableCondition)

		hetznerCluster := &infrav1.HetznerCluster{
			Status: infrav1.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav1.LoadBalancerStatus{
					ID: 123,
				},
			},
		}

		// It should add the load balancer target via Hetzner API.
		// But it should not call the Hetzner API to list load balancers, instead it should utilize
		// the HetznerCluster.Status to find out which targets should be added to the load balancer.
		hcloudClient.On(
			"AddIPTargetToLoadBalancer",
			mock.Anything,
			mock.MatchedBy(func(opts hcloud.LoadBalancerAddIPTargetOpts) bool {
				return opts.IP.String() == "192.0.2.10"
			}),
			mock.MatchedBy(func(lb *hcloud.LoadBalancer) bool {
				return lb.ID == 123
			}),
		).Return(nil).Once()

		service := newServiceForLoadBalancerAttachment(machine, bareMetalMachine, newControlPlaneCluster(), hetznerCluster, hcloudClient)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHost("192.0.2.10"))).To(Succeed())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "ListLoadBalancers", mock.Anything, mock.Anything)).To(BeTrue())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
	})
})

var _ = Describe("Reconcile with control-plane load balancer attachment", func() {
	const (
		testNamespace = "default"
		testHostName  = "bm-host"
		testBMMName   = "bm-machine"
		testCluster   = "test-cluster"
	)

	buildService := func(lbTargets []infrav1.LoadBalancerTarget, apiServerHealthy, isControlPlane bool) (
		*Service, *infrav1.HetznerBareMetalMachine, *mocks.Client,
	) {
		scheme := runtime.NewScheme()
		utilruntime.Must(infrav1.AddToScheme(scheme))
		utilruntime.Must(clusterv1.AddToScheme(scheme))
		utilruntime.Must(corev1.AddToScheme(scheme))

		host := &infrav1.HetznerBareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: testHostName, Namespace: testNamespace},
			Spec: infrav1.HetznerBareMetalHostSpec{
				ServerID: 42,
				Status: infrav1.ControllerGeneratedStatus{
					IPv4:              "192.0.2.10",
					ProvisioningState: infrav1.StateProvisioned,
				},
			},
		}

		bareMetalMachine := &infrav1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testBMMName,
				Namespace: testNamespace,
				Annotations: map[string]string{
					infrav1.HostAnnotation: testNamespace + "/" + testHostName,
				},
			},
		}

		machineLabels := map[string]string{}
		if isControlPlane {
			machineLabels[clusterv1.MachineControlPlaneLabel] = ""
		}
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testBMMName,
				Namespace: testNamespace,
				Labels:    machineLabels,
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-data"),
				},
				ClusterName: testCluster,
			},
		}
		if apiServerHealthy {
			conditions.Set(machine, metav1.Condition{
				Type:   controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
				Status: metav1.ConditionTrue,
				Reason: "Healthy",
			})
		} else {
			conditions.Set(machine, metav1.Condition{
				Type:    controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  "PodNotHealthy",
				Message: "kube-apiserver is still starting",
			})
		}

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: testCluster, Namespace: testNamespace},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: clusterv1.ContractVersionedObjectReference{Kind: "KubeadmControlPlane"},
			},
		}

		hetznerCluster := &infrav1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{Name: testCluster, Namespace: testNamespace},
			Status: infrav1.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav1.LoadBalancerStatus{
					ID:     123,
					Target: lbTargets,
				},
			},
		}

		c := fakeclient.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(host, bareMetalMachine, machine).
			Build()

		hcloudClient := mocks.NewClient(GinkgoT())

		service := &Service{
			scope: &scope.BareMetalMachineScope{
				Logger:           log,
				Client:           c,
				Cluster:          cluster,
				Machine:          machine,
				BareMetalMachine: bareMetalMachine,
				HetznerCluster:   hetznerCluster,
				HCloudClient:     hcloudClient,
			},
		}

		return service, bareMetalMachine, hcloudClient
	}

	It("keeps ProviderID and Ready set when reconcileLoadBalancerAttachment requeues for WaitingForAPIServer", func() {
		// Existing LB target plus an unhealthy kube-apiserver pod makes
		// reconcileLoadBalancerAttachment return a RequeueAfterError and mark
		// ServerAvailableCondition=False/WaitingForAPIServer. Reconcile must
		// still set Ready=true and ProviderID so CAPI can copy ProviderID onto
		// the core Machine - otherwise MachineAPIServerPodHealthy never flips
		// true and the attachment requeues forever (bootstrap deadlock).
		service, bareMetalMachine, hcloudClient := buildService(
			[]infrav1.LoadBalancerTarget{
				{Type: infrav1.LoadBalancerTargetTypeIP, IP: "192.0.2.9"},
			},
			false,
			true,
		)

		hcloudClient.On("ListLoadBalancers", mock.Anything, mock.Anything).Return([]*hcloud.LoadBalancer{
			{
				ID: 123,
				Targets: []hcloud.LoadBalancerTarget{
					{
						Type: hcloud.LoadBalancerTargetTypeIP,
						IP:   &hcloud.LoadBalancerTargetIP{IP: "192.0.2.9"},
					},
				},
			},
		}, nil).Once()

		res, err := service.Reconcile(context.Background())
		Expect(err).To(BeNil())
		Expect(res.RequeueAfter).To(Equal(requeueAfter))

		Expect(bareMetalMachine.Status.Ready).To(BeTrue())
		Expect(bareMetalMachine.Spec.ProviderID).NotTo(BeNil())
		Expect(*bareMetalMachine.Spec.ProviderID).NotTo(BeEmpty())
		Expect(isPresentAndFalseWithReason(bareMetalMachine, infrav1.ServerAvailableCondition, "WaitingForAPIServer")).To(BeTrue())
	})

	It("does not requeue on the happy path and marks ServerAvailableCondition=True", func() {
		// LB target list already contains this host's IPv4, so
		// reconcileLoadBalancerAttachment returns no requeue and Reconcile
		// marks the condition true.
		service, bareMetalMachine, hcloudClient := buildService(
			[]infrav1.LoadBalancerTarget{
				{Type: infrav1.LoadBalancerTargetTypeIP, IP: "192.0.2.10"},
			},
			true,
			true,
		)

		hcloudClient.On("ListLoadBalancers", mock.Anything, mock.Anything).Return([]*hcloud.LoadBalancer{
			{
				ID: 123,
				Targets: []hcloud.LoadBalancerTarget{
					{
						Type: hcloud.LoadBalancerTargetTypeIP,
						IP:   &hcloud.LoadBalancerTargetIP{IP: "192.0.2.10"},
					},
				},
			},
		}, nil).Once()

		res, err := service.Reconcile(context.Background())
		Expect(err).To(BeNil())
		Expect(res).To(Equal(reconcile.Result{}))

		Expect(bareMetalMachine.Status.Ready).To(BeTrue())
		Expect(bareMetalMachine.Spec.ProviderID).NotTo(BeNil())
		Expect(v1beta1conditions.IsTrue(bareMetalMachine, infrav1.ServerAvailableCondition)).To(BeTrue())
	})

	It("marks ServerAvailableCondition=True for worker nodes without touching the load balancer", func() {
		// Worker nodes never hit reconcileLoadBalancerAttachment, but Reconcile
		// must still mark the condition true so the condition is meaningful on
		// non-control-plane HetznerBareMetalMachines too.
		service, bareMetalMachine, _ := buildService(nil, true, false)

		res, err := service.Reconcile(context.Background())
		Expect(err).To(BeNil())
		Expect(res).To(Equal(reconcile.Result{}))

		Expect(bareMetalMachine.Status.Ready).To(BeTrue())
		Expect(bareMetalMachine.Spec.ProviderID).NotTo(BeNil())
		Expect(v1beta1conditions.IsTrue(bareMetalMachine, infrav1.ServerAvailableCondition)).To(BeTrue())
	})
})

func isPresentAndFalseWithReason(getter v1beta1conditions.Getter, condition clusterv1beta1.ConditionType, reason string) bool {
	if !v1beta1conditions.Has(getter, condition) {
		return false
	}
	objectCondition := v1beta1conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionFalse &&
		objectCondition.Reason == reason
}
