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

package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	// +kubebuilder:scaffold:imports
	infrastructurev1beta1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/controllers"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
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
	metricsAddr          string
	enableLeaderElection bool
	verbose              bool
	probeAddr            string
	watchFilterValue     string
	watchNamespace       string
	logLevel             string
)

func main() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&metricsAddr, "metrics-bind-address", "localhost:8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":9440", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&watchFilterValue, "watch-filter", "", fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel))
	flag.StringVar(&watchNamespace, "namespace", "", "Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	flag.StringVar(&logLevel, "log-level", "debug", "Specifies log level. Options are 'debug', 'info' and 'error'")

	flag.Parse()

	ctrl.SetLogger(utils.GetDefaultLogger(logLevel))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         metricsAddr,
		Port:                       9443,
		HealthProbeBindAddress:     probeAddr,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "hetzner.cluster.x-k8s.io",
		LeaderElectionResourceLock: "leases",
		Namespace:                  watchNamespace,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: secretutil.AddSecretSelector(nil),
		}),
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
		HCloudClientFactory:            hcloudClientFactory,
		WatchFilterValue:               watchFilterValue,
		TargetClusterManagersWaitGroup: &wg,
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerCluster")
		os.Exit(1)
	}
	if err = (&controllers.HCloudMachineReconciler{
		Client:              mgr.GetClient(),
		APIReader:           mgr.GetAPIReader(),
		HCloudClientFactory: hcloudClientFactory,
		WatchFilterValue:    watchFilterValue,
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HCloudMachine")
		os.Exit(1)
	}

	if err = (&infrastructurev1beta1.HetznerCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerCluster")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HetznerClusterTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerClusterTemplate")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HCloudMachine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HCloudMachine")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HCloudMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HCloudMachineTemplate")
		os.Exit(1)
	}
	if err = (&controllers.HetznerBareMetalMachineReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalMachine")
		os.Exit(1)
	}
	if err = (&controllers.HetznerBareMetalMachineTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalMachineTemplate")
		os.Exit(1)
	}
	if err = (&controllers.HetznerBareMetalRemediationTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalRemediationTemplate")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HetznerBareMetalMachine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalMachine")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HetznerBareMetalMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalMachineTemplate")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HetznerBareMetalRemediationTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalRemediationTemplate")
		os.Exit(1)
	}
	if err = (&controllers.HetznerBareMetalHostReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalHost")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HetznerBareMetalHost{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalHost")
		os.Exit(1)
	}
	if err = (&controllers.HetznerBareMetalRemediationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HetznerBareMetalRemediation")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.HetznerBareMetalRemediation{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HetznerBareMetalRemediation")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	wg.Done()
	// Wait for all target cluster managers to gracefully shut down.
	wg.Wait()
}
