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
	"github.com/mitchellh/copystructure"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/conditions"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	fakehcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

func Test_statusAddresses(t *testing.T) {
	server := newTestServer()

	// Create deep copy.
	saved, err := copystructure.Copy(server)
	require.NoError(t, err)

	addresses := statusAddresses(server)

	// should have three addresses
	require.Equal(t, 3, len(addresses))

	// should have the right address IPs
	ips := []string{"1.2.3.4", "2001:db8::1", "10.0.0.2"}
	for i, addr := range addresses {
		require.Equal(t, ips[i], addr.Address)
	}

	// Check that input was not altered.
	require.Equal(t, saved, server)

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
				SSHClientFactory: testEnv.HCloudSSHClientFactory,
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

var _ = Describe("handleServerStatusOff", func() {
	var hcloudMachine *infrav1.HCloudMachine
	var server *hcloud.Server
	var client hcloudclient.Client
	BeforeEach(func() {
		client = fakehcloudclient.NewHCloudClientFactory().NewClient("")

		var err error
		server, err = client.CreateServer(context.Background(), hcloud.ServerCreateOpts{Name: "serverName"})
		Expect(err).To(Succeed())

		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-machine",
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

		_, exists := service.scope.Machine.Annotations[clusterv1.RemediateMachineAnnotation]
		Expect(exists).To(BeTrue())
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

var _ = Describe("getSSHKeys", func() {
	var (
		service      *Service
		hcloudClient *mocks.Client
	)

	BeforeEach(func() {
		hcloudClient = mocks.NewClient(GinkgoT())
		clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
			Client:       testEnv.Manager.GetClient(),
			APIReader:    testEnv.Manager.GetAPIReader(),
			HCloudClient: hcloudClient,
			Logger:       GinkgoLogr,

			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clustername",
					Namespace: "default",
				},
			},

			HetznerCluster: &infrav1.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clustername",
					Namespace: "default",
				},
				Spec: infrav1.HetznerClusterSpec{
					HetznerSecret: infrav1.HetznerSecretRef{
						Name: "secretname",
						Key: infrav1.HetznerSecretKeyRef{
							SSHKey: "hcloud-ssh-key-name",
						},
					},
				},
			},

			HetznerSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secretname",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"hcloud-ssh-key-name": []byte("sshKey1"),
				},
			},
		})
		Expect(err).To(BeNil())

		service = &Service{
			scope: &scope.MachineScope{
				ClusterScope: *clusterScope,
				HCloudMachine: &infrav1.HCloudMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-machine",
						Namespace: "default",
					},
				},
				SSHClientFactory: testEnv.HCloudSSHClientFactory,
			},
		}
	})

	AfterEach(func() {
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
	})

	It("uses HCloudMachine.Spec.SSHKeys if present", func() {
		By("populating the HCloudMachine.Spec.SSHKeys")
		service.scope.HCloudMachine.Spec.SSHKeys = []infrav1.SSHKey{
			{
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
			{
				Name:        "sshKey3",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
			},
		}

		By("ensuring that the mocked hcloud client returns all the ssh keys")
		sshKeysByHCloudClient := []*hcloud.SSHKey{
			{
				ID:          1,
				Name:        "sshKey1",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f",
			},
			{
				ID:          2,
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
			{
				ID:          3,
				Name:        "sshKey3",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
			},
		}

		hcloudClient.On("ListSSHKeys", mock.Anything, mock.Anything).Return(sshKeysByHCloudClient, nil)

		By("ensuring that the getSSHKeys method returns all the referenced ssh keys")
		caphSSHKeys, hcloudSSHKeys, err := service.getSSHKeys(context.Background())
		Expect(err).To(BeNil())

		Expect(caphSSHKeys).To(ConsistOf([]infrav1.SSHKey{
			{
				Name: "sshKey1",
			},
			{
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
			{
				Name:        "sshKey3",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
			},
		}))

		Expect(hcloudSSHKeys).To(ConsistOf(sshKeysByHCloudClient))
	})

	It("falls back to HetznerCluster.Spec.SSHKeys.HCloud, if HCloudMachine.Spec.SSHKeys is empty", func() {
		By("populating the HCloudMachine.Spec.SSHKeys")
		service.scope.HetznerCluster.Spec.SSHKeys.HCloud = []infrav1.SSHKey{
			{
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
			{
				Name:        "sshKey3",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
			},
		}

		By("ensuring that the mocked hcloud client returns all the ssh keys")
		sshKeysByHCloudClient := []*hcloud.SSHKey{
			{
				ID:          1,
				Name:        "sshKey1",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f",
			},
			{
				ID:          2,
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
			{
				ID:          3,
				Name:        "sshKey3",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
			},
		}

		hcloudClient.On("ListSSHKeys", mock.Anything, mock.Anything).Return(sshKeysByHCloudClient, nil)

		By("ensuring that the getSSHKeys method returns all the referenced ssh keys")
		caphSSHKeys, hcloudSSHKeys, err := service.getSSHKeys(context.Background())
		Expect(err).To(BeNil())

		Expect(caphSSHKeys).To(ConsistOf([]infrav1.SSHKey{
			{
				Name: "sshKey1",
			},
			{
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
			{
				Name:        "sshKey3",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
			},
		}))

		Expect(hcloudSSHKeys).To(ConsistOf(sshKeysByHCloudClient))
	})

	It("one of the ssh key defined in HCloudMachine.Spec.SSHKeys is not present in hcloud", func() {
		By("populating the HCloudMachine.Spec.SSHKeys")
		service.scope.HCloudMachine.Spec.SSHKeys = []infrav1.SSHKey{
			{
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
			{
				Name:        "sshKey3",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
			},
		}

		By("ensuring that the mocked hcloud client doesn't return one of the ssh key")
		sshKeysByHCloudClient := []*hcloud.SSHKey{
			{
				ID:          1,
				Name:        "sshKey1",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f",
			},
			{
				ID:          2,
				Name:        "sshKey2",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
			},
		}

		hcloudClient.On("ListSSHKeys", mock.Anything, mock.Anything).Return(sshKeysByHCloudClient, nil)

		By("ensuring that the getSSHKeys method fails")
		_, _, err := service.getSSHKeys(context.Background())
		Expect(err).ToNot(BeNil())
	})

	It("adds secret SSH key if not already present", func() {
		// no machine keys, secretKey should be added

		sshKeysByHCloudClient := []*hcloud.SSHKey{
			{
				ID:          1,
				Name:        "sshKey1",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f",
			},
		}

		hcloudClient.On("ListSSHKeys", mock.Anything, mock.Anything).Return(sshKeysByHCloudClient, nil)

		caphKeys, hcloudSSHKeys, err := service.getSSHKeys(context.Background())
		Expect(err).ToNot(HaveOccurred())

		Expect(caphKeys).To(ConsistOf([]infrav1.SSHKey{
			{
				Name: "sshKey1",
			},
		}))

		Expect(hcloudSSHKeys).To(ConsistOf(sshKeysByHCloudClient))
	})

	It("does not duplicate secret SSH key if already in list", func() {
		sshKeyName := "sshKey1"
		sshKeyFingerprint := "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f"

		service.scope.HCloudMachine.Spec.SSHKeys = []infrav1.SSHKey{
			{
				Name:        sshKeyName,
				Fingerprint: sshKeyFingerprint,
			},
		}

		sshKeysByHCloudClient := []*hcloud.SSHKey{
			{
				ID:          1,
				Name:        sshKeyName,
				Fingerprint: sshKeyFingerprint,
			},
		}

		hcloudClient.On("ListSSHKeys", mock.Anything, mock.Anything).Return(sshKeysByHCloudClient, nil)

		caphKeys, hcloudSSHKeys, err := service.getSSHKeys(context.Background())
		Expect(err).ToNot(HaveOccurred())

		Expect(caphKeys).To(ConsistOf([]infrav1.SSHKey{
			{
				Name:        sshKeyName,
				Fingerprint: sshKeyFingerprint,
			},
		}))

		Expect(hcloudSSHKeys).To(ConsistOf(sshKeysByHCloudClient))
	})
})

var _ = Describe("Reconcile", func() {
	var (
		service      *Service
		testNs       *corev1.Namespace
		hcloudClient *mocks.Client
	)

	testScheme := runtime.NewScheme()
	err := infrav1.AddToScheme(testScheme)
	Expect(err).To(BeNil())

	BeforeEach(func() {
		hcloudClient = mocks.NewClient(GinkgoT())
		testNs, err = testEnv.ResetAndCreateNamespace(ctx, "server-reconcile")
		Expect(err).To(BeNil())

		err = testEnv.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrapsecret",
				Namespace: testNs.Name,
			},
			Data: map[string][]byte{
				"value": []byte("dummy-bootstrap-data"),
			},
		})
		Expect(err).To(BeNil())

		clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
			Client:       testEnv.Manager.GetClient(),
			APIReader:    testEnv.Manager.GetAPIReader(),
			HCloudClient: hcloudClient,
			Logger:       GinkgoLogr,

			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clustername",
					Namespace: testNs.Name,
				},
			},

			HetznerCluster: &infrav1.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clustername",
					Namespace: testNs.Name,
				},
				Spec: infrav1.HetznerClusterSpec{
					HetznerSecret: infrav1.HetznerSecretRef{
						Name: "secretname",
						Key: infrav1.HetznerSecretKeyRef{
							SSHKey: "hcloud-ssh-key-name",
						},
					},
					SSHKeys: infrav1.HetznerSSHKeys{
						HCloud: []infrav1.SSHKey{},
						RobotRescueSecretRef: infrav1.SSHSecretRef{
							Name: "rescue-ssh-secret",
							Key: infrav1.SSHSecretKeyRef{
								Name:       "sshkey-name",
								PublicKey:  "public-key",
								PrivateKey: "private-key",
							},
						},
					},
				},
			},

			HetznerSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secretname",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"hcloud-ssh-key-name": []byte("sshKey1"),
				},
			},
		})

		Expect(err).To(BeNil())

		err = testEnv.Create(ctx, helpers.GetDefaultSSHSecret("rescue-ssh-secret", testNs.Name))
		Expect(err).To(BeNil())

		hcloudMachine := &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-machine",
				Namespace: testNs.Name,
			},
			Spec: infrav1.HCloudMachineSpec{
				Type:      "cpx11",
				ImageName: "ubuntu-24.04",
				SSHKeys: []infrav1.SSHKey{
					{
						Name:        "sshKey1",
						Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f",
					},
				},
			},
		}
		Expect(testEnv.Create(ctx, hcloudMachine)).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return testEnv.Get(ctx, client.ObjectKeyFromObject(hcloudMachine), hcloudMachine)
		}).Should(Succeed())
		Expect(hcloudMachine.Kind).To(Equal("HCloudMachine"))

		capiMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-machine",
				Namespace: testNs.Name,
			},
			Spec: clusterv1.MachineSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					Name:     hcloudMachine.Name,
					Kind:     hcloudMachine.Kind,
					APIGroup: infrav1.GroupVersion.Group,
				},
				ClusterName:   "clustername",
				FailureDomain: "nbg1",
				Bootstrap: clusterv1.Bootstrap{
					ConfigRef:      clusterv1.ContractVersionedObjectReference{},
					DataSecretName: ptr.To("bootstrapsecret"),
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).ShouldNot(HaveOccurred())

		err = controllerutil.SetOwnerReference(capiMachine, hcloudMachine, testEnv.Scheme())
		Expect(err).ShouldNot(HaveOccurred())

		service = &Service{
			scope: &scope.MachineScope{
				ClusterScope:     *clusterScope,
				Machine:          capiMachine,
				HCloudMachine:    hcloudMachine,
				SSHClientFactory: testEnv.HCloudSSHClientFactory,
			},
		}
		service.scope.Client = testEnv.Client

		Eventually(func() error {
			capiMachine, err = util.GetOwnerMachine(ctx, service.scope.Client,
				service.scope.HCloudMachine.ObjectMeta)
			if err != nil {
				return err
			}
			Expect(capiMachine).NotTo(BeNil())
			return nil
		})
	})

	AfterEach(func() {
		Expect(hcloudClient.AssertExpectations(GinkgoT())).To(BeTrue())
		Expect(testEnv.Cleanup(ctx, testNs)).To(Succeed())
	})

	It("sets the region in status of hcloudMachine, by fetching the failure domain from machine.spec", func() {
		By("calling reconcile")
		_, err := service.Reconcile(ctx)
		Expect(err).To(BeNil())

		By("ensuring the region is set in the status of hcloudMachine")
		Expect(service.scope.HCloudMachine.Status.Region).To(Equal(infrav1.Region("nbg1")))

		By("ensuring the BootstrapReady condition is marked as false")
		Expect(isPresentAndFalseWithReason(service.scope.HCloudMachine, infrav1.BootstrapReadyCondition, infrav1.BootstrapNotReadyReason)).To(BeTrue())
	})

	It("sets the region in status of hcloudMachine, by fetching the failure domain from cluster.status if machine.spec.failureDomain is empty", func() {
		By("setting the failure domain in cluster.status")
		service.scope.Machine.Spec = clusterv1.MachineSpec{}
		service.scope.Cluster.Status.FailureDomains = []clusterv1.FailureDomain{{
			Name: "nbg1",
		}}

		By("calling reconcile")
		_, err := service.Reconcile(ctx)
		Expect(err).To(BeNil())

		By("ensuring the region is set in the status of hcloudMachine")
		Expect(service.scope.HCloudMachine.Status.Region).To(Equal(infrav1.Region("nbg1")))
	})

	It("sets the CreateMachineError if the ProviderID is set on the HCloudMachine but the actual server was not found in the cloud", func() {
		By("setting the bootstrap data")
		err = testEnv.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrapsecret",
				Namespace: testNs.Name,
			},
			Data: map[string][]byte{
				"value": []byte("dummy-bootstrap-data"),
			},
		})
		Expect(err).To(BeNil())

		By("setting the ProviderID on the HCloudMachine")
		service.scope.HCloudMachine.Spec.ProviderID = ptr.To("hcloud://1234567")
		err = testEnv.Update(ctx, service.scope.HCloudMachine)
		Expect(err).To(BeNil())

		service.scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("bootstrapsecret")

		By("ensuring that the hcloud client returns both server and error as nil")
		hcloudClient.On("GetServer", mock.Anything, int64(1234567)).Return(nil, nil)
		hcloudClient.On("ListServers", mock.Anything, mock.Anything).Return(nil, nil)

		By("calling reconcile")
		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())

		By("validating if CreateMachineError was set on HCloudMachine object")
		c := conditions.Get(service.scope.HCloudMachine, infrav1.RemediationSucceededCondition)
		Expect(c).NotTo(BeNil())
		Expect(c.Status).To(Equal(metav1.ConditionFalse))
		Expect(c.Message).To(Equal(`hcloud server ("hcloud://1234567") no longer available. Setting MachineError.`))
	})

	It("transitions the BootStrate from BootStateUnset -> BootStateBootingToRealOS -> BootStateOperatingSystemRunning (imageName)", func() {
		By("setting the bootstrap data")
		err = testEnv.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrapsecret",
				Namespace: testNs.Name,
			},
			Data: map[string][]byte{
				"value": []byte("dummy-bootstrap-data"),
			},
		})
		Expect(err).To(BeNil())

		service.scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("bootstrapsecret")

		hcloudClient.On("GetServerType", mock.Anything, mock.Anything).Return(&hcloud.ServerType{
			Architecture: hcloud.ArchitectureX86,
		}, nil)

		hcloudClient.On("ListImages", mock.Anything, hcloud.ImageListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: "caph-image-name==ubuntu-24.04",
			},
			Architecture: []hcloud.Architecture{hcloud.ArchitectureX86},
		}).Return([]*hcloud.Image{
			{
				ID:   123456,
				Name: "ubuntu",
			},
		}, nil)

		hcloudClient.On("ListImages", mock.Anything, hcloud.ImageListOpts{
			Name:         "ubuntu-24.04",
			Architecture: []hcloud.Architecture{hcloud.ArchitectureX86},
		}).Return([]*hcloud.Image{}, nil)

		hcloudClient.On("ListSSHKeys", mock.Anything, mock.Anything).Return([]*hcloud.SSHKey{
			{
				ID:          1,
				Name:        "sshKey1",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f",
			},
		}, nil)

		hcloudClient.On("CreateServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:     1,
			Name:   "my-machine",
			Status: hcloud.ServerStatusInitializing,
		}, nil)

		By("calling reconcile")
		_, err := service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to BootStateBootingToRealOS")

		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateBootingToRealOS))

		By("reconciling again")
		hcloudClient.On("GetServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:     1,
			Name:   "my-machine",
			Status: hcloud.ServerStatusRunning,
		}, nil)

		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to BootStateOperatingSystemRunning once the server's status changes to running")
		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateOperatingSystemRunning))
	})

	It("transitions to BootStateOperatingSystemRunning (imageURL)", func() {
		By("setting the bootstrap data")
		err = testEnv.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrapsecret",
				Namespace: testNs.Name,
			},
			Data: map[string][]byte{
				"value": []byte("dummy-bootstrap-data"),
			},
		})
		Expect(err).To(BeNil())
		service.scope.ImageURLCommand = "dummy-image-url-command.sh"
		service.scope.HCloudMachine.Spec.ImageName = ""
		service.scope.HCloudMachine.Spec.ImageURL = "oci://example.com/repo/image:v1"

		service.scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("bootstrapsecret")

		hcloudClient.On("GetServerType", mock.Anything, mock.Anything).Return(&hcloud.ServerType{
			Architecture: hcloud.ArchitectureX86,
		}, nil)

		hcloudClient.On("ListImages", mock.Anything, hcloud.ImageListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: "caph-image-name==ubuntu-24.04",
			},
			Architecture: []hcloud.Architecture{hcloud.ArchitectureX86},
		}).Return([]*hcloud.Image{
			{
				ID:   123456,
				Name: "ubuntu",
			},
		}, nil)

		hcloudClient.On("ListImages", mock.Anything, hcloud.ImageListOpts{
			Name:         "ubuntu-24.04",
			Architecture: []hcloud.Architecture{hcloud.ArchitectureX86},
		}).Return([]*hcloud.Image{}, nil)

		hcloudClient.On("ListSSHKeys", mock.Anything, mock.Anything).Return([]*hcloud.SSHKey{
			{
				ID:          1,
				Name:        "sshKey1",
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:1f",
			},
		}, nil)

		hcloudClient.On("CreateServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:     1,
			Name:   "my-machine",
			Status: hcloud.ServerStatusInitializing,
		}, nil)

		By("calling reconcile")
		_, err := service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to Initializing")

		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateInitializing))

		By("reconciling again")
		hcloudClient.On("GetServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:     1,
			Name:   "my-machine",
			Status: hcloud.ServerStatusRunning,
		}, nil).Once()

		startTime := time.Now()
		hcloudClient.On("EnableRescueSystem", mock.Anything, mock.Anything, mock.Anything).Return(
			hcloud.ServerEnableRescueResult{
				Action: &hcloud.Action{
					ID:           334455,
					Status:       hcloud.ActionStatusRunning,
					Command:      "",
					Progress:     0,
					Started:      startTime,
					Finished:     time.Time{},
					ErrorCode:    "",
					ErrorMessage: "",
					Resources:    []*hcloud.ActionResource{},
				},
				RootPassword: "",
			}, nil).Once()
		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to EnablingRescue")
		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateEnablingRescue))

		By("reconcile again --------------------------------------------------------")
		hcloudClient.On("GetServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:            1,
			Name:          "my-machine",
			RescueEnabled: true,
			Status:        hcloud.ServerStatusRunning,
		}, nil).Once()
		hcloudClient.On("GetAction", mock.Anything, mock.Anything).Return(
			&hcloud.Action{
				ID:           1,
				Status:       hcloud.ActionStatusSuccess,
				Command:      "",
				Progress:     0,
				Started:      startTime,
				Finished:     time.Now(),
				ErrorCode:    "",
				ErrorMessage: "",
				Resources:    []*hcloud.ActionResource{},
			}, nil,
		)
		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to EnablingRescue")
		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateEnablingRescue))

		By("reconcile again --------------------------------------------------------")
		hcloudClient.On("GetServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:            1,
			Name:          "hcloudmachinenameWithRescueEnabled",
			RescueEnabled: true,
			Status:        hcloud.ServerStatusRunning,
		}, nil).Once()

		testEnv.HCloudSSHClient.On("Reboot").Return(sshclient.Output{
			Err:    nil,
			StdOut: "ok",
			StdErr: "",
		})
		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to BootingToRescue")
		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateBootingToRescue))

		By("reconcile again --------------------------------------------------------")
		hcloudClient.On("GetServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:     1,
			Name:   "hcloudmachinenameWithRescueEnabled",
			Status: hcloud.ServerStatusRunning,
		}, nil).Once()
		testEnv.HCloudSSHClient.On("GetHostName").Return(sshclient.Output{
			StdOut: "rescue",
			StdErr: "",
			Err:    nil,
		})
		startImageURLCommandMock := testEnv.HCloudSSHClient.On("StartImageURLCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "my-machine", []string{"sda"}).Return(0, "", nil)
		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to RunningImageCommand")
		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateRunningImageCommand))
		startImageURLCommandMock.Parent.AssertNumberOfCalls(GinkgoT(), "StartImageURLCommand", 1)

		By("reconcile again --------------------------------------------------------")
		testEnv.HCloudSSHClient.On("GetHostName").Return(sshclient.Output{
			StdOut: "rescue",
			StdErr: "",
			Err:    nil,
		})
		testEnv.HCloudSSHClient.On("StateOfImageURLCommand").Return(sshclient.ImageURLCommandStateFinishedSuccessfully, "output-of-image-url-command", nil)
		hcloudClient.On("GetServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:            1,
			Name:          "hcloudmachinenameWithRescueEnabled",
			RescueEnabled: true,
			Status:        hcloud.ServerStatusRunning,
		}, nil).Once()
		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())

		By("ensuring the bootstate has transitioned to BootingToRealOS")
		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateBootingToRealOS))

		By("reconcile again --------------------------------------------------------")
		hcloudClient.On("GetServer", mock.Anything, mock.Anything).Return(&hcloud.Server{
			ID:            1,
			Name:          "hcloudmachinenameWithRescueEnabled",
			RescueEnabled: true,
			Status:        hcloud.ServerStatusRunning,
		}, nil).Once()
		_, err = service.Reconcile(ctx)
		Expect(err).To(BeNil())
		Expect(noErrorOccured(service.scope)).To(BeNil())
		By("ensuring the bootstate has transitioned to OperatingSystemRunning")
		Expect(service.scope.HCloudMachine.Status.BootState).To(Equal(infrav1.HCloudBootStateOperatingSystemRunning))
	})
})

func isPresentAndFalseWithReason(getter capiconditions.Getter, condition string, reason string) bool {
	if !conditions.Has(getter, condition) {
		return false
	}
	objectCondition := conditions.Get(getter, condition)
	return objectCondition.Status == metav1.ConditionFalse &&
		objectCondition.Reason == reason
}

func noErrorOccured(s *scope.MachineScope) error {
	c := conditions.Get(s.HCloudMachine, infrav1.RemediationSucceededCondition)
	if c == nil {
		return nil
	}
	if c.Status == metav1.ConditionTrue {
		return nil
	}
	return fmt.Errorf("Error on HCloudMachine: %s: %s", c.Reason, c.Message)
}
