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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
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
		IP: "172.0.20.2",
	}

	addr1 := clusterv1.MachineAddress{
		Type:    clusterv1.MachineInternalIP,
		Address: "192.168.1.1",
	}

	addr2 := clusterv1.MachineAddress{
		Type:    clusterv1.MachineInternalIP,
		Address: "172.0.20.2",
	}

	addr3 := clusterv1.MachineAddress{
		Type:    clusterv1.MachineHostName,
		Address: "bm-machine",
	}

	addr4 := clusterv1.MachineAddress{
		Type:    clusterv1.MachineInternalDNS,
		Address: "bm-machine",
	}

	type testCaseNodeAddress struct {
		Machine               clusterv1.Machine
		BareMetalMachine      infrav1.HetznerBareMetalMachine
		Host                  *infrav1.HetznerBareMetalHost
		ExpectedNodeAddresses []clusterv1.MachineAddress
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
			ExpectedNodeAddresses: []clusterv1.MachineAddress{addr1, addr3, addr4},
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
			ExpectedNodeAddresses: []clusterv1.MachineAddress{addr1, addr2, addr3, addr4},
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

var _ = Describe("Test providerIDFromServerID", func() {
	Expect(providerIDFromServerID(42)).To(Equal("hcloud://bm-42"))
})

var _ = Describe("Test hostKey", func() {
	host := &infrav1.HetznerBareMetalHost{}
	host.Namespace = "namespace"
	host.Name = "name"

	Expect(hostKey(host)).To(Equal("namespace/name"))
})

var _ = Describe("Test splitHostKey", func() {
	namespace, name := splitHostKey("namespace/name")
	Expect(namespace).To(Equal("namespace"))
	Expect(name).To(Equal("name"))
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

var _ = Describe("Test splitHostKey", func() {
	namespace, name := splitHostKey("namespace/name")
	Expect(namespace).To(Equal("namespace"))
	Expect(name).To(Equal("name"))
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
