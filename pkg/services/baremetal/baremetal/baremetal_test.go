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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2/extensions/table"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	hostInMaintenanceMode := infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostInMaintenanceMode",
			Namespace: defaultNamespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			MaintenanceMode: true,
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
			MaintenanceMode: true,
			Status: infrav1.ControllerGeneratedStatus{
				ProvisioningState: infrav1.StateNone,
			},
		},
	}

	type testCaseChooseHost struct {
		Hosts            []client.Object
		HostSelector     infrav1.HostSelector
		ExpectedHostName string
	}
	DescribeTable("chooseHost",
		func(tc testCaseChooseHost) {
			scheme := runtime.NewScheme()
			utilruntime.Must(infrav1.AddToScheme(scheme))
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(tc.Hosts...).Build()
			bmMachine.Spec.HostSelector = tc.HostSelector
			service := newTestService(bmMachine, c)

			host, _, err := service.chooseHost(context.TODO())
			Expect(err).To(Succeed())
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
})

var _ = Describe("Test NodeAddresses", func() {
	nic1 := infrav1.NIC{
		IP: "192.168.1.1",
	}

	nic2 := infrav1.NIC{
		IP: "172.0.20.2",
	}

	addr1 := corev1.NodeAddress{
		Type:    corev1.NodeInternalIP,
		Address: "192.168.1.1",
	}

	addr2 := corev1.NodeAddress{
		Type:    corev1.NodeInternalIP,
		Address: "172.0.20.2",
	}

	addr3 := corev1.NodeAddress{
		Type:    corev1.NodeHostName,
		Address: "bm-machine",
	}

	addr4 := corev1.NodeAddress{
		Type:    corev1.NodeInternalDNS,
		Address: "bm-machine",
	}

	type testCaseNodeAddress struct {
		Machine               clusterv1.Machine
		BareMetalMachine      infrav1.HetznerBareMetalMachine
		Host                  *infrav1.HetznerBareMetalHost
		ExpectedNodeAddresses []corev1.NodeAddress
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
			ExpectedNodeAddresses: []corev1.NodeAddress{addr1, addr3, addr4},
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
			ExpectedNodeAddresses: []corev1.NodeAddress{addr1, addr2, addr3, addr4},
		}),
		Entry("No host", testCaseNodeAddress{
			Host:                  nil,
			ExpectedNodeAddresses: nil,
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

var _ = Describe("Test findOwnerRefFromList", func() {
	type testCaseFindOwnerRefFromList struct {
		RefList          []metav1.OwnerReference
		ExpectedPosition *int
	}

	objectMeta := metav1.ObjectMeta{
		Name:      "bm-machine",
		Namespace: "default",
	}
	objectType := metav1.TypeMeta{
		Kind:       "HetznerBareMetalMachine",
		APIVersion: "v1beta1",
	}

	DescribeTable("Test findOwnerRefFromList",
		func(tc testCaseFindOwnerRefFromList) {
			position, err := findOwnerRefFromList(tc.RefList, objectType, objectMeta)

			if tc.ExpectedPosition != nil {
				Expect(err).To(Succeed())
				Expect(position).To(Equal(*tc.ExpectedPosition))
			} else {
				Expect(err).ToNot(Succeed())
			}
		},
		Entry("Matching consumer", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(0),
		}),
		Entry("Matching consumer position 1", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(1),
		}),
		Entry("Matching consumer position 1a", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "OtherBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(1),
		}),
		Entry("Matching consumer position 1b", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "hetzner/v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(1),
		}),
	)
})

var _ = Describe("Test deleteOwnerRefFromList", func() {
	type testCaseFindOwnerRefFromList struct {
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

	expectedRefList3 := make([]metav1.OwnerReference, 0, 2)
	expectedRefList3 = append(expectedRefList3, metav1.OwnerReference{

		Name:       "bm-machine2",
		Kind:       "HetznerBareMetalMachine",
		APIVersion: "v1beta1",
	})

	DescribeTable("Test deleteOwnerRefFromList",
		func(tc testCaseFindOwnerRefFromList) {
			refList, err := deleteOwnerRefFromList(tc.RefList, objectType, objectMeta)
			Expect(err).To(Succeed())
			Expect(refList).To(Equal(tc.ExpectedRefList))

		},
		Entry("List of one matching entry", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedRefList: []metav1.OwnerReference{},
		}),
		Entry("List of one non-matching entry", testCaseFindOwnerRefFromList{
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
			},
		}),
		Entry("Two entries with matching", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedRefList: expectedRefList3,
		}),
	)
})

var _ = Describe("Test setOwnerRefInList", func() {
	type testCaseSetOwnerRefInList struct {
		RefList         []metav1.OwnerReference
		Controller      bool
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
			refList, err := setOwnerRefInList(tc.RefList, tc.Controller, objectType, objectMeta)
			Expect(err).To(Succeed())
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
			Controller: false,
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
					Controller: pointer.Bool(false),
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
			Controller: false,
			ExpectedRefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
					Controller: pointer.Bool(false),
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
			Controller: true,
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
					Controller: pointer.Bool(true),
				},
			},
		}),
	)
})

var _ = Describe("Test ensureAnnotation", func() {
	type testCaseEnsureAnnotation struct {
		Annotations         map[string]string
		ExpectedAnnotations map[string]string
	}

	DescribeTable("Test setOwnerRefInList",
		func(tc testCaseEnsureAnnotation) {
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

			Expect(service.ensureAnnotation(&host)).To(Succeed())
			Expect(bmMachine.GetAnnotations()).Should(Equal(tc.ExpectedAnnotations))
		},
		Entry("List of one non-matching entry", testCaseEnsureAnnotation{
			Annotations:         map[string]string{"key1": "val1"},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav1.HostAnnotation: "default/hostName"},
		}),
		Entry("Empty list", testCaseEnsureAnnotation{
			Annotations:         map[string]string{},
			ExpectedAnnotations: map[string]string{infrav1.HostAnnotation: "default/hostName"},
		}),
		Entry("Nil list", testCaseEnsureAnnotation{
			Annotations:         nil,
			ExpectedAnnotations: map[string]string{infrav1.HostAnnotation: "default/hostName"},
		}),
		Entry("List of one non-matching and one matching entry", testCaseEnsureAnnotation{
			Annotations:         map[string]string{"key1": "val1", infrav1.HostAnnotation: "default/hostName"},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav1.HostAnnotation: "default/hostName"},
		}),
	)
})
