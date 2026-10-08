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
	"net"
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
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
)

var _ = Describe("chooseHost", func() {
	const defaultNamespace = "default"

	bmMachine := &infrav2.HetznerBareMetalMachine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HetznerBareMetalMachine",
			APIVersion: infrav2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bm-machine",
			Namespace: defaultNamespace,
		},
	}

	hostWithCorrectConsumerRef := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithCorrectConsumerRef",
			Namespace: defaultNamespace,
		},
		Spec: infrav2.HetznerBareMetalHostSpec{
			ConsumerRef: &infrav2.HetznerBareMetalHostConsumerReference{
				Name:     "bm-machine",
				Kind:     "HetznerBareMetalMachine",
				APIGroup: infrav2.GroupVersion.Group,
			},
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	hostWithIncorrectConsumerRef := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithIncorrectConsumerRef",
			Namespace: defaultNamespace,
		},
		Spec: infrav2.HetznerBareMetalHostSpec{
			ConsumerRef: &infrav2.HetznerBareMetalHostConsumerReference{
				Name:     "bm-machine-other",
				Kind:     "HetznerBareMetalMachine",
				APIGroup: infrav2.GroupVersion.Group,
			},
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	maintenanceMode := true
	hostInMaintenanceMode := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostInMaintenanceMode",
			Namespace: defaultNamespace,
		},
		Spec: infrav2.HetznerBareMetalHostSpec{
			MaintenanceMode: &maintenanceMode,
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	now := metav1.Now()
	hostWithDeletionTimeStamp := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "hostWithDeletionTimeStamp",
			Namespace:         defaultNamespace,
			DeletionTimestamp: &now,
			Finalizers:        []string{"finalizer"},
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	hostWithError := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithError",
			Namespace: defaultNamespace,
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}
	hostWithError.SetError(infrav2.ErrorTypePermanent, "")

	hostWithStateRegistering := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithStateRegistering",
			Namespace: defaultNamespace,
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateRegistering,
		},
	}

	hostWithOtherLabel := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithOtherLabel",
			Namespace: defaultNamespace,
			Labels:    map[string]string{"wrong": "label"},
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	hostWithOtherNamespace := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithOtherNamespace",
			Namespace: "other-ns",
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	host := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host",
			Namespace: defaultNamespace,
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	hostWithRaidWwnConfig := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithRaidWwnConfig",
			Namespace: defaultNamespace,
		},
		Spec: infrav2.HetznerBareMetalHostSpec{
			RootDeviceHints: &infrav2.RootDeviceHints{
				WWN: "",
				Raid: infrav2.Raid{
					WWN: []string{"wwnRaid1", "wwnRaid2"},
				},
			},
		},
	}
	hostWithNonRaidWwnConfig := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithNonRaidWwnConfig",
			Namespace: defaultNamespace,
		},
		Spec: infrav2.HetznerBareMetalHostSpec{
			RootDeviceHints: &infrav2.RootDeviceHints{
				WWN: "wwnNoRaid",
			},
		},
	}

	hostWithLabel := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithLabel",
			Namespace: defaultNamespace,
			Labels:    map[string]string{"key": "value"},
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	hostWithLabelAndMaintenanceMode := infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostWithLabelAndMaintenanceMode",
			Namespace: defaultNamespace,
			Labels:    map[string]string{"key": "value"},
		},
		Spec: infrav2.HetznerBareMetalHostSpec{
			MaintenanceMode: &maintenanceMode,
		},
		Status: infrav2.HetznerBareMetalHostStatus{
			ProvisioningState: infrav2.StateNone,
		},
	}

	type testCaseChooseHost struct {
		Hosts            []client.Object
		HostSelector     infrav2.HostSelector
		ExpectedHostName string
		RootDeviceHints  infrav2.RootDeviceHints
	}
	DescribeTable(
		"chooseHost",
		func(tc testCaseChooseHost) {
			scheme := runtime.NewScheme()
			utilruntime.Must(infrav2.AddToScheme(scheme))
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(tc.Hosts...).Build()
			bmMachine.Spec.HostSelector = tc.HostSelector
			service := newTestService(bmMachine, c)

			hosts := &infrav2.HetznerBareMetalHostList{}
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
		Entry("No host with error in status",
			testCaseChooseHost{
				Hosts:            []client.Object{&hostWithError, &host},
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
				HostSelector:     infrav2.HostSelector{MatchLabels: map[string]string{"key": "value"}},
				ExpectedHostName: "hostWithLabel",
			}),
		Entry("Choosing host with right label through MatchExpressions",
			testCaseChooseHost{
				Hosts: []client.Object{&hostWithLabel, &hostWithOtherLabel, &hostWithLabelAndMaintenanceMode, &host},
				HostSelector: infrav2.HostSelector{MatchExpressions: []infrav2.HostSelectorRequirement{
					{Key: "key", Operator: selection.In, Values: []string{"value", "value2"}},
				}},
				ExpectedHostName: "hostWithLabel",
			}),
	)

	DescribeTable("chooseHost(): customProvisioner uses its swraid setting",
		func(swraid int, expectedHostName string) {
			bmMachine := &infrav2.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "bmMachine", Namespace: defaultNamespace},
				Spec: infrav2.HetznerBareMetalMachineSpec{
					CustomProvisioner: &infrav2.CustomProvisioner{Swraid: swraid},
				},
			}

			host, reason, err := ChooseHost(bmMachine, []infrav2.HetznerBareMetalHost{hostWithNonRaidWwnConfig, hostWithRaidWwnConfig})
			Expect(err).To(Succeed())
			Expect(reason).To(BeEmpty())
			Expect(host).ToNot(BeNil())
			Expect(host.Name).To(Equal(expectedHostName))
		},
		Entry("swraid 1 chooses the RAID host", 1, "hostWithRaidWwnConfig"),
		Entry("swraid 0 chooses the non-RAID host", 0, "hostWithNonRaidWwnConfig"),
	)

	type testCaseChooseHostWithReason struct {
		hosts            []client.Object
		expectedHostName string
		expectedReason   string
		swraid           int
	}

	DescribeTable(
		"chooseHost(): Test with reason, because RAID config does not match.",
		func(tc testCaseChooseHostWithReason) {
			scheme := runtime.NewScheme()
			utilruntime.Must(infrav2.AddToScheme(scheme))
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(tc.hosts...).Build()
			bmMachine := &infrav2.HetznerBareMetalMachine{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{Name: "bmMachine", Namespace: defaultNamespace},
				Spec: infrav2.HetznerBareMetalMachineSpec{
					InstallImage: &infrav2.InstallImage{
						Swraid: tc.swraid,
					},
				},
			}
			service := newTestService(bmMachine, c)

			hosts := &infrav2.HetznerBareMetalHostList{}
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
	nic1 := infrav2.NIC{
		IP: "192.168.1.1",
	}

	nic2 := infrav2.NIC{
		IP: "172.0.20.2",
	}

	nic3 := infrav2.NIC{
		IP: "203.0.113.5/26",
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

	addr5 := clusterv1.MachineAddress{
		Type:    clusterv1.MachineExternalIP,
		Address: "203.0.113.5",
	}

	addr6 := clusterv1.MachineAddress{
		Type:    clusterv1.MachineInternalIP,
		Address: "203.0.113.5/26",
	}

	type testCaseNodeAddress struct {
		Machine               clusterv1.Machine
		BareMetalMachine      infrav2.HetznerBareMetalMachine
		Host                  *infrav2.HetznerBareMetalHost
		HasOldStyle           bool
		ExpectedNodeAddresses []clusterv1.MachineAddress
	}

	DescribeTable(
		"Test NodeAddress",
		func(tc testCaseNodeAddress) {
			nodeAddresses := nodeAddresses(tc.Host, "bm-machine", tc.HasOldStyle)
			for i, address := range tc.ExpectedNodeAddresses {
				Expect(nodeAddresses[i]).To(Equal(address))
			}
		},
		Entry("One NIC", testCaseNodeAddress{
			Host: &infrav2.HetznerBareMetalHost{
				Status: infrav2.HetznerBareMetalHostStatus{
					HardwareDetails: &infrav2.HardwareDetails{
						NIC: []infrav2.NIC{nic1},
					},
				},
			},
			HasOldStyle:           true,
			ExpectedNodeAddresses: []clusterv1.MachineAddress{addr1, addr3, addr4},
		}),
		Entry("Two NICs", testCaseNodeAddress{
			Host: &infrav2.HetznerBareMetalHost{
				Status: infrav2.HetznerBareMetalHostStatus{
					HardwareDetails: &infrav2.HardwareDetails{
						NIC: []infrav2.NIC{nic1, nic2},
					},
				},
			},
			HasOldStyle:           true,
			ExpectedNodeAddresses: []clusterv1.MachineAddress{addr1, addr2, addr3, addr4},
		}),
		Entry("existing machine (hasOldStyle=true) keeps CIDR suffix and always reports InternalIP", testCaseNodeAddress{
			Host: &infrav2.HetznerBareMetalHost{
				Status: infrav2.HetznerBareMetalHostStatus{
					HardwareDetails: &infrav2.HardwareDetails{
						NIC: []infrav2.NIC{nic3},
					},
				},
			},
			HasOldStyle:           true,
			ExpectedNodeAddresses: []clusterv1.MachineAddress{addr6, addr3, addr4},
		}),
		Entry("new machine (hasOldStyle=false) strips CIDR suffix and reports public IP as ExternalIP", testCaseNodeAddress{
			Host: &infrav2.HetznerBareMetalHost{
				Status: infrav2.HetznerBareMetalHostStatus{
					HardwareDetails: &infrav2.HardwareDetails{
						NIC: []infrav2.NIC{nic3},
					},
				},
			},
			HasOldStyle:           false,
			ExpectedNodeAddresses: []clusterv1.MachineAddress{addr5, addr3, addr4},
		}),
		Entry("new machine (hasOldStyle=false) keeps private IP as InternalIP", testCaseNodeAddress{
			Host: &infrav2.HetznerBareMetalHost{
				Status: infrav2.HetznerBareMetalHostStatus{
					HardwareDetails: &infrav2.HardwareDetails{
						NIC: []infrav2.NIC{nic1},
					},
				},
			},
			HasOldStyle:           false,
			ExpectedNodeAddresses: []clusterv1.MachineAddress{addr1, addr3, addr4},
		}),
	)
})

var _ = Describe("Test hasOldStyleIPAddress", func() {
	DescribeTable(
		"hasOldStyleIPAddress",
		func(addrs []clusterv1.MachineAddress, expected bool) {
			Expect(hasOldStyleIPAddress(addrs)).To(Equal(expected))
		},
		Entry("nil addresses", nil, false),
		Entry("only hostname/internalDNS addresses", []clusterv1.MachineAddress{
			{Type: clusterv1.MachineHostName, Address: "bm-machine"},
			{Type: clusterv1.MachineInternalDNS, Address: "bm-machine"},
		}, false),
		Entry("InternalIP with CIDR suffix (old logic)", []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: "192.168.1.1/24"},
		}, true),
		Entry("ExternalIP with CIDR suffix is never old-style (old logic never produces ExternalIP)", []clusterv1.MachineAddress{
			{Type: clusterv1.MachineExternalIP, Address: "203.0.113.5/26"},
		}, false),
		Entry("InternalIP without CIDR suffix (corrected logic already applied)", []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: "192.168.1.1"},
		}, false),
		Entry("ExternalIP without CIDR suffix (corrected logic already applied)", []clusterv1.MachineAddress{
			{Type: clusterv1.MachineExternalIP, Address: "203.0.113.5"},
		}, false),
	)
})

