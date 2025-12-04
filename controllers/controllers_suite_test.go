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
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
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
	HetznerClusterReconciler              *HetznerClusterReconciler
	HCloudMachineReconciler               *HCloudMachineReconciler
	HCloudMachineTemplateReconciler       *HCloudMachineTemplateReconciler
	HetznerBareMetalHostReconciler        *HetznerBareMetalHostReconciler
	HetznerBareMetalMachineReconciler     *HetznerBareMetalMachineReconciler
	HCloudRemediationReconciler           *HCloudRemediationReconciler
	HetznerBareMetalRemediationReconciler *HetznerBareMetalRemediationReconciler
}

func NewControllerResetter(
	hetznerClusterReconciler *HetznerClusterReconciler,
	hcloudMachineReconciler *HCloudMachineReconciler,
	hcloudMachineTemplateReconciler *HCloudMachineTemplateReconciler,
	hetznerBareMetalHostReconciler *HetznerBareMetalHostReconciler,
	hetznerBareMetalMachineReconciler *HetznerBareMetalMachineReconciler,
	hcloudRemediationReconciler *HCloudRemediationReconciler,
	hetznerBareMetalRemediationReconciler *HetznerBareMetalRemediationReconciler,
) *ControllerResetter {
	return &ControllerResetter{
		HetznerClusterReconciler:              hetznerClusterReconciler,
		HCloudMachineReconciler:               hcloudMachineReconciler,
		HCloudMachineTemplateReconciler:       hcloudMachineTemplateReconciler,
		HetznerBareMetalHostReconciler:        hetznerBareMetalHostReconciler,
		HetznerBareMetalMachineReconciler:     hetznerBareMetalMachineReconciler,
		HCloudRemediationReconciler:           hcloudRemediationReconciler,
		HetznerBareMetalRemediationReconciler: hetznerBareMetalRemediationReconciler,
	}
}

var _ helpers.Resetter = &ControllerResetter{}

// ResetAndInitNamespace implements Resetter.ResetAndInitNamespace(). Documentation is on the
// interface.
func (r *ControllerResetter) ResetAndInitNamespace(namespace string, testEnv *helpers.TestEnvironment, t FullGinkgoTInterface) {
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
	baremetalSSHClientFactory := mocks.NewSSHFactory(rescueSSHClient,
		osSSHClientAfterInstallImage, osSSHClientAfterCloudInit)

	// Reset clients used by the test code
	testEnv.BaremetalSSHClientFactory = mocks.NewSSHFactory(rescueSSHClient,
		osSSHClientAfterInstallImage, osSSHClientAfterCloudInit)
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
	r.HCloudMachineReconciler.SSHClientFactory = baremetalSSHClientFactory
	r.HCloudMachineReconciler.Namespace = namespace

	r.HCloudMachineTemplateReconciler.HCloudClientFactory = hcloudClientFactory
	r.HCloudMachineTemplateReconciler.Namespace = namespace

	r.HetznerBareMetalHostReconciler.RobotClientFactory = robotClientFactory
	r.HetznerBareMetalHostReconciler.SSHClientFactory = baremetalSSHClientFactory
	r.HetznerBareMetalHostReconciler.Namespace = namespace

	r.HCloudRemediationReconciler.HCloudClientFactory = hcloudClientFactory
	r.HCloudRemediationReconciler.Namespace = namespace

	r.HetznerBareMetalMachineReconciler.HCloudClientFactory = hcloudClientFactory
	r.HetznerBareMetalMachineReconciler.Namespace = namespace

	r.HetznerBareMetalRemediationReconciler.Namespace = namespace
}

