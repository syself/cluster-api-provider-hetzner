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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	fakehcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/mockedsshclient"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

const (
	defaultPodNamespace = "caph-system"
	timeout             = time.Second * 5
	interval            = time.Millisecond * 100
)

var (
	testEnv                   *helpers.TestEnvironment
	ctx                       = ctrl.SetupSignalHandler()
	wg                        sync.WaitGroup
	defaultPlacementGroupName = "caph-placement-group"
	defaultFailureDomain      = "fsn1"
)

func TestControllers(t *testing.T) {
	secretErrorRetryDelay = 1 * time.Millisecond
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

type ControllerResetter struct {
	debug bool
	// reconcileGate ensures no Reconcile is running while mock clients are being replaced between tests.
	// Each Reconcile call holds it as a read lock (via ReconcileGate); test setup holds it as a write
	// lock. Because a write lock waits for all read lock holders to finish, mock setup can only proceed
	// once all in-flight reconciles from the previous test have completed.
	reconcileGate                         sync.RWMutex
	baremetalSSHClientFactory             *mocks.SSHFactory
	HetznerClusterReconciler              *HetznerClusterReconciler
	HCloudMachineReconciler               *HCloudMachineReconciler
	HCloudMachineTemplateReconciler       *HCloudMachineTemplateReconciler
	HetznerBareMetalHostReconciler        *HetznerBareMetalHostReconciler
	HetznerBareMetalMachineReconciler     *HetznerBareMetalMachineReconciler
	HCloudRemediationReconciler           *HCloudRemediationReconciler
	HetznerBareMetalRemediationReconciler *HetznerBareMetalRemediationReconciler
}

func NewControllerResetter(
	sshFactory *mocks.SSHFactory,
	hetznerClusterReconciler *HetznerClusterReconciler,
	hcloudMachineReconciler *HCloudMachineReconciler,
	hcloudMachineTemplateReconciler *HCloudMachineTemplateReconciler,
	hetznerBareMetalHostReconciler *HetznerBareMetalHostReconciler,
	hetznerBareMetalMachineReconciler *HetznerBareMetalMachineReconciler,
	hcloudRemediationReconciler *HCloudRemediationReconciler,
	hetznerBareMetalRemediationReconciler *HetznerBareMetalRemediationReconciler,
) *ControllerResetter {
	r := &ControllerResetter{
		baremetalSSHClientFactory:             sshFactory,
		HetznerClusterReconciler:              hetznerClusterReconciler,
		HCloudMachineReconciler:               hcloudMachineReconciler,
		HCloudMachineTemplateReconciler:       hcloudMachineTemplateReconciler,
		HetznerBareMetalHostReconciler:        hetznerBareMetalHostReconciler,
		HetznerBareMetalMachineReconciler:     hetznerBareMetalMachineReconciler,
		HCloudRemediationReconciler:           hcloudRemediationReconciler,
		HetznerBareMetalRemediationReconciler: hetznerBareMetalRemediationReconciler,
		debug:                                 os.Getenv("DEBUG") != "",
	}

	// Give both reconcilers access to the shared gate. Each Reconcile call holds it as a read lock,
	// and test setup in ResetAndInitNamespace holds it as a write lock — which blocks until all
	// in-flight reconciles (read lock holders) finish before mock clients are swapped.
	hetznerBareMetalHostReconciler.ReconcileGate = &r.reconcileGate
	hcloudMachineReconciler.ReconcileGate = &r.reconcileGate

	return r
}

var _ helpers.Resetter = &ControllerResetter{}

// ResetAndInitNamespace implements Resetter.ResetAndInitNamespace(). Documentation is on the
// interface.
func (r *ControllerResetter) ResetAndInitNamespace(namespace string, testEnv *helpers.TestEnvironment, t FullGinkgoTInterface) func() {
	// Acquire the write lock. This blocks until all in-flight Reconcile calls (which hold
	// the read lock) finish, so no reconcile from the previous test is running when we
	// swap the mock clients. The returned func releases it once all On() expectations
	// have been registered by the caller's BeforeEach (via defer).
	r.reconcileGate.Lock()

	rescueSSHClient := &sshmock.Client{}
	// Register Testify helpers so failed expectations are reported against this test instance.
	rescueSSHClient.Test(t)

	osSSHClientAfterInstallImage := &sshmock.Client{}
	osSSHClientAfterInstallImage.Test(t)

	osSSHClientAfterCloudInit := &sshmock.Client{}
	osSSHClientAfterCloudInit.Test(t)

	robotClient := &robotmock.Client{}
	robotClient.Test(t)

	hcloudSSHClient := &sshmock.Client{}
	hcloudSSHClient.Test(t)

	hcloudClientFactory := fakehcloudclient.NewHCloudClientFactory()

	robotClientFactory := mocks.NewRobotFactory(robotClient)

	// Reset clients used by the test code
	testEnv.BaremetalSSHClientFactory = r.baremetalSSHClientFactory
	testEnv.HCloudSSHClientFactory = mockedsshclient.NewSSHFactory(hcloudSSHClient)
	testEnv.RescueSSHClient = rescueSSHClient
	testEnv.OSSSHClientAfterInstallImage = osSSHClientAfterInstallImage
	testEnv.OSSSHClientAfterCloudInit = osSSHClientAfterCloudInit
	testEnv.RobotClientFactory = robotClientFactory
	testEnv.RobotClient = robotClient
	testEnv.HCloudClientFactory = hcloudClientFactory

	// Reset clients used by Reconcile() and the namespace
	r.HetznerClusterReconciler.HCloudClientFactory = hcloudClientFactory
	r.HetznerClusterReconciler.Namespace = namespace

	r.HCloudMachineReconciler.HCloudClientFactory = hcloudClientFactory
	r.HCloudMachineReconciler.Namespace = namespace

	r.HCloudMachineTemplateReconciler.HCloudClientFactory = hcloudClientFactory
	r.HCloudMachineTemplateReconciler.Namespace = namespace

	r.HetznerBareMetalHostReconciler.RobotClientFactory = robotClientFactory
	r.HetznerBareMetalHostReconciler.Namespace = namespace
	r.HetznerBareMetalHostReconciler.WorkloadClusterClientFactory = newFakeWorkloadClusterClientFactory()

	r.HCloudRemediationReconciler.HCloudClientFactory = hcloudClientFactory
	r.HCloudRemediationReconciler.Namespace = namespace

	r.HetznerBareMetalMachineReconciler.HCloudClientFactory = hcloudClientFactory
	r.HetznerBareMetalMachineReconciler.Namespace = namespace

	r.HetznerBareMetalRemediationReconciler.Namespace = namespace

	if r.debug {
		testEnv.GetLogger().Info("Starting test: ===> ===> ===> ===> ===> ===> ===> " + t.Name())
	}

	return func() {
		r.baremetalSSHClientFactory.SetClients(rescueSSHClient, osSSHClientAfterInstallImage, osSSHClientAfterCloudInit)
		r.reconcileGate.Unlock()
	}
}

type fakeWorkloadClusterClientFactory struct {
	client client.Client
}

func (f *fakeWorkloadClusterClientFactory) NewWorkloadClient(_ context.Context) (client.Client, error) {
	return f.client, nil
}

func newFakeWorkloadClusterClientFactory() *fakeWorkloadClusterClientFactory {
	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))

	c := fakeclient.NewClientBuilder().WithScheme(s).Build()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: hostName},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{BootID: "test-boot-id"},
		},
	}
	if err := c.Create(context.Background(), node); err != nil {
		panic(err)
	}

	return &fakeWorkloadClusterClientFactory{client: c}
}