var _ = Describe("Test updateMachineAddresses", func() {
	newHostWithNIC := func(ip string) *infrav2.HetznerBareMetalHost {
		return &infrav2.HetznerBareMetalHost{
			Status: infrav2.HetznerBareMetalHostStatus{
				HardwareDetails: &infrav2.HardwareDetails{
					NIC: []infrav2.NIC{{IP: ip}},
				},
			},
		}
	}

	It("keeps computing addresses the old way for a machine that already reported InternalIP", func() {
		bmMachine := &infrav2.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-machine", Namespace: "default"},
			Status: infrav2.HetznerBareMetalMachineStatus{
				Addresses: []clusterv1.MachineAddress{
					{Type: clusterv1.MachineInternalIP, Address: "203.0.113.5/26"},
				},
			},
		}
		s := &Service{scope: &scope.BareMetalMachineScope{BareMetalMachine: bmMachine}}

		s.updateMachineAddresses(newHostWithNIC("203.0.113.5/26"))

		Expect(bmMachine.Status.Addresses).To(ContainElement(clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalIP,
			Address: "203.0.113.5/26",
		}))
	})

	It("uses the corrected classification for a machine that never reported an IP-type address", func() {
		bmMachine := &infrav2.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-machine", Namespace: "default"},
		}
		s := &Service{scope: &scope.BareMetalMachineScope{BareMetalMachine: bmMachine}}

		s.updateMachineAddresses(newHostWithNIC("203.0.113.5/26"))

		Expect(bmMachine.Status.Addresses).To(ContainElement(clusterv1.MachineAddress{
			Type:    clusterv1.MachineExternalIP,
			Address: "203.0.113.5",
		}))
	})

	It("keeps reporting the corrected classification across repeated reconciles of a new machine", func() {
		// Regression test: checking merely "does status.addresses already have an
		// InternalIP/ExternalIP entry" would flip back to the old logic as soon as the
		// corrected logic wrote its first result, since that result IS such an entry.
		bmMachine := &infrav2.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-machine", Namespace: "default"},
		}
		s := &Service{scope: &scope.BareMetalMachineScope{BareMetalMachine: bmMachine}}
		host := newHostWithNIC("203.0.113.5/26")

		for i := range 3 {
			s.updateMachineAddresses(host)
			Expect(bmMachine.Status.Addresses).To(ContainElement(clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
				Address: "203.0.113.5",
			}), "reconcile #%d should still report the corrected ExternalIP classification", i+1)
		}
	})
})

