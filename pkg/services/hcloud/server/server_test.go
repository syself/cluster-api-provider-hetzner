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

package server

import (
	"context"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	fakeclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

var _ = Describe("statusFromHCloudServer", func() {
	var sts infrav1.HCloudMachineStatus
	BeforeEach(func() {
		sts = statusFromHCloudServer(server)
	})
	It("should have the right instance state", func() {
		Expect(*sts.InstanceState).To(Equal(instanceState))
	})
	It("should have three addresses", func() {
		Expect(len(sts.Addresses)).To(Equal(3))
	})
	It("should have the right address IPs", func() {
		for i, addr := range sts.Addresses {
			Expect(addr.Address).To(Equal(ips[i]))
		}
	})
	It("should have the right address types", func() {
		for i, addr := range sts.Addresses {
			Expect(addr.Type).To(Equal(addressTypes[i]))
		}
	})
})

var _ = DescribeTable("createLabels",
	func(hetznerClusterName, hcloudMachineName string, isControlPlane bool, expectedOutput map[string]string) {
		hcloudMachine := infrav1.HCloudMachine{}
		hcloudMachine.Name = hcloudMachineName

		hetznerCluster := infrav1.HetznerCluster{}
		hetznerCluster.Name = hetznerClusterName

		capiMachine := clusterv1.Machine{}

		if isControlPlane {
			// set label on capi machine to mark it as control plane
			capiMachine.Labels = make(map[string]string)
			capiMachine.Labels[clusterv1.MachineControlPlaneLabel] = "control-plane"
		}

		service := Service{
			scope: &scope.MachineScope{
				HCloudMachine: &hcloudMachine,
				Machine:       &capiMachine,
				ClusterScope: scope.ClusterScope{
					HetznerCluster: &hetznerCluster,
				},
			},
		}
		Expect(service.createLabels()).To(Equal(expectedOutput))
	},
	Entry("is_controlplane", "hcloudCluster", "hcloudMachine", true, map[string]string{
		"caph-cluster-hcloudCluster": string(infrav1.ResourceLifecycleOwned),
		infrav1.MachineNameTagKey:    "hcloudMachine",
		"machine_type":               "control_plane",
	}),
	Entry("is_worker", "hcloudCluster", "hcloudMachine", false, map[string]string{
		"caph-cluster-hcloudCluster": string(infrav1.ResourceLifecycleOwned),
		infrav1.MachineNameTagKey:    "hcloudMachine",
		"machine_type":               "worker",
	}),
)

var _ = Describe("filterHCloudSSHKeys", func() {
	var sshKeysAPI []*hcloud.SSHKey
	BeforeEach(func() {
		sshKeysAPI = []*hcloud.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
				ID:          42,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:3g",
				Name:        "sshkey2",
				ID:          43,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4h",
				Name:        "sshkey3",
				ID:          44,
			},
		}
	})
	_ = DescribeTable("no_error",
		func(sshKeysSpec []infrav1.SSHKey, expectedOutput []*hcloud.SSHKey) {
			Expect(filterHCloudSSHKeys(sshKeysAPI, sshKeysSpec)).Should(Equal(expectedOutput))
		},
		Entry("no_error_same_length", []infrav1.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:3g",
				Name:        "sshkey2",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
				Name:        "sshkey3",
			},
		}, []*hcloud.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
				ID:          42,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:3g",
				Name:        "sshkey2",
				ID:          43,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4h",
				Name:        "sshkey3",
				ID:          44,
			},
		}),
		Entry("no_error_different_length", []infrav1.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
				Name:        "sshkey3",
			},
		}, []*hcloud.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
				ID:          42,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4h",
				Name:        "sshkey3",
				ID:          44,
			},
		}),
	)

	It("should error", func() {
		_, err := filterHCloudSSHKeys(sshKeysAPI, []infrav1.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:5i",
				Name:        "sshkey4",
			},
		})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("handleServerStatusOff", func() {
	var hcloudMachine *infrav1.HCloudMachine
	client := fakeclient.NewHCloudClientFactory().NewClient("")

	server, err := client.CreateServer(context.Background(), hcloud.ServerCreateOpts{Name: "serverName"})
	Expect(err).To(Succeed())

	BeforeEach(func() {
		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloudMachineName",
				Namespace: "default",
			},
			Spec: infrav1.HCloudMachineSpec{
				ImageName: "fedora-control-plane",
				Type:      "cpx31",
			},
		}

		server.Status = hcloud.ServerStatusOff
	})

	It("sets a condition if none is previously set", func() {
		service := newTestService(hcloudMachine, client)
		res, err := service.handleServerStatusOff(context.Background(), server)
		Expect(err).To(Succeed())
		Expect(res).Should(Equal(reconcile.Result{RequeueAfter: 30 * time.Second}))
		Expect(conditions.GetReason(hcloudMachine, infrav1.ServerAvailableCondition)).To(Equal(infrav1.ServerOffReason))
		Expect(server.Status).To(Equal(hcloud.ServerStatusRunning))
	})

	It("tries to power on server again if it is not timed out", func() {
		conditions.MarkFalse(hcloudMachine, infrav1.ServerAvailableCondition, infrav1.ServerOffReason, clusterv1.ConditionSeverityInfo, "")
		service := newTestService(hcloudMachine, client)
		res, err := service.handleServerStatusOff(context.Background(), server)
		Expect(err).To(Succeed())
		Expect(res).Should(Equal(reconcile.Result{RequeueAfter: 30 * time.Second}))
		Expect(server.Status).To(Equal(hcloud.ServerStatusRunning))
		Expect(conditions.GetReason(hcloudMachine, infrav1.ServerAvailableCondition)).To(Equal(infrav1.ServerOffReason))
	})

	It("sets a failure message if it timed out", func() {
		conditions.MarkFalse(hcloudMachine, infrav1.ServerAvailableCondition, infrav1.ServerOffReason, clusterv1.ConditionSeverityInfo, "")
		// manipulate lastTransitionTime
		conditionsList := hcloudMachine.GetConditions()
		for i, c := range conditionsList {
			if c.Type == infrav1.ServerAvailableCondition {
				conditionsList[i].LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Hour))
			}
		}
		service := newTestService(hcloudMachine, client)
		res, err := service.handleServerStatusOff(context.Background(), server)
		Expect(err).To(Succeed())
		var emptyResult reconcile.Result
		Expect(res).Should(Equal(emptyResult))
		Expect(server.Status).To(Equal(hcloud.ServerStatusOff))
		Expect(hcloudMachine.Status.FailureMessage).Should(Equal(pointer.String("reached timeout of waiting for machines that are switched off")))
	})

	It("tries to power on server and sets new condition if different one is set", func() {
		conditions.MarkTrue(hcloudMachine, infrav1.ServerAvailableCondition)
		service := newTestService(hcloudMachine, client)
		res, err := service.handleServerStatusOff(context.Background(), server)
		Expect(err).To(Succeed())
		Expect(res).Should(Equal(reconcile.Result{RequeueAfter: 30 * time.Second}))
		Expect(conditions.GetReason(hcloudMachine, infrav1.ServerAvailableCondition)).To(Equal(infrav1.ServerOffReason))
		Expect(server.Status).To(Equal(hcloud.ServerStatusRunning))
	})
})

