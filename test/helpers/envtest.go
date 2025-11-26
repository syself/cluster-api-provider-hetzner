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

// Package helpers includes helper functions important for unit and integration testing.
package helpers

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"time"

	g "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/log"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

func init() {
	klog.InitFlags(nil)
	logger := textlogger.NewLogger(textlogger.NewConfig(textlogger.Verbosity(4)))

	// use klog as the internal logger for this envtest environment.
	log.SetLogger(logger)
	// additionally force all of the controllers to use the Ginkgo logger.
	ctrl.SetLogger(logger)
	// add logger for ginkgo
	klog.SetOutput(g.GinkgoWriter)
}

var (
	scheme = runtime.NewScheme()
	env    *envtest.Environment
)

func init() {
	// Calculate the scheme.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(infrav1.AddToScheme(scheme))

	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0) //nolint:dogsled
	root := path.Join(path.Dir(filename), "..", "..")

	crdPaths := []string{
		filepath.Join(root, "config", "crd", "bases"),
	}

	// append CAPI CRDs path
	if capiPath := getFilePathToCAPICRDs(root); capiPath != "" {
		crdPaths = append(crdPaths, capiPath)
	}

	// Create the test environment.
	env = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     crdPaths,
	}
}

// Resetter provides an interface, so that test-suites using TestEnvironment can define their custom
// resetting.
type Resetter interface {
	// Reset resets the objects shared between tests. This avoids that changes done by the first
	// test will modify the environment of the second test.
	//
	// namespace: is the name of the new namespace which the test will use. Related: https://book.kubebuilder.io/reference/envtest.html#namespace-usage-limitation
	//
	// testEnv: the TestEnvironment which should be resetted.
	//
	// t: g.GinkgoT()
	Reset(namespace string, testEnv *TestEnvironment, t g.FullGinkgoTInterface)
}

// TestEnvironment encapsulates a Kubernetes local test environment.
type TestEnvironment struct {
	ctrl.Manager
	client.Client
	Config                       *rest.Config
	HCloudClientFactory          hcloudclient.Factory
	RobotClientFactory           robotclient.Factory
	BaremetalSSHClientFactory    sshclient.Factory
	HCloudSSHClientFactory       sshclient.Factory
	HCloudSSHClient              *sshmock.Client
	RescueSSHClient              *sshmock.Client
	OSSSHClientAfterInstallImage *sshmock.Client
	OSSSHClientAfterCloudInit    *sshmock.Client
	RobotClient                  *robotmock.Client
	cancel                       context.CancelFunc
	RateLimitWaitTime            time.Duration
	Resetter                     Resetter
}

// NewTestEnvironment creates a new environment spinning up a local api-server. Factories for
// clients (like HCloudClientFactory) are not created. This gets done in ResetAndCreateNamespace(),
// which should be use at the beginning of each test.
func NewTestEnvironment() *TestEnvironment {
	// initialize webhook here to be able to test the envtest install via webhookOptions
	initializeWebhookInEnvironment()

	if _, err := env.Start(); err != nil {
		err = kerrors.NewAggregate([]error{err, env.Stop()})
		panic(err)
	}

	// Build the controller manager.
	mgr, err := ctrl.NewManager(env.Config, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			ByObject: secretutil.AddSecretSelector(),
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port:    env.WebhookInstallOptions.LocalServingPort,
				CertDir: env.WebhookInstallOptions.LocalServingCertDir,
				Host:    "localhost",
			},
		),
		Metrics: metricsserver.Options{
			// Disable MetricsServer, so that two tests processes can run concurrently
			BindAddress: "0",
		},
		Logger: utils.GetDefaultLogger("info"),
	})
	if err != nil {
		klog.Fatalf("unable to create manager: %s", err)
	}

	record.InitFromRecorder(mgr.GetEventRecorderFor("hetzner-controller"))

	if err := (&infrav1.HetznerCluster{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HetznerCluster: %s", err)
	}
	if err := (&infrav1.HetznerClusterTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HetznerClusterTemplate: %s", err)
	}
	if err := (&infrav1.HCloudMachine{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HCloudMachine: %s", err)
	}
	if err := (&infrav1.HCloudMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HCloudMachineTemplate: %s", err)
	}
	if err := (&infrav1.HetznerBareMetalMachine{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HetznerBareMetalMachine: %s", err)
	}
	if err := (&infrav1.HetznerBareMetalMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HetznerBareMetalMachineTemplate: %s", err)
	}
	if err := (&infrav1.HetznerBareMetalHost{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HetznerBareMetalHost: %s", err)
	}
	if err := (&infrav1.HetznerBareMetalRemediation{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HetznerBareMetalRemediation: %s", err)
	}
	if err := (&infrav1.HetznerBareMetalRemediationTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HetznerBareMetalRemediationTemplate: %s", err)
	}
	if err := (&infrav1.HCloudRemediation{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HCloudRemediation: %s", err)
	}
	if err := (&infrav1.HCloudRemediationTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for HCloudRemediationTemplate: %s", err)
	}

	return &TestEnvironment{
		Manager:           mgr,
		Client:            mgr.GetClient(),
		Config:            mgr.GetConfig(),
		RateLimitWaitTime: 5 * time.Minute,
	}
}

// StartManager starts the manager and sets a cancel function into the testEnv object.
func (t *TestEnvironment) StartManager(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	return t.Manager.Start(ctx)
}

// Stop stops the manager and cancels the context.
func (t *TestEnvironment) Stop() error {
	t.cancel()
	return env.Stop()
}

// Cleanup deletes client objects.
func (t *TestEnvironment) Cleanup(ctx context.Context, objs ...client.Object) error {
	errs := make([]error, 0, len(objs))
	for _, o := range objs {
		err := t.Client.Delete(ctx, o)
		if apierrors.IsNotFound(err) {
			// If the object is not found, it must've been garbage collected
			// already. For example, if we delete namespace first and then
			// objects within it.
			continue
		}
		errs = append(errs, err)
	}
	return kerrors.NewAggregate(errs)
}

// ResetAndCreateNamespace creates a namespace.
func (t *TestEnvironment) ResetAndCreateNamespace(ctx context.Context, generateName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", generateName),
			Labels: map[string]string{
				"testenv/original-name": generateName,
			},
		},
	}
	if err := t.Client.Create(ctx, ns); err != nil {
		return nil, err
	}

	if t.Resetter != nil {
		t.Resetter.Reset(ns.Name, t, g.GinkgoT())
	}

	return ns, nil
}

// CreateKubeconfigSecret generates a kubeconfig secret in a given capi cluster.
func (t *TestEnvironment) CreateKubeconfigSecret(ctx context.Context, cluster *clusterv1.Cluster) error {
	return t.Create(ctx, kubeconfig.GenerateSecret(cluster, kubeconfig.FromEnvTestConfig(t.Config, cluster)))
}

func getFilePathToCAPICRDs(root string) string {
	mod, err := newMod(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	packageName := "sigs.k8s.io/cluster-api"
	clusterAPIVersion, err := mod.FindDependencyVersion(packageName)
	if err != nil {
		return ""
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@%s", clusterAPIVersion), "config", "crd", "bases")
}

func envOr(envKey, defaultValue string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	return defaultValue
}
