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
	"flag"
	"fmt"
	"os"
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
)

func main() {
	fs := pflag.CommandLine
	fs.StringVar(&metricsAddr, "metrics-bind-address", "localhost:8080", "The address the metric endpoint binds to.")
	fs.StringVar(&probeAddr, "health-probe-bind-address", ":9440", "The address the probe endpoint binds to.")
	fs.BoolVar(&enableLeaderElection, "leader-elect", true, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
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

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(utils.GetDefaultLogger(logLevel))

	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
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
	})
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
		TargetClusterManagersWaitGroup: &wg,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: hetznerClusterConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerCluster")
		os.Exit(1)
	}

	if err = (&controllers.HCloudMachineReconciler{
		Client:              mgr.GetClient(),
		APIReader:           mgr.GetAPIReader(),
		RateLimitWaitTime:   rateLimitWaitTime,
		HCloudClientFactory: hcloudClientFactory,
		WatchFilterValue:    watchFilterValue,
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
		Client:             mgr.GetClient(),
		RobotClientFactory: robotclient.NewFactory(),
		SSHClientFactory:   sshclient.NewFactory(),
		APIReader:          mgr.GetAPIReader(),
		RateLimitWaitTime:  rateLimitWaitTime,
		WatchFilterValue:   watchFilterValue,
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

	setupLog.Info("starting manager", "version", caphversion.Get().String())
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