var _ = Describe("Test consumerRefMatches", func() {
	type testCaseConsumerRefMatches struct {
		Consumer       *infrav2.HetznerBareMetalHostConsumerReference
		ExpectedResult bool
	}

	bmMachine := &infrav2.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bm-machine",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "HetznerBareMetalMachine",
			APIVersion: infrav2.GroupVersion.String(),
		},
	}

	// The consumer ref has no namespace. The host and the consuming HetznerBareMetalMachine always
	// live in the same namespace, so there is no entry for a namespace mismatch.
	DescribeTable(
		"Test consumerRefMatches",
		func(tc testCaseConsumerRefMatches) {
			Expect(consumerRefMatches(tc.Consumer, bmMachine)).To(Equal(tc.ExpectedResult))
		},
		Entry("Matching consumer", testCaseConsumerRefMatches{
			Consumer: &infrav2.HetznerBareMetalHostConsumerReference{
				Name:     "bm-machine",
				Kind:     "HetznerBareMetalMachine",
				APIGroup: infrav2.GroupVersion.Group,
			},
			ExpectedResult: true,
		}),
		Entry("No matching name", testCaseConsumerRefMatches{
			Consumer: &infrav2.HetznerBareMetalHostConsumerReference{
				Name:     "other-bm-machine",
				Kind:     "HetznerBareMetalMachine",
				APIGroup: infrav2.GroupVersion.Group,
			},
			ExpectedResult: false,
		}),
		Entry("No matching kind", testCaseConsumerRefMatches{
			Consumer: &infrav2.HetznerBareMetalHostConsumerReference{
				Name:     "bm-machine",
				Kind:     "OtherBareMetalMachine",
				APIGroup: infrav2.GroupVersion.Group,
			},
			ExpectedResult: false,
		}),
		Entry("No matching API group", testCaseConsumerRefMatches{
			Consumer: &infrav2.HetznerBareMetalHostConsumerReference{
				Name:     "bm-machine",
				Kind:     "HetznerBareMetalMachine",
				APIGroup: "other-group.example.com",
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

	DescribeTable(
		"Test setOwnerRefInList",
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

	DescribeTable(
		"Test ensureMachineAnnotation",
		func(tc testCaseEnsureMachineyyAnnotation) {
			bmMachine := &infrav2.HetznerBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "bm-machine",
					Namespace:   "default",
					Annotations: tc.Annotations,
				},
			}
			service := newTestService(bmMachine, nil)

			host := infrav2.HetznerBareMetalHost{
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
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav2.HostAnnotation: "default/hostName"},
		}),
		Entry("Empty list", testCaseEnsureMachineyyAnnotation{
			Annotations:         map[string]string{},
			ExpectedAnnotations: map[string]string{infrav2.HostAnnotation: "default/hostName"},
		}),
		Entry("Nil list", testCaseEnsureMachineyyAnnotation{
			Annotations:         nil,
			ExpectedAnnotations: map[string]string{infrav2.HostAnnotation: "default/hostName"},
		}),
		Entry("List of one non-matching and one matching entry", testCaseEnsureMachineyyAnnotation{
			Annotations:         map[string]string{"key1": "val1", infrav2.HostAnnotation: "default/hostName"},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav2.HostAnnotation: "default/hostName"},
		}),
	)
})

