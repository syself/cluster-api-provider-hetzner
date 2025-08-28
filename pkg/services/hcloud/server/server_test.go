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
	"fmt"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	fakeclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

func Test_getStatusAdressesFromHCloudServer(t *testing.T) {
	addresses := getStatusAdressesFromHCloudServer(newTestServer())

	// should have three addresses
	require.Equal(t, 3, len(addresses))

	// should have the right address IPs
	ips := []string{"1.2.3.4", "2001:db8::1", "10.0.0.2"}
	for i, addr := range addresses {
		require.Equal(t, ips[i], addr.Address)
	}

	// should have the right address types
	addressTypes := []clusterv1.MachineAddressType{clusterv1.MachineExternalIP, clusterv1.MachineExternalIP, clusterv1.MachineInternalIP}
	for i, addr := range addresses {
		require.Equal(t, addressTypes[i], addr.Type)
	}
}

type testCaseStatusFromHCloudServer struct {
	isControlPlane bool
	expectedOutput map[string]string
}

var _ = DescribeTable("createLabels",
	func(tc testCaseStatusFromHCloudServer) {
		hcloudMachine := infrav1.HCloudMachine{}
		hcloudMachine.Name = "hcloudMachine"

		hetznerCluster := infrav1.HetznerCluster{}
		hetznerCluster.Name = "hcloudCluster"

		capiMachine := clusterv1.Machine{}

		if tc.isControlPlane {
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
		Expect(service.createLabels()).To(Equal(tc.expectedOutput))
	},
	Entry("is_controlplane", testCaseStatusFromHCloudServer{
		isControlPlane: true,
		expectedOutput: map[string]string{
			"caph-cluster-hcloudCluster": string(infrav1.ResourceLifecycleOwned),
			infrav1.MachineNameTagKey:    "hcloudMachine",
			"machine_type":               "control_plane",
		},
	}),
	Entry("is_worker", testCaseStatusFromHCloudServer{
		isControlPlane: false,
		expectedOutput: map[string]string{
			"caph-cluster-hcloudCluster": string(infrav1.ResourceLifecycleOwned),
			infrav1.MachineNameTagKey:    "hcloudMachine",
			"machine_type":               "worker",
		},
	}),
)

var _ = Describe("filterHCloudSSHKeys", func() {
	type testCaseFilterHCloudSSHKeys struct {
		sshKeysSpec    []infrav1.SSHKey
		expectedOutput []*hcloud.SSHKey
	}

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
		func(tc testCaseFilterHCloudSSHKeys) {
			Expect(convertCaphSSHKeysToHcloudSSHKeys(sshKeysAPI, tc.sshKeysSpec)).Should(Equal(tc.expectedOutput))
		},
		Entry("no_error_same_length", testCaseFilterHCloudSSHKeys{
			sshKeysSpec: []infrav1.SSHKey{
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
			},
			expectedOutput: []*hcloud.SSHKey{
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
			},
		}),
		Entry("no_error_different_length", testCaseFilterHCloudSSHKeys{
			sshKeysSpec: []infrav1.SSHKey{
				{
					Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
					Name:        "sshkey1",
				},
				{
					Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
					Name:        "sshkey3",
				},
			},
			expectedOutput: []*hcloud.SSHKey{
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
			},
		}),
	)

	It("should error", func() {
		_, err := convertCaphSSHKeysToHcloudSSHKeys(sshKeysAPI, []infrav1.SSHKey{
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
				ImageName: "my-control-plane",
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
		Expect(hcloudMachine.Status.FailureMessage).Should(Equal(ptr.To("reached timeout of waiting for machines that are switched off")))
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

var _ = Describe("Test handleRateLimit", func() {
	type testCaseHandleRateLimit struct {
		hm              *infrav1.HCloudMachine
		err             error
		functionName    string
		errMsg          string
		expectError     error
		expectCondition bool
	}

	DescribeTable("Test handleRateLimit",
		func(tc testCaseHandleRateLimit) {
			err := handleRateLimit(tc.hm, tc.err, tc.functionName, tc.errMsg)
			if tc.expectError != nil {
				Expect(err).To(MatchError(tc.expectError))
			} else {
				Expect(err).To(BeNil())
			}
			if tc.expectCondition {
				Expect(isPresentAndFalseWithReason(tc.hm, infrav1.HetznerAPIReachableCondition, infrav1.RateLimitExceededReason)).To(BeTrue())
			} else {
				Expect(conditions.Get(tc.hm, infrav1.HetznerAPIReachableCondition)).To(BeNil())
			}
		},
		Entry("machine not ready, rate limit exceeded error", testCaseHandleRateLimit{
			hm: &infrav1.HCloudMachine{
				Status: infrav1.HCloudMachineStatus{Ready: false},
			},
			err:             hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded},
			functionName:    "TestFunction",
			errMsg:          "Test error message",
			expectError:     fmt.Errorf("Test error message: %w", hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded}),
			expectCondition: true,
		}),
		Entry("machine has deletion timestamp, rate limit exceeded error", testCaseHandleRateLimit{
			hm: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Status: infrav1.HCloudMachineStatus{Ready: true},
			},
			err:             hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded},
			functionName:    "TestFunction",
			errMsg:          "Test error message",
			expectError:     fmt.Errorf("Test error message: %w", hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded}),
			expectCondition: true,
		}),
		Entry("machine not ready, has deletion timestamp, rate limit exceeded error", testCaseHandleRateLimit{
			hm: &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Status: infrav1.HCloudMachineStatus{Ready: false},
			},
			err:             hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded},
			functionName:    "TestFunction",
			errMsg:          "Test error message",
			expectError:     fmt.Errorf("Test error message: %w", hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded}),
			expectCondition: true,
		}),
		Entry("machine ready, rate limit exceeded error", testCaseHandleRateLimit{
			hm: &infrav1.HCloudMachine{
				Status: infrav1.HCloudMachineStatus{Ready: true},
			},
			err:             hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded},
			functionName:    "TestFunction",
			errMsg:          "Test error message",
			expectError:     nil,
			expectCondition: false,
		}),
		Entry("machine ready, other error", testCaseHandleRateLimit{
			hm: &infrav1.HCloudMachine{
				Status: infrav1.HCloudMachineStatus{Ready: true},
			},
			err:             hcloud.Error{Code: hcloud.ErrorCodeResourceUnavailable},
			functionName:    "TestFunction",
			errMsg:          "Test error message",
			expectError:     fmt.Errorf("Test error message: %w", hcloud.Error{Code: hcloud.ErrorCodeResourceUnavailable}),
			expectCondition: false,
		}),
		Entry("machine not ready, other error", testCaseHandleRateLimit{
			hm: &infrav1.HCloudMachine{
				Status: infrav1.HCloudMachineStatus{Ready: false},
			},
			err:             hcloud.Error{Code: hcloud.ErrorCodeConflict},
			functionName:    "TestFunction",
			errMsg:          "Test conflict error message",
			expectError:     fmt.Errorf("Test conflict error message: %w", hcloud.Error{Code: hcloud.ErrorCodeConflict}),
			expectCondition: false,
		}),
	)
})

func isPresentAndFalseWithReason(getter conditions.Getter, condition clusterv1.ConditionType, reason string) bool {
	if !conditions.Has(getter, condition) {
		return false
	}
	objectCondition := conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionFalse &&
		objectCondition.Reason == reason
}
