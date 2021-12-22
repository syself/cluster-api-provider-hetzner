package baremetal

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log = klogr.New()
)

func TestBaremetal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Baremetal Suite")
}

func newTestService(
	bmMachine *infrav1.HetznerBareMetalMachine,
	client client.Client,
) *Service {

	return &Service{
		&scope.BareMetalMachineScope{
			Logger:           &log,
			Client:           client,
			BareMetalMachine: bmMachine,
		},
	}
}
