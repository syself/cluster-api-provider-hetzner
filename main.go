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

// Package main contains main function to start CAPH.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// +kubebuilder:scaffold:imports
	infrastructurev1beta1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/controllers"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	caphversion "github.com/syself/cluster-api-provider-hetzner/pkg/version"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// We do not want filenames to start with a dot or a number.
	// Only lowercase letters are allowed.
	commandRegex = regexp.MustCompile(`^[a-z][a-z0-9_.-]+[a-z0-9]$`)
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

var (
	metricsAddr                        string
	enableLeaderElection               bool
	disableCSRApproval                 bool
	leaderElectionNamespace            string
	probeAddr                          string
	watchFilterValue                   string
	watchNamespace                     string
	hetznerClusterConcurrency          int
	hcloudMachineConcurrency           int
	hetznerBareMetalMachineConcurrency int
	hetznerBareMetalHostConcurrency    int
	logLevel                           string
	syncPeriod                         time.Duration
	rateLimitWaitTime                  time.Duration
	preProvisionCommand                string
	hcloudImageURLCommand              string
	skipWebhooks                       bool
)

func main() {
	fs := pflag.CommandLine
	fs.StringVar(&metricsAddr, "metrics-bind-address", "localhost:8080", "The address the metric endpoint binds to.")
	fs.StringVar(&probeAddr, "health-probe-bind-address", ":9440", "The address the probe endpoint binds to.")
	fs.BoolVar(&enableLeaderElection, "leader-elect", true, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.BoolVar(&disableCSRApproval, "disable-csr-approval", false, "Disables builtin workload cluster CSR validation and approval.")
	fs.StringVar(&leaderElectionNamespace, "leader-elect-namespace", "", "Namespace that the controller performs leader election in. If unspecified, the controller will discover which namespace it is running in.")
	fs.StringVar(&watchFilterValue, "watch-filter", "", fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel))
	fs.StringVar(&watchNamespace, "namespace", "", "Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	fs.IntVar(&hetznerClusterConcurrency, "hetznercluster-concurrency", 1, "Number of HetznerClusters to process simultaneously")
	fs.IntVar(&hcloudMachineConcurrency, "hcloudmachine-concurrency", 1, "Number of HcloudMachines to process simultaneously")
	fs.IntVar(&hetznerBareMetalMachineConcurrency, "hetznerbaremetalmachine-concurrency", 1, "Number of HetznerBareMetalMachines to process simultaneously")
	fs.IntVar(&hetznerBareMetalHostConcurrency, "hetznerbaremetalhost-concurrency", 1, "Number of HetznerBareMetalHosts to process simultaneously")
	fs.StringVar(&logLevel, "log-level", "info", "Specifies log level. Options are 'debug', 'info' and 'error'")
	fs.DurationVar(&syncPeriod, "sync-period", 3*time.Minute, "The minimum interval at which watched resources are reconciled (e.g. 3m)")
	fs.DurationVar(&rateLimitWaitTime, "rate-limit", 5*time.Minute, "The rate limiting for HCloud controller (e.g. 5m)")
	fs.BoolVar(&hcloudclient.DebugAPICalls, "debug-hcloud-api-calls", false, "Debug all calls to the hcloud API.")
	fs.StringVar(&preProvisionCommand, "pre-provision-command", "", "Command to run (in rescue-system) before installing the image on bare metal servers. You can use that to check if the machine is healthy before installing the image. If the exit value is non-zero, the machine is considered unhealthy. This command must be accessible by the controller pod. You can use an initContainer to copy the command to a shared emptyDir.")
	fs.StringVar(&hcloudImageURLCommand, "hcloud-image-url-command", "", "Command to run (in rescue-system) to provision an hcloud machine. The command will get the imageURL of the coresponding hcloudmachine as argument. It is up to the command to download from that URL and provision the disk accordingly. This command must be accessible by the controller pod. You can use an initContainer to copy the command to a shared emptyDir. The env var OCI_REGISTRY_AUTH_TOKEN from the caph process will be set for the command, too. The command must end with the last line containing IMAGE_INSTALL_DONE. Otherwise the execution is considered to have failed. Related https://pkg.go.dev/github.com/syself/cluster-api-provider-hetzner/api/v1beta1#HCloudMachineSpec.ImageURL")
	fs.BoolVar(&skipWebhooks, "skip-webhooks", false, "Skip setting up of webhooks. Together with --leader-elect=false, you can use `go run main.go` to run CAPH in a cluster connected via KUBECONFIG. You should scale down the caph deployment to 0 before doing that. This is only for testing!")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(utils.GetDefaultLogger(logLevel))

	// If preProvisionCommand is set, check if the file exists and validate the basename.
	if preProvisionCommand != "" {
		baseName := filepath.Base(preProvisionCommand)
		if !commandRegex.MatchString(baseName) {
			msg := fmt.Sprintf("basename (%s) must match the regex %s", baseName, commandRegex.String())
			setupLog.Error(errors.New(msg), "")
			os.Exit(1)
		}

		_, err := os.Stat(preProvisionCommand)
		if err != nil {
			setupLog.Error(err, "pre-provision-command not found")
			os.Exit(1)
		}
	}

	// If hcloudImageURLCommand is set, check if the file exists and validate the basename.
	if hcloudImageURLCommand != "" {
		baseName := filepath.Base(hcloudImageURLCommand)
		if !commandRegex.MatchString(baseName) {
			msg := fmt.Sprintf("basename (%s) must match the regex %s", baseName, commandRegex.String())
			setupLog.Error(errors.New(msg), "")
			os.Exit(1)
		}

		_, err := os.Stat(hcloudImageURLCommand)
		if err != nil {
			setupLog.Error(err, "hcloud-image-url-command not found")
			os.Exit(1)
		}
	}

	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	options := ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "hetzner.cluster.x-k8s.io",
		LeaderElectionNamespace:       leaderElectionNamespace,
		LeaderElectionResourceLock:    "leases",
		LeaderElectionReleaseOnCancel: true,
		Cache: cache.Options{
			ByObject:          secretutil.AddSecretSelector(),
			SyncPeriod:        &syncPeriod,
			DefaultNamespaces: watchNamespaces,
		},
	}

	if !skipWebhooks {
		options.WebhookServer = webhook.NewServer(webhook.Options{
			Port: 9443,
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("hetzner-controller"))

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	hcloudClientFactory := hcloudclient.NewFactory()

	var wg sync.WaitGroup
	wg.Add(1)

	if err = (&controllers.HetznerClusterReconciler{
		Client:                         mgr.GetClient(),
		APIReader:                      mgr.GetAPIReader(),
		RateLimitWaitTime:              rateLimitWaitTime,
		HCloudClientFactory:            hcloudClientFactory,
		WatchFilterValue:               watchFilterValue,
		DisableCSRApproval:             disableCSRApproval,
		TargetClusterManagersWaitGroup: &wg,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: hetznerClusterConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerCluster")
		os.Exit(1)
	}

	if err = (&controllers.HCloudMachineReconciler{
		Client:                mgr.GetClient(),
		APIReader:             mgr.GetAPIReader(),
		RateLimitWaitTime:     rateLimitWaitTime,
		HCloudClientFactory:   hcloudClientFactory,
		SSHClientFactory:      sshclient.NewFactory(),
		WatchFilterValue:      watchFilterValue,
		HCloudImageURLCommand: hcloudImageURLCommand,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: hcloudMachineConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudMachine")
		os.Exit(1)
	}

	if err = (&controllers.HCloudMachineTemplateReconciler{
		Client:              mgr.GetClient(),
		APIReader:           mgr.GetAPIReader(),
		RateLimitWaitTime:   rateLimitWaitTime,
		HCloudClientFactory: hcloudClientFactory,
		WatchFilterValue:    watchFilterValue,
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudMachineTemplate")
		os.Exit(1)
	}

	if err = (&controllers.HetznerBareMetalHostReconciler{
		Client:              mgr.GetClient(),
		RobotClientFactory:  robotclient.NewFactory(),
		SSHClientFactory:    sshclient.NewFactory(),
		APIReader:           mgr.GetAPIReader(),
		RateLimitWaitTime:   rateLimitWaitTime,
		WatchFilterValue:    watchFilterValue,
		PreProvisionCommand: preProvisionCommand,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: hetznerBareMetalHostConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalHost")
		os.Exit(1)
	}

	if err = (&controllers.HetznerBareMetalMachineReconciler{
		Client:              mgr.GetClient(),
		APIReader:           mgr.GetAPIReader(),
		RateLimitWaitTime:   rateLimitWaitTime,
		HCloudClientFactory: hcloudClientFactory,
		WatchFilterValue:    watchFilterValue,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: hetznerBareMetalMachineConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalMachine")
		os.Exit(1)
	}

	if err = (&controllers.HetznerBareMetalRemediationReconciler{
		Client:           mgr.GetClient(),
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalRemediation")
		os.Exit(1)
	}

	if err = (&controllers.HCloudRemediationReconciler{
		Client:              mgr.GetClient(),
		APIReader:           mgr.GetAPIReader(),
		RateLimitWaitTime:   rateLimitWaitTime,
		HCloudClientFactory: hcloudClientFactory,
		WatchFilterValue:    watchFilterValue,
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudRemediation")
		os.Exit(1)
	}

	if !skipWebhooks {
		setUpWebhookWithManager(mgr)

		//+kubebuilder:scaffold:builder

		if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
			setupLog.Error(err, "unable to create ready check")
			os.Exit(1)
		}

		if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
			setupLog.Error(err, "unable to create health check")
			os.Exit(1)
		}
	}

	setupLog.Info("starting manager", "version", caphversion.Get().String(), "args", os.Args)
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	wg.Done()
	// Wait for all target cluster managers to gracefully shut down.
	wg.Wait()
}

func setUpWebhookWithManager(mgr ctrl.Manager) {
	if err := (&infrastructurev1beta1.HetznerCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerCluster")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HetznerClusterTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerClusterTemplate")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HCloudMachine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HCloudMachine")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HCloudMachineTemplateWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HCloudMachineTemplate")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HetznerBareMetalHostWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalHost")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HetznerBareMetalMachine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalMachine")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HetznerBareMetalMachineTemplateWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalMachineTemplate")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HetznerBareMetalRemediation{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalRemediation")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HetznerBareMetalRemediationTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalRemediationTemplate")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HCloudRemediation{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HCloudRemediation")
		os.Exit(1)
	}
	if err := (&infrastructurev1beta1.HCloudRemediationTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HCloudRemediationTemplate")
		os.Exit(1)
	}
}