var _ = BeforeSuite(func() {
	utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))

	testEnv = helpers.NewTestEnvironment()
	wg.Add(1)

	hetznerClusterReconciler := &HetznerClusterReconciler{
		Client:                         testEnv.Manager.GetClient(),
		APIReader:                      testEnv.Manager.GetAPIReader(),
		RateLimitWaitTime:              5 * time.Minute,
		TargetClusterManagersWaitGroup: &wg,
	}
	Expect(hetznerClusterReconciler.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	hcloudMachineReconciler := &HCloudMachineReconciler{
		Client:    testEnv.Manager.GetClient(),
		APIReader: testEnv.Manager.GetAPIReader(),
	}
	Expect(hcloudMachineReconciler.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	hcloudMachineTemplateReconciler := &HCloudMachineTemplateReconciler{
		Client:    testEnv.Manager.GetClient(),
		APIReader: testEnv.Manager.GetAPIReader(),
	}
	Expect(hcloudMachineTemplateReconciler.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	hetznerBareMetalHostReconciler := &HetznerBareMetalHostReconciler{
		Client:               testEnv.Manager.GetClient(),
		APIReader:            testEnv.Manager.GetAPIReader(),
		PreProvisionCommand:  "dummy-pre-provision-command",
		SSHAfterInstallImage: true,
	}
	Expect(hetznerBareMetalHostReconciler.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	hetznerBareMetalMachineReconciler := &HetznerBareMetalMachineReconciler{
		Client:    testEnv.Manager.GetClient(),
		APIReader: testEnv.Manager.GetAPIReader(),
	}
	Expect(hetznerBareMetalMachineReconciler.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	hcloudRemediationReconciler := &HCloudRemediationReconciler{
		Client:            testEnv.Manager.GetClient(),
		APIReader:         testEnv.Manager.GetAPIReader(),
		RateLimitWaitTime: 5 * time.Minute,
	}
	Expect(hcloudRemediationReconciler.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	hetznerBareMetalRemediationReconciler := &HetznerBareMetalRemediationReconciler{
		Client: testEnv.Manager.GetClient(),
	}
	Expect(hetznerBareMetalRemediationReconciler.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	testEnv.Resetter = NewControllerResetter(hetznerClusterReconciler, hcloudMachineReconciler,
		hcloudMachineTemplateReconciler, hetznerBareMetalHostReconciler,
		hetznerBareMetalMachineReconciler, hcloudRemediationReconciler,
		hetznerBareMetalRemediationReconciler)

	go func() {
		defer GinkgoRecover()
		Expect(testEnv.StartManager(ctx)).To(Succeed())
	}()

	<-testEnv.Manager.Elected()

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

func dumpMetrics() error {
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
	defer f.Close()

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
	Expect(testEnv.Stop()).To(Succeed())
	wg.Done() // Main manager has been stopped
	wg.Wait() // Wait for target cluster manager
})

func getDefaultHetznerClusterSpec() infrav1.HetznerClusterSpec {
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
		ControlPlaneEndpoint: &clusterv1.APIEndpoint{},
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

func isPresentAndFalseWithReason(key types.NamespacedName, getter conditions.Getter, condition clusterv1.ConditionType, reason string) bool {
	err := testEnv.Get(ctx, key, getter)
	if err != nil {
		return false
	}

	if !conditions.Has(getter, condition) {
		return false
	}
	objectCondition := conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionFalse &&
		objectCondition.Reason == reason
}

func isPresentAndTrue(key types.NamespacedName, getter conditions.Getter, condition clusterv1.ConditionType) bool {
	err := testEnv.Get(ctx, key, getter)
	if err != nil {
		return false
	}

	if !conditions.Has(getter, condition) {
		return false
	}
	objectCondition := conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionTrue
}

func hasEvent(ctx context.Context, c client.Client, namespace, involvedObjectName, reason, message string) bool {
	eventList := &corev1.EventList{}
	if err := c.List(ctx, eventList, client.InNamespace(namespace)); err != nil {
		return false
	}

	for i := range eventList.Items {
		event := eventList.Items[i]
		if event.Reason == reason &&
			event.Message == message &&
			event.InvolvedObject.Name == involvedObjectName {
			return true
		}
	}

	return false
}
