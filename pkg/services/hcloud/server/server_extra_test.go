package server

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
)

var _ = Describe("handleOperatingSystemRunning", func() {
	var (
		hcloudMachine  *infrav1.HCloudMachine
		hetznerCluster *infrav1.HetznerCluster
		capiMachine    *clusterv1.Machine
		server         *hcloud.Server
		mockClient     *mocks.Client
		service        *Service
	)

	BeforeEach(func() {
		mockClient = mocks.NewClient(GinkgoT())
		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloudMachine",
				Namespace: "default",
			},
			Status: infrav1.HCloudMachineStatus{
				Ready: false,
			},
		}
		hetznerCluster = &infrav1.HetznerCluster{
			Status: infrav1.HetznerClusterStatus{
				Network: &infrav1.NetworkStatus{
					ID:              1,
					AttachedServers: []int64{},
				},
			},
		}
		capiMachine = &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{},
			},
		}
		server = &hcloud.Server{
			ID:     1,
			Status: hcloud.ServerStatusRunning,
			PublicNet: hcloud.ServerPublicNet{
				IPv4: hcloud.ServerPublicNetIPv4{
					IP: net.ParseIP("1.2.3.4"),
				},
			},
		}

		service = &Service{
			scope: &scope.MachineScope{
				HCloudMachine: hcloudMachine,
				Machine:       capiMachine,
				ClusterScope: scope.ClusterScope{
					Cluster:        &clusterv1.Cluster{},
					HetznerCluster: hetznerCluster,
					HCloudClient:   mockClient,
				},
			},
		}
	})

	It("should mark ready if worker node and network attached", func() {
		// Worker node (default)

		// Network attached
		hetznerCluster.Status.Network.AttachedServers = []int64{1}

		res, err := service.handleOperatingSystemRunning(context.Background(), server)
		Expect(err).To(Succeed())
		Expect(res).To(Equal(reconcile.Result{}))
		Expect(hcloudMachine.Status.Ready).To(BeTrue())
		Expect(conditions.IsTrue(hcloudMachine, infrav1.ServerAvailableCondition)).To(BeTrue())
	})

	It("should attach network if not attached", func() {
		mockClient.On("AttachServerToNetwork", mock.Anything, mock.MatchedBy(func(s *hcloud.Server) bool {
			return s.ID == 1
		}), mock.Anything).Return(nil)

		res, err := service.handleOperatingSystemRunning(context.Background(), server)
		Expect(err).To(Succeed())
		Expect(res).To(Equal(reconcile.Result{}))
		Expect(hcloudMachine.Status.Ready).To(BeTrue())
	})

	It("should handle network attachment error", func() {
		mockClient.On("AttachServerToNetwork", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("error"))

		_, err := service.handleOperatingSystemRunning(context.Background(), server)
		Expect(err).To(HaveOccurred())
		Expect(conditions.IsFalse(hcloudMachine, infrav1.ServerAvailableCondition)).To(BeTrue())
	})

	It("should handle control plane load balancer attachment", func() {
		capiMachine.Labels[clusterv1.MachineControlPlaneLabel] = "control-plane"
		hetznerCluster.Status.ControlPlaneLoadBalancer = &infrav1.LoadBalancerStatus{
			ID:     2,
			Target: []infrav1.LoadBalancerTarget{},
		}
		// Network already attached (so we don't need to mock AttachServerToNetwork)
		hetznerCluster.Status.Network.AttachedServers = []int64{1}

		// Assume API server pod is healthy
		conditions.MarkTrue(capiMachine, controlplanev1.MachineAPIServerPodHealthyCondition)

		// Mock AddTargetServerToLoadBalancer
		mockClient.On("AddTargetServerToLoadBalancer", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		res, err := service.handleOperatingSystemRunning(context.Background(), server)
		Expect(err).To(Succeed())
		Expect(res).To(Equal(reconcile.Result{}))
		Expect(hcloudMachine.Status.Ready).To(BeTrue())
	})
})

var _ = Describe("Delete", func() {
	var (
		hcloudMachine  *infrav1.HCloudMachine
		hetznerCluster *infrav1.HetznerCluster
		capiMachine    *clusterv1.Machine
		server         *hcloud.Server
		mockClient     *mocks.Client
		service        *Service
	)

	BeforeEach(func() {
		mockClient = mocks.NewClient(GinkgoT())
		hcloudMachine = &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloudMachine",
				Namespace: "default",
			},
			Spec: infrav1.HCloudMachineSpec{
				ProviderID: ptr.To("hcloud://1"),
			},
		}
		hetznerCluster = &infrav1.HetznerCluster{}
		capiMachine = &clusterv1.Machine{}
		server = &hcloud.Server{
			ID:     1,
			Status: hcloud.ServerStatusRunning,
		}

		service = &Service{
			scope: &scope.MachineScope{
				HCloudMachine: hcloudMachine,
				Machine:       capiMachine,
				ClusterScope: scope.ClusterScope{
					Cluster:        &clusterv1.Cluster{},
					HetznerCluster: hetznerCluster,
					HCloudClient:   mockClient,
				},
			},
		}
	})

	It("should return nil if server not found", func() {
		// Mock ListServers as well because findServer might fall back to it or use it if ProviderID is missing/invalid
		mockClient.On("GetServer", mock.Anything, int64(1)).Return(nil, nil).Maybe()
		mockClient.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{}, nil).Maybe()

		res, err := service.Delete(context.Background())
		Expect(err).To(Succeed())
		Expect(res).To(Equal(reconcile.Result{}))
	})

	It("should delete server if found", func() {
		mockClient.On("GetServer", mock.Anything, int64(1)).Return(server, nil).Maybe()
		mockClient.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{server}, nil).Maybe()
		mockClient.On("DeleteServer", mock.Anything, mock.MatchedBy(func(s *hcloud.Server) bool { return s.ID == 1 })).Return(nil)

		res, err := service.Delete(context.Background())
		Expect(err).To(Succeed())
		Expect(res).To(Equal(reconcile.Result{}))
	})

	It("should delete server if found and off", func() {
		server.Status = hcloud.ServerStatusOff
		mockClient.On("GetServer", mock.Anything, int64(1)).Return(server, nil).Maybe()
		mockClient.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{server}, nil).Maybe()
		mockClient.On("DeleteServer", mock.Anything, mock.MatchedBy(func(s *hcloud.Server) bool { return s.ID == 1 })).Return(nil)

		res, err := service.Delete(context.Background())
		Expect(err).To(Succeed())
		Expect(res).To(Equal(reconcile.Result{}))
	})

	It("should shutdown server if found and running", func() {
		conditions.MarkTrue(hcloudMachine, infrav1.ServerAvailableCondition)
		mockClient.On("GetServer", mock.Anything, int64(1)).Return(server, nil).Maybe()
		mockClient.On("ListServers", mock.Anything, mock.Anything).Return([]*hcloud.Server{server}, nil).Maybe()
		mockClient.On("ShutdownServer", mock.Anything, mock.MatchedBy(func(s *hcloud.Server) bool { return s.ID == 1 })).Return(nil)

		res, err := service.Delete(context.Background())
		Expect(err).To(Succeed())
		Expect(res).To(Equal(reconcile.Result{RequeueAfter: 30 * time.Second}))
	})
})