var _ scope.WorkloadClusterClientFactory = &fakeWorkloadClusterClientFactory{}

var _ = BeforeSuite(func() {
	utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
	utilruntime.Must(infrav2.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))

	testEnv = helpers.NewTestEnvironment()
	wg.Add(1)

	hetznerClusterReconciler := &HetznerClusterReconciler{
		Client:                         testEnv.GetClient(),
		APIReader:                      testEnv.GetAPIReader(),
		RateLimitWaitTime:              5 * time.Minute,
		TargetClusterManagersWaitGroup: &wg,
	}
	Expect(hetznerClusterReconciler.SetupWithManager(ctx, testEnv, controller.Options{})).To(Succeed())

	hcloudMachineReconciler := &HCloudMachineReconciler{
		Client:    testEnv.GetClient(),
		APIReader: testEnv.GetAPIReader(),
	}
	Expect(hcloudMachineReconciler.SetupWithManager(ctx, testEnv, controller.Options{})).To(Succeed())

	hcloudMachineTemplateReconciler := &HCloudMachineTemplateReconciler{
		Client:    testEnv.GetClient(),
		APIReader: testEnv.GetAPIReader(),
	}
	Expect(hcloudMachineTemplateReconciler.SetupWithManager(ctx, testEnv, controller.Options{})).To(Succeed())

	hetznerBareMetalHostReconciler := &HetznerBareMetalHostReconciler{
		Client:              testEnv.GetClient(),
		APIReader:           testEnv.GetAPIReader(),
		PreProvisionCommand: "dummy-pre-provision-command",
	}
	Expect(hetznerBareMetalHostReconciler.SetupWithManager(ctx, testEnv, controller.Options{})).To(Succeed())

	hetznerBareMetalMachineReconciler := &HetznerBareMetalMachineReconciler{
		Client:    testEnv.GetClient(),
		APIReader: testEnv.GetAPIReader(),
	}
	Expect(hetznerBareMetalMachineReconciler.SetupWithManager(ctx, testEnv, controller.Options{})).To(Succeed())

	hcloudRemediationReconciler := &HCloudRemediationReconciler{
		Client:            testEnv.GetClient(),
		APIReader:         testEnv.GetAPIReader(),
		RateLimitWaitTime: 5 * time.Minute,
	}
	Expect(hcloudRemediationReconciler.SetupWithManager(ctx, testEnv, controller.Options{})).To(Succeed())

	hetznerBareMetalRemediationReconciler := &HetznerBareMetalRemediationReconciler{
		Client: testEnv.GetClient(),
	}
	Expect(hetznerBareMetalRemediationReconciler.SetupWithManager(ctx, testEnv, controller.Options{})).To(Succeed())

	// One factory shared across resets so in-flight goroutines always hold a valid pointer.
	sshFactory := &mocks.SSHFactory{}
	hcloudMachineReconciler.SSHClientFactory = sshFactory
	hetznerBareMetalHostReconciler.SSHClientFactory = sshFactory

	testEnv.Resetter = NewControllerResetter(
		sshFactory, hetznerClusterReconciler, hcloudMachineReconciler,
		hcloudMachineTemplateReconciler, hetznerBareMetalHostReconciler,
		hetznerBareMetalMachineReconciler, hcloudRemediationReconciler,
		hetznerBareMetalRemediationReconciler)

	go func() {
		defer GinkgoRecover()
		Expect(testEnv.StartManager(ctx)).To(Succeed())
	}()

	<-testEnv.Elected()

	// wait for webhook port to be open prior to running tests
	testEnv.WaitForWebhooks()

	// create manager pod namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPodNamespace,
		},
	}

	Expect(testEnv.Create(ctx, ns)).To(Succeed())
})

