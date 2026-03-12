package host

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func testClientWithHost(t *testing.T, host *infrav1.HetznerBareMetalHost) (ctrlclient.Client, *infrav1.HetznerBareMetalHost) {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(infrav1.AddToScheme(scheme))

	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(host).Build()

	liveHost := &infrav1.HetznerBareMetalHost{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: host.Name, Namespace: host.Namespace}, liveHost)
	require.NoError(t, err)

	return cl, liveHost
}

func TestSaveHostAndReturnPreservesLastUpdatedForTimeoutTrackedErrors(t *testing.T) {
	old := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	host := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-timeout",
			Namespace: "default",
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				LastUpdated: &old,
				ErrorType:   infrav1.ErrorTypeSSHRebootTriggered,
			},
		},
	}

	cl, liveHost := testClientWithHost(t, host)
	_, err := SaveHostAndReturn(context.Background(), cl, liveHost)
	require.NoError(t, err)
	require.Equal(t, old.Time.Unix(), liveHost.Spec.Status.LastUpdated.Time.Unix())
}

func TestSaveHostAndReturnRefreshesLastUpdatedForOtherStates(t *testing.T) {
	old := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	host := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-non-timeout",
			Namespace: "default",
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			Status: infrav1.ControllerGeneratedStatus{
				LastUpdated: &old,
				ErrorType:   infrav1.ProvisioningError,
			},
		},
	}

	cl, liveHost := testClientWithHost(t, host)
	_, err := SaveHostAndReturn(context.Background(), cl, liveHost)
	require.NoError(t, err)
	require.True(t, liveHost.Spec.Status.LastUpdated.Time.After(old.Time))
}
