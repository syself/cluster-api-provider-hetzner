package host

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioner"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	bareMetalHostID = 1
	sshFingerprint  = "my-fingerprint"
)

type timeoutError struct {
	error
}

func (e timeoutError) Timeout() bool {
	return true
}

func (e timeoutError) Error() string {
	return "timeout"
}

func TestHost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Host Suite")
}

var (
	ctx     = ctrl.SetupSignalHandler()
	log     = klogr.New()
	timeout = timeoutError{errors.New("timeout")}
)

func newTestHostStateMachine(host *infrav1.HetznerBareMetalHost, service *Service) *hostStateMachine {
	return newHostStateMachine(host, service)
}

func newTestService(
	host *infrav1.HetznerBareMetalHost,
	provisioner provisioner.Provisioner,
	sshClient sshclient.Client,
	robotClient robotclient.Client,
) *Service {
	scheme := runtime.NewScheme()
	utilruntime.Must(infrav1.AddToScheme(scheme))
	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(host).Build()
	return &Service{
		&scope.BareMetalHostScope{
			Logger:               &log,
			Client:               c,
			SSHClient:            sshClient,
			RobotClient:          robotClient,
			HetznerBareMetalHost: host,
			HetznerCluster: &infrav1.HetznerCluster{
				Spec: getDefaultHetznerClusterSpec(),
			},
		},
	}
}

type hostOpts func(*infrav1.HetznerBareMetalHost)

func bareMetalHost(opts ...hostOpts) *infrav1.HetznerBareMetalHost {
	host := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-host",
			Namespace: "default",
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			ServerID: bareMetalHostID,
			Status: infrav1.ControllerGeneratedStatus{
				HetznerRobotSSHKey: &infrav1.SSHKey{
					Name:        "sshkey",
					Fingerprint: sshFingerprint,
				},
				ResetTypes: []infrav1.ResetType{
					infrav1.ResetTypeSoftware,
					infrav1.ResetTypeHardware,
					infrav1.ResetTypePower,
				},
			},
		},
	}
	for _, o := range opts {
		o(host)
	}
	return host
}

func withError(errorType infrav1.ErrorType, errorMessage string, errorCount int, lastUpdated metav1.Time) hostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.ErrorType = errorType
		host.Spec.Status.ErrorMessage = errorMessage
		host.Spec.Status.ErrorCount = errorCount
		host.Spec.Status.LastUpdated = &lastUpdated
	}
}

func withResetTypes(resetTypes []infrav1.ResetType) hostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.ResetTypes = resetTypes
	}
}

func getDefaultHetznerClusterSpec() infrav1.HetznerClusterSpec {
	return infrav1.HetznerClusterSpec{
		ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
			Algorithm: "round_robin",
			ExtraServices: []infrav1.LoadBalancerServiceSpec{
				{
					DestinationPort: 8132,
					ListenPort:      8132,
					Protocol:        "tcp",
				},
				{
					DestinationPort: 8133,
					ListenPort:      8133,
					Protocol:        "tcp",
				},
			},
			Port:   6443,
			Region: "fsn1",
			Type:   "lb11",
		},
		ControlPlaneEndpoint: &clusterv1.APIEndpoint{},
		ControlPlaneRegions:  []infrav1.Region{"fsn1"},
		HCloudNetwork: infrav1.HCloudNetworkSpec{
			CIDRBlock:       "10.0.0.0/16",
			Enabled:         true,
			NetworkZone:     "eu-central",
			SubnetCIDRBlock: "10.0.0.0/24",
		},
		HCloudPlacementGroup: []infrav1.HCloudPlacementGroupSpec{
			{
				Name: "control-plane",
				Type: "spread",
			},
			{
				Name: "md-0",
				Type: "spread",
			},
		},
		HetznerSecret: infrav1.HetznerSecretRef{
			Key: infrav1.HetznerSecretKeyRef{
				HCloudToken: "hcloud",
			},
			Name: "hetzner-secret",
		},
		SSHKeys: infrav1.HetznerSSHKeys{
			HCloud: []infrav1.SSHKey{
				{
					Name: "testsshkey",
				},
			},
		},
	}
}