func dumpMetrics() (reterr error) {
	metricFamilies, err := metrics.Registry.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %w", err)
	}

	if err := os.MkdirAll("../.reports", 0o750); err != nil {
		return fmt.Errorf("Error creating directory: %w", err)
	}
	f, err := os.Create("../.reports/controller_suite_test-metrics.txt")
	if err != nil {
		return fmt.Errorf("Error creating file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			reterr = errors.Join(reterr, fmt.Errorf("error closing metrics file: %w", err))
		}
	}()

	// Encode the metrics into text format
	encoder := expfmt.NewEncoder(f, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, mf := range metricFamilies {
		if err := encoder.Encode(mf); err != nil {
			return fmt.Errorf("error encoding metric family: %w", err)
		}
	}
	return nil
}

var _ = AfterSuite(func() {
	Expect(dumpMetrics()).To(Succeed())
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
		wg.Done() // Main manager has been stopped
		wg.Wait() // Wait for target cluster manager
	}
})

func getDefaultHetznerClusterV1Beta1Spec() infrav1.HetznerClusterSpec {
	return infrav1.HetznerClusterSpec{
		ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
			Enabled:   true,
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
		ControlPlaneEndpoint: &clusterv1beta1.APIEndpoint{},
		ControlPlaneRegions:  []infrav1.Region{"fsn1"},
		HCloudNetwork: infrav1.HCloudNetworkSpec{
			CIDRBlock:       "10.0.0.0/16",
			Enabled:         true,
			NetworkZone:     "eu-central",
			SubnetCIDRBlock: "10.0.0.0/24",
		},
		HCloudPlacementGroups: []infrav1.HCloudPlacementGroupSpec{
			{
				Name: defaultPlacementGroupName,
				Type: "spread",
			},
			{
				Name: "md-0",
				Type: "spread",
			},
		},
		HetznerSecret: infrav1.HetznerSecretRef{
			Key: infrav1.HetznerSecretKeyRef{
				HCloudToken:          "hcloud",
				HetznerRobotUser:     "robot-user",
				HetznerRobotPassword: "robot-password",
			},
			Name: "hetzner-secret",
		},
		SSHKeys: infrav1.HetznerSSHKeys{
			HCloud: []infrav1.SSHKey{
				{
					Name: "testsshkey",
				},
			},
			RobotRescueSecretRef: infrav1.SSHSecretRef{
				Name: "rescue-ssh-secret",
				Key: infrav1.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
		},
	}
}

func getDefaultHetznerSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hetzner-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"hcloud":         []byte("my-token"),
			"robot-user":     []byte("my-user-name"),
			"robot-password": []byte("my-password"),
		},
	}
}

func getDefaultBootstrapSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value": []byte("my-bootstrap"),
		},
	}
}

func getDefaultHetznerBareMetalMachineSpec() infrav1.HetznerBareMetalMachineSpec {
	return infrav1.HetznerBareMetalMachineSpec{
		InstallImage: infrav1.InstallImage{
			Image: infrav1.Image{
				Name: "image-name",
				URL:  "https://myfile.tar.gz",
			},
			PostInstallScript: "my script",
			Partitions: []infrav1.Partition{
				{
					Mount:      "lvm",
					FileSystem: "ext2",
					Size:       "1G",
				},
			},
		},
		SSHSpec: infrav1.SSHSpec{
			SecretRef: infrav1.SSHSecretRef{
				Name: "os-ssh-secret",
				Key: infrav1.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
			PortAfterInstallImage: 22,
		},
	}
}

// isPresentAndFalseWithReasonV1Beta1 reads a condition via the deprecated v1beta1conditions package
// (util/deprecated/v1beta1/conditions), i.e. the object's own status.conditions under the v1beta1
// contract. Only objects still served through the v1beta1 API satisfy this getter. This is distinct
// from isPresentAndFalseWithReasonDeprecatedV1Beta1, which reads status.deprecated.v1beta1.conditions
// on a v1beta2 object.
//
// TODO: remove this helper (and isPresentAndTrueV1Beta1) once every resource is migrated to native
// v1beta2 conditions, after which nothing satisfies the v1beta1conditions getter.
func isPresentAndFalseWithReasonV1Beta1(key types.NamespacedName, getter v1beta1conditions.Getter, condition clusterv1beta1.ConditionType, reason string) bool {
	err := testEnv.Get(ctx, key, getter)
	if err != nil {
		return false
	}

	if !v1beta1conditions.Has(getter, condition) {
		return false
	}
	objectCondition := v1beta1conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionFalse &&
		objectCondition.Reason == reason
}

// isPresentAndFalseWithReasonDeprecatedV1Beta1 reads a legacy-shape condition from a v1beta2
// CAPI core object (Cluster, Machine) via GetV1Beta1Conditions(), i.e. the
// status.deprecated.v1beta1.conditions field. This is how CAPI 1.11 exposes
// legacy conditions on v1beta2 objects under the v1beta1 contract compat layer.
func isPresentAndFalseWithReasonDeprecatedV1Beta1(key types.NamespacedName, obj client.Object, condition clusterv1.ConditionType, reason string) bool {
	if err := testEnv.Get(ctx, key, obj); err != nil {
		return false
	}
	getter, ok := obj.(deprecatedv1beta1conditions.Getter)
	if !ok {
		return false
	}
	c := deprecatedv1beta1conditions.Get(getter, condition)
	return c != nil && c.Status == corev1.ConditionFalse && c.Reason == reason
}

// isPresentAndTrueDeprecatedV1Beta1 reads a legacy-shape condition from a v1beta2
// CAPI core object (Cluster, Machine) via GetV1Beta1Conditions(), i.e. the
// status.deprecated.v1beta1.conditions field. This is how CAPI 1.11 exposes
// legacy conditions on v1beta2 objects under the v1beta1 contract compat layer.
func isPresentAndTrueDeprecatedV1Beta1(key types.NamespacedName, obj client.Object, condition clusterv1.ConditionType) bool {
	if err := testEnv.Get(ctx, key, obj); err != nil {
		return false
	}
	getter, ok := obj.(deprecatedv1beta1conditions.Getter)
	if !ok {
		return false
	}
	c := deprecatedv1beta1conditions.Get(getter, condition)
	return c != nil && c.Status == corev1.ConditionTrue
}

// isPresentAndTrueV1Beta1 reads a condition via the deprecated v1beta1conditions package
// (util/deprecated/v1beta1/conditions), i.e. the object's own status.conditions under the v1beta1
// contract. Only objects still served through the v1beta1 API satisfy this getter. This is distinct
// from isPresentAndTrueDeprecatedV1Beta1, which reads status.deprecated.v1beta1.conditions on a
// v1beta2 object.
//
// TODO: remove this helper (and isPresentAndFalseWithReasonV1Beta1) once every resource is migrated
// to native v1beta2 conditions, after which nothing satisfies the v1beta1conditions getter.
func isPresentAndTrueV1Beta1(key types.NamespacedName, getter v1beta1conditions.Getter, condition clusterv1beta1.ConditionType) bool {
	err := testEnv.Get(ctx, key, getter)
	if err != nil {
		return false
	}

	if !v1beta1conditions.Has(getter, condition) {
		return false
	}
	objectCondition := v1beta1conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionTrue
}

// isConditionWithStatusAndReason reads a condition from either a native v1beta2 object (via
// conditions.Getter, which reads status.conditions) or a still-v1beta1 object that stages its
// conditions (via v1beta2conditions.Getter, which reads status.v1beta2.conditions). A native object
// has GetConditions(), not the staged GetV1Beta2Conditions(), so it does not satisfy the staged
// getter; that is why we try the native getter first and fall back to the staged one. The staged
// branch goes away once every resource is a native v1beta2 type.
func isConditionWithStatusAndReason(key types.NamespacedName, getter client.Object, condition string, status metav1.ConditionStatus, reason string) bool {
	if err := testEnv.Get(ctx, key, getter); err != nil {
		return false
	}

	if nativeGetter, ok := getter.(conditions.Getter); ok {
		objectCondition := conditions.Get(nativeGetter, condition)
		return objectCondition != nil && objectCondition.Status == status && objectCondition.Reason == reason
	}

	v1beta2Getter, ok := getter.(v1beta2conditions.Getter)
	if !ok || !v1beta2conditions.Has(v1beta2Getter, condition) {
		return false
	}

	objectCondition := v1beta2conditions.Get(v1beta2Getter, condition)
	return objectCondition.Status == status && objectCondition.Reason == reason
}

func isPresentAndTrueWithReason(key types.NamespacedName, getter client.Object, condition string, reason string) bool {
	return isConditionWithStatusAndReason(key, getter, condition, metav1.ConditionTrue, reason)
}

func isPresentAndFalseWithReason(key types.NamespacedName, getter client.Object, condition string, reason string) bool {
	return isConditionWithStatusAndReason(key, getter, condition, metav1.ConditionFalse, reason)
}

func isAbsent(key types.NamespacedName, getter client.Object, condition string) bool {
	if err := testEnv.Get(ctx, key, getter); err != nil {
		return false
	}

	if nativeGetter, ok := getter.(conditions.Getter); ok {
		return conditions.Get(nativeGetter, condition) == nil
	}

	v1beta2Getter, ok := getter.(v1beta2conditions.Getter)
	if !ok {
		return false
	}

	return !v1beta2conditions.Has(v1beta2Getter, condition)
}

func hasEvent(ctx context.Context, c client.Client, namespace, involvedObjectName, reason, message string) bool {
	eventList := &corev1.EventList{}
	if err := c.List(ctx, eventList, client.InNamespace(namespace)); err != nil {
		return false
	}

	for i := range eventList.Items {
		event := eventList.Items[i]
		if event.Reason == reason &&
			event.InvolvedObject.Name == involvedObjectName &&
			strings.Contains(event.Message, message) {
			return true
		}
	}

	return false
}