var _ = Describe("Test updateHostAnnotation", func() {
	type testCaseUpdateHostAnnotation struct {
		Annotations         map[string]string
		ExpectedAnnotations map[string]string
	}

	const hostKey = "default/hostName"

	DescribeTable(
		"Test updateHostAnnotation",
		func(tc testCaseUpdateHostAnnotation) {
			updatedAnnotations := updateHostAnnotation(tc.Annotations, hostKey, logr.Discard())
			Expect(updatedAnnotations).Should(Equal(tc.ExpectedAnnotations))
		},
		Entry("List of one non-matching entry", testCaseUpdateHostAnnotation{
			Annotations:         map[string]string{"key1": "val1"},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav2.HostAnnotation: hostKey},
		}),
		Entry("Empty list", testCaseUpdateHostAnnotation{
			Annotations:         map[string]string{},
			ExpectedAnnotations: map[string]string{infrav2.HostAnnotation: hostKey},
		}),
		Entry("Nil list", testCaseUpdateHostAnnotation{
			Annotations:         nil,
			ExpectedAnnotations: map[string]string{infrav2.HostAnnotation: hostKey},
		}),
		Entry("List of one non-matching and one matching entry", testCaseUpdateHostAnnotation{
			Annotations:         map[string]string{"key1": "val1", infrav2.HostAnnotation: hostKey},
			ExpectedAnnotations: map[string]string{"key1": "val1", infrav2.HostAnnotation: hostKey},
		}),
	)
})