var _ = Describe("Test ValidateLabels", func() {
	type testCaseValidateLabels struct {
		gotLabels   map[string]string
		wantLabels  map[string]string
		expectError error
	}

	DescribeTable("Test ValidateLabels",
		func(tc testCaseValidateLabels) {
			err := validateLabels(&hcloud.Server{Labels: tc.gotLabels}, tc.wantLabels)

			if tc.expectError != nil {
				Expect(err).To(MatchError(tc.expectError))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("exact equality", testCaseValidateLabels{
			gotLabels:   map[string]string{"key1": "val1", "key2": "val2"},
			wantLabels:  map[string]string{"key1": "val1", "key2": "val2"},
			expectError: nil,
		}),
		Entry("subset of labels", testCaseValidateLabels{
			gotLabels:   map[string]string{"key1": "val1", "otherkey": "otherval", "key2": "val2"},
			wantLabels:  map[string]string{"key1": "val1", "key2": "val2"},
			expectError: nil,
		}),
		Entry("wrong value", testCaseValidateLabels{
			gotLabels:   map[string]string{"key1": "val1", "otherkey": "otherval", "key2": "otherval"},
			wantLabels:  map[string]string{"key1": "val1", "key2": "val2"},
			expectError: errWrongLabel,
		}),
		Entry("missing key", testCaseValidateLabels{
			gotLabels:   map[string]string{"key1": "val1", "otherkey": "otherval"},
			wantLabels:  map[string]string{"key1": "val1", "key2": "val2"},
			expectError: errMissingLabel,
		}),
	)
})