var _ = Describe("Test ensureClusterLabel", func() {
	type testCaseEnsureClusterLabel struct {
		labels         map[string]string
		expectedLabels map[string]string
	}

	const clusterName = "clusterName"

	DescribeTable(
		"Test ensureClusterLabel",
		func(tc testCaseEnsureClusterLabel) {
			host := &infrav2.HetznerBareMetalHost{}
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
	host := &infrav2.HetznerBareMetalHost{}
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

	DescribeTable(
		"Test ensureClusterLabel",
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

	DescribeTable(
		"Test analyzePatchError",
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
		hetznerCluster     *infrav2.HetznerCluster
		serverNumber       int
		expectedProviderID string
	}

	newHetznerCluster := func() *infrav2.HetznerCluster {
		return &infrav2.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-hetzner-cluster",
				Annotations: map[string]string{},
			},
		}
	}

	DescribeTable(
		"GenerateProviderID",
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
			hetznerCluster: func() *infrav2.HetznerCluster {
				hetznerCluster := newHetznerCluster()
				hetznerCluster.Annotations = map[string]string{
					infrav2.UseHrobotProviderIDForBaremetalAnnotation: "true",
				}
				return hetznerCluster
			}(),
			serverNumber:       11,
			expectedProviderID: "hrobot://11",
		}),
		Entry("Uses legacy prefix for non-true annotation value", testCaseGenerateProviderID{
			hetznerCluster: func() *infrav2.HetznerCluster {
				hetznerCluster := newHetznerCluster()
				hetznerCluster.Annotations = map[string]string{
					infrav2.UseHrobotProviderIDForBaremetalAnnotation: "invalid",
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
		bareMetalMachine *infrav2.HetznerBareMetalMachine,
		cluster *clusterv1.Cluster,
		hetznerCluster *infrav2.HetznerCluster,
		hcloudClient *mocks.Client,
	) *Service {
		return &Service{
			scope: &scope.BareMetalMachineScope{
				Machine:          machine,
				BareMetalMachine: bareMetalMachine,
				Cluster:          cluster,
				HetznerCluster:   hetznerCluster,
				HCloudClient:     hcloudClient,
				EventRecorder:    record.NewFakeRecorder(100),
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

	newHostWithIPs := func(ipv4, ipv6 string) *infrav2.HetznerBareMetalHost {
		return &infrav2.HetznerBareMetalHost{
			Spec: infrav2.HetznerBareMetalHostSpec{
				ServerID: 42,
			},
			Status: infrav2.HetznerBareMetalHostStatus{
				IPv4: ipv4,
				IPv6: ipv6,
			},
		}
	}

	newHost := func(ipv4 string) *infrav2.HetznerBareMetalHost {
		return newHostWithIPs(ipv4, "")
	}

	// The address family cases below use a host that has both addresses, so that the
	// selected family is the only thing deciding which of them ends up as a target.
	const (
		hostIPv4 = "192.0.2.10"
		hostIPv6 = "2001:db8::10"
	)

	// newClusterForAddressFamily returns a cluster whose load balancer already exists and
	// has the given targets, so that reconcileLoadBalancerAttachment reads them from the
	// status instead of calling the HCloud API.
	newClusterForAddressFamily := func(family infrav2.LoadBalancerTargetAddressFamily, attached ...string) *infrav2.HetznerCluster {
		targets := make([]infrav2.LoadBalancerTarget, 0, len(attached))
		for _, ip := range attached {
			targets = append(targets, infrav2.LoadBalancerTarget{Type: infrav2.LoadBalancerTargetTypeIP, IP: ip})
		}
		return &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{TargetAddressFamily: family},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{ID: 123, Target: targets},
			},
		}
	}

	// newHealthyMachine returns a machine that passes the kube-apiserver health gate, so
	// that the gate does not interfere with the address family assertions.
	newHealthyMachine := func() *clusterv1.Machine {
		machine := &clusterv1.Machine{}
		conditions.Set(machine, metav1.Condition{
			Type:   controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
			Status: metav1.ConditionTrue,
			Reason: "PodHealthy",
		})
		return machine
	}

	// newAvailableBareMetalMachine returns a machine that makes the reconcile read the
	// attached targets from the cluster status rather than from the HCloud API.
	newAvailableBareMetalMachine := func() *infrav2.HetznerBareMetalMachine {
		bareMetalMachine := &infrav2.HetznerBareMetalMachine{}
		deprecatedv1beta1conditions.MarkTrue(bareMetalMachine, infrav2.ServerAvailableV1Beta1Condition)
		conditions.Set(bareMetalMachine, metav1.Condition{
			Type:   infrav2.HetznerBareMetalMachineServerAvailableCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HetznerBareMetalMachineServerAvailableReason,
		})
		return bareMetalMachine
	}

	expectAddIPTarget := func(hcloudClient *mocks.Client, ip string) {
		hcloudClient.On(
			"AddIPTargetToLoadBalancer",
			mock.Anything,
			mock.MatchedBy(func(opts hcloud.LoadBalancerAddIPTargetOpts) bool {
				return opts.IP.String() == ip
			}),
			mock.MatchedBy(func(lb *hcloud.LoadBalancer) bool { return lb.ID == 123 }),
		).Return(nil).Once()
	}

	expectDeleteIPTarget := func(hcloudClient *mocks.Client, ip string) {
		hcloudClient.On(
			"DeleteIPTargetOfLoadBalancer",
			mock.Anything,
			mock.MatchedBy(func(lb *hcloud.LoadBalancer) bool { return lb.ID == 123 }),
			mock.MatchedBy(func(target net.IP) bool { return target.String() == ip }),
		).Return(nil).Once()
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

		bareMetalMachine := &infrav2.HetznerBareMetalMachine{}
		hetznerCluster := &infrav2.HetznerCluster{
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
					ID: 123,
					Target: []infrav2.LoadBalancerTarget{
						{Type: infrav2.LoadBalancerTargetTypeIP, IP: "192.0.2.9"},
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
		Expect(deprecatedv1beta1conditions.IsFalse(bareMetalMachine, infrav2.ServerAvailableV1Beta1Condition)).To(BeTrue())
		Expect(deprecatedv1beta1conditions.GetReason(bareMetalMachine, infrav2.ServerAvailableV1Beta1Condition)).To(Equal("WaitingForAPIServer"))
		Expect(conditions.IsFalse(bareMetalMachine, infrav2.HetznerBareMetalMachineServerAvailableCondition)).To(BeTrue())
		Expect(conditions.GetReason(bareMetalMachine, infrav2.HetznerBareMetalMachineServerAvailableCondition)).To(Equal(infrav2.HetznerBareMetalMachineWaitingForAPIServerReason))
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

		bareMetalMachine := &infrav2.HetznerBareMetalMachine{}
		hetznerCluster := &infrav2.HetznerCluster{
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
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

	It("attaches both addresses when no address family is configured", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		expectAddIPTarget(hcloudClient, hostIPv4)
		expectAddIPTarget(hcloudClient, hostIPv6)

		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(""), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))).To(Succeed())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "DeleteIPTargetOfLoadBalancer", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
	})

	It("attaches only the IPv6 address when the address family is ipv6", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		expectAddIPTarget(hcloudClient, hostIPv6)

		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyIPv6), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))).To(Succeed())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
	})

	It("attaches both addresses when the address family is dualstack", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		expectAddIPTarget(hcloudClient, hostIPv4)
		expectAddIPTarget(hcloudClient, hostIPv6)

		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyDualStack), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))).To(Succeed())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
	})

	It("detaches an address that the configured address family no longer selects", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		expectDeleteIPTarget(hcloudClient, hostIPv6)

		// Both addresses are attached but the family is ipv4, so the IPv6 target is stale.
		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyIPv4, hostIPv4, hostIPv6), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))).To(Succeed())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "AddIPTargetToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
	})

	It("detaches an unselected address even while the api server pod is not healthy", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		expectDeleteIPTarget(hcloudClient, hostIPv6)

		machine := &clusterv1.Machine{}
		conditions.Set(machine, metav1.Condition{
			Type:    controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "PodNotHealthy",
			Message: "kube-apiserver is still starting",
		})

		// The IPv6 address of this host is attached and no longer selected, while its
		// IPv4 address is not attached yet. The health gate holds the attachment back,
		// but a target that cannot serve traffic is removed right away.
		service := newServiceForLoadBalancerAttachment(
			machine, newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyIPv4, "192.0.2.9", hostIPv6), hcloudClient,
		)

		err := service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))
		var requeueErr *scope.RequeueAfterError
		Expect(errors.As(err, &requeueErr)).To(BeTrue())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "AddIPTargetToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
	})

	It("skips an address that cannot be used as a target and attaches the other one", func() {
		hcloudClient := mocks.NewClient(GinkgoT())
		expectAddIPTarget(hcloudClient, hostIPv4)

		// dualstack selects both addresses, but only the one the HCloud API can accept
		// as a target is attached.
		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyDualStack), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, "not-an-address"))).To(Succeed())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
	})

	It("does not call the HCloud API when the attached targets already match the address family", func() {
		hcloudClient := mocks.NewClient(GinkgoT())

		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyIPv4, hostIPv4), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))).To(Succeed())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "AddIPTargetToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "DeleteIPTargetOfLoadBalancer", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "ListLoadBalancers", mock.Anything, mock.Anything)).To(BeTrue())
	})

	It("keeps attaching the remaining address when the first is already a target", func() {
		hcloudClient := mocks.NewClient(GinkgoT())

		// dualstack attaches the IPv4 address first, then the IPv6 one. The IPv4 target
		// already exists, so the API returns TargetAlreadyDefined for it. The reconcile has
		// to carry on and still attach the IPv6 address.
		hcloudClient.On(
			"AddIPTargetToLoadBalancer",
			mock.Anything,
			mock.MatchedBy(func(opts hcloud.LoadBalancerAddIPTargetOpts) bool { return opts.IP.String() == hostIPv4 }),
			mock.MatchedBy(func(lb *hcloud.LoadBalancer) bool { return lb.ID == 123 }),
		).Return(hcloud.Error{Code: hcloud.ErrorCodeTargetAlreadyDefined}).Once()
		expectAddIPTarget(hcloudClient, hostIPv6)

		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyDualStack), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))).To(Succeed())
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
	})

	It("returns an error when detaching an unselected address fails", func() {
		hcloudClient := mocks.NewClient(GinkgoT())

		// ipv4 is in effect, so the stale IPv6 target is detached. The API call fails with an
		// error that is not "target not found", so the reconcile surfaces it.
		hcloudClient.On(
			"DeleteIPTargetOfLoadBalancer",
			mock.Anything,
			mock.MatchedBy(func(lb *hcloud.LoadBalancer) bool { return lb.ID == 123 }),
			mock.MatchedBy(func(target net.IP) bool { return target.String() == hostIPv6 }),
		).Return(errors.New("some hcloud error")).Once()

		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyIPv4, hostIPv4, hostIPv6), hcloudClient,
		)

		err := service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))
		Expect(err).To(MatchError(ContainSubstring("some hcloud error")))
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
		Expect(hcloudClient.AssertNotCalled(GinkgoT(), "AddIPTargetToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
	})

	It("treats a not-found error while detaching as already done", func() {
		hcloudClient := mocks.NewClient(GinkgoT())

		// The stale IPv6 target is already gone, so the delete reports it as not found. That
		// is the desired state, so the reconcile treats it as a success.
		hcloudClient.On(
			"DeleteIPTargetOfLoadBalancer",
			mock.Anything,
			mock.MatchedBy(func(lb *hcloud.LoadBalancer) bool { return lb.ID == 123 }),
			mock.MatchedBy(func(target net.IP) bool { return target.String() == hostIPv6 }),
		).Return(errors.New("load_balancer_target_not_found")).Once()

		service := newServiceForLoadBalancerAttachment(
			newHealthyMachine(), newAvailableBareMetalMachine(), newControlPlaneCluster(),
			newClusterForAddressFamily(infrav2.LoadBalancerTargetAddressFamilyIPv4, hostIPv4, hostIPv6), hcloudClient,
		)

		Expect(service.reconcileLoadBalancerAttachment(context.Background(), newHostWithIPs(hostIPv4, hostIPv6))).To(Succeed())
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

		bareMetalMachine := &infrav2.HetznerBareMetalMachine{}
		conditions.Set(bareMetalMachine, metav1.Condition{
			Type:   infrav2.HetznerBareMetalMachineServerAvailableCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HetznerBareMetalMachineServerAvailableReason,
		})

		hetznerCluster := &infrav2.HetznerCluster{
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
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

	buildService := func(lbTargets []infrav2.LoadBalancerTarget, apiServerHealthy, isControlPlane bool) (
		*Service, *infrav2.HetznerBareMetalMachine, *mocks.Client,
	) {
		scheme := runtime.NewScheme()
		utilruntime.Must(infrav2.AddToScheme(scheme))
		utilruntime.Must(clusterv1.AddToScheme(scheme))
		utilruntime.Must(corev1.AddToScheme(scheme))

		host := &infrav2.HetznerBareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: testHostName, Namespace: testNamespace},
			Spec: infrav2.HetznerBareMetalHostSpec{
				ServerID: 42,
			},
			Status: infrav2.HetznerBareMetalHostStatus{
				IPv4:              "192.0.2.10",
				ProvisioningState: infrav2.StateProvisioned,
			},
		}

		bareMetalMachine := &infrav2.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testBMMName,
				Namespace: testNamespace,
				Annotations: map[string]string{
					infrav2.HostAnnotation: testNamespace + "/" + testHostName,
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

		hetznerCluster := &infrav2.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{Name: testCluster, Namespace: testNamespace},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
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
				EventRecorder:    record.NewFakeRecorder(100),
			},
		}

		return service, bareMetalMachine, hcloudClient
	}

	It("keeps ProviderID and initialization.provisioned set when reconcileLoadBalancerAttachment requeues for WaitingForAPIServer", func() {
		// An existing load balancer target and an unhealthy kube-apiserver pod make
		// reconcileLoadBalancerAttachment return a RequeueAfterError with WaitingForAPIServer.
		// Reconcile must still set initialization.provisioned and ProviderID. See the comment
		// in Reconcile.
		service, bareMetalMachine, hcloudClient := buildService(
			[]infrav2.LoadBalancerTarget{
				{Type: infrav2.LoadBalancerTargetTypeIP, IP: "192.0.2.9"},
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

		Expect(ptr.Deref(bareMetalMachine.Status.Initialization.Provisioned, false)).To(BeTrue())
		Expect(bareMetalMachine.Spec.ProviderID).NotTo(BeNil())
		Expect(*bareMetalMachine.Spec.ProviderID).NotTo(BeEmpty())
		Expect(deprecatedv1beta1conditions.IsFalse(bareMetalMachine, infrav2.ServerAvailableV1Beta1Condition)).To(BeTrue())
		Expect(deprecatedv1beta1conditions.GetReason(bareMetalMachine, infrav2.ServerAvailableV1Beta1Condition)).To(Equal("WaitingForAPIServer"))
		Expect(conditions.IsFalse(bareMetalMachine, infrav2.HetznerBareMetalMachineServerAvailableCondition)).To(BeTrue())
		Expect(conditions.GetReason(bareMetalMachine, infrav2.HetznerBareMetalMachineServerAvailableCondition)).To(Equal(infrav2.HetznerBareMetalMachineWaitingForAPIServerReason))
	})

	It("does not requeue on the happy path and marks ServerAvailableCondition=True", func() {
		// LB target list already contains this host's IPv4, so
		// reconcileLoadBalancerAttachment returns no requeue and Reconcile
		// marks the condition true.
		service, bareMetalMachine, hcloudClient := buildService(
			[]infrav2.LoadBalancerTarget{
				{Type: infrav2.LoadBalancerTargetTypeIP, IP: "192.0.2.10"},
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

		Expect(ptr.Deref(bareMetalMachine.Status.Initialization.Provisioned, false)).To(BeTrue())
		Expect(bareMetalMachine.Spec.ProviderID).NotTo(BeNil())
		Expect(deprecatedv1beta1conditions.IsTrue(bareMetalMachine, infrav2.ServerAvailableV1Beta1Condition)).To(BeTrue())
		Expect(conditions.IsTrue(bareMetalMachine, infrav2.HetznerBareMetalMachineServerAvailableCondition)).To(BeTrue())
	})

	It("marks ServerAvailableCondition=True for worker nodes without touching the load balancer", func() {
		// Worker nodes never hit reconcileLoadBalancerAttachment, but Reconcile
		// must still mark the condition true so the condition is meaningful on
		// non-control-plane HetznerBareMetalMachines too.
		service, bareMetalMachine, _ := buildService(nil, true, false)

		res, err := service.Reconcile(context.Background())
		Expect(err).To(BeNil())
		Expect(res).To(Equal(reconcile.Result{}))

		Expect(ptr.Deref(bareMetalMachine.Status.Initialization.Provisioned, false)).To(BeTrue())
		Expect(bareMetalMachine.Spec.ProviderID).NotTo(BeNil())
		Expect(deprecatedv1beta1conditions.IsTrue(bareMetalMachine, infrav2.ServerAvailableV1Beta1Condition)).To(BeTrue())
		Expect(conditions.IsTrue(bareMetalMachine, infrav2.HetznerBareMetalMachineServerAvailableCondition)).To(BeTrue())
	})
})

var _ = Describe("Delete", func() {
	It("sets the Deleting condition and requeues while the host is still provisioned", func() {
		const ns = "default"
		scheme := runtime.NewScheme()
		utilruntime.Must(infrav2.AddToScheme(scheme))
		utilruntime.Must(clusterv1.AddToScheme(scheme))

		host := &infrav2.HetznerBareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-host", Namespace: ns},
			Spec: infrav2.HetznerBareMetalHostSpec{
				ConsumerRef: &infrav2.HetznerBareMetalHostConsumerReference{
					Name:     "bm-machine",
					Kind:     "HetznerBareMetalMachine",
					APIGroup: infrav2.GroupVersion.Group,
				},
			},
			Status: infrav2.HetznerBareMetalHostStatus{
				ProvisioningState: infrav2.StateProvisioned,
			},
		}
		bareMetalMachine := &infrav2.HetznerBareMetalMachine{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HetznerBareMetalMachine",
				APIVersion: infrav2.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bm-machine",
				Namespace:   ns,
				Annotations: map[string]string{infrav2.HostAnnotation: ns + "/bm-host"},
			},
		}
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-machine", Namespace: ns},
		}

		c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(host, bareMetalMachine).Build()
		service := &Service{
			scope: &scope.BareMetalMachineScope{
				Logger:           log,
				Client:           c,
				Machine:          machine,
				BareMetalMachine: bareMetalMachine,
			},
		}

		res, err := service.Delete(context.Background())
		Expect(err).To(BeNil())
		Expect(res.RequeueAfter).To(Equal(requeueAfter))

		Expect(conditions.IsTrue(bareMetalMachine, infrav2.HetznerBareMetalMachineDeletingCondition)).To(BeTrue())
		Expect(conditions.GetReason(bareMetalMachine, infrav2.HetznerBareMetalMachineDeletingCondition)).To(Equal(infrav2.HetznerBareMetalMachineDeletingReason))
		Expect(bareMetalMachine.Status.Phase).To(Equal(clusterv1.MachinePhaseDeleting))
	})
})
