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

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guettli/check-conditions/pkg/checkconditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// Test suite flags.
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// clusterctlConfig is the file which tests will use as a clusterctl config.
	// If it is not set, a local clusterctl repository (including a clusterctl config) will be created automatically.
	clusterctlConfig string

	// kubetestConfigFilePath is the path to the kubetest configuration file.
	kubetestConfigFilePath string

	// alsoLogToFile enables additional logging to the 'ginkgo-log.txt' file in the artifact folder.
	// These logs also contain timestamps.
	alsoLogToFile bool

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool
)

// Test suite global vars.
var (
	ctx = ctrl.SetupSignalHandler()

	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.BoolVar(&alsoLogToFile, "e2e.also-log-to-file", true, "if true, ginkgo logs are additionally written to the `ginkgo-log.txt` file in the artifacts folder (including timestamps)")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.StringVar(&clusterctlConfig, "e2e.clusterctl-config", "", "file which tests will use as a clusterctl config. If it is not set, a local clusterctl repository (including a clusterctl config) will be created automatically.")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.StringVar(&kubetestConfigFilePath, "kubetest.config-file", "", "path to the kubetest configuration file")

	ctrl.SetLogger(klog.Background())
}

func TestE2E(t *testing.T) {
	// If running in prow, make sure to use the artifacts folder that will be reported in test grid (ignoring the value provided by flag).
	if prowArtifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		artifactFolder = prowArtifactFolder
	}

	RegisterFailHandler(Fail)

	if alsoLogToFile {
		w, err := ginkgoextensions.EnableFileLogging(filepath.Join(artifactFolder, "ginkgo-log.txt"))
		Expect(err).ToNot(HaveOccurred())
		defer w.Close()
	}

	RunSpecs(t, "caph-e2e")
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo threads).
// The local clusterctl repository & the bootstrap cluster are created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	// Before all ParallelNodes.

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0o755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder) //nolint:gosec

	log("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	Byf("Loading the e2e test configuration from %q", configPath)

	log(fmt.Sprintf("Loading the e2e test configuration from %q", configPath))
	e2eConfig = loadE2EConfig(ctx, configPath)

	log(fmt.Sprintf("Creating a clusterctl local repository into %q", artifactFolder))
	clusterctlConfigPath = createClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(artifactFolder, "repository"))

	log("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy = setupBootstrapCluster(e2eConfig, scheme, useExistingCluster)

	log("Initializing the bootstrap cluster")
	initBootstrapCluster(ctx, bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)
	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.

	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(4))

	artifactFolder = parts[0]
	configPath = parts[1]
	clusterctlConfigPath = parts[2]
	kubeconfigPath := parts[3]

	e2eConfig = loadE2EConfig(ctx, configPath)
	bootstrapClusterProxy = framework.NewClusterProxy("bootstrap", kubeconfigPath, initScheme(), framework.WithMachineLogCollector(logCollector{}))
})

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
// The local clusterctl repository is preserved like everything else created into the artifact folder.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
	log("Tearing down the management cluster")
	if !skipCleanup {
		tearDown(ctx, nil, bootstrapClusterProxy)
	}
}, func() {
	// After all ParallelNodes.
	if !skipCleanup {
		tearDown(ctx, bootstrapClusterProvider, nil)
	}
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	_ = infrav1.AddToScheme(sc)
	return sc
}

func loadE2EConfig(ctx context.Context, configPath string) *clusterctl.E2EConfig {
	config := clusterctl.LoadE2EConfig(ctx, clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
	Expect(config).ToNot(BeNil(), "Failed to load E2E config from %s", configPath)

	return config
}

func createClusterctlLocalRepository(ctx context.Context, config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	// Ensuring a CCM file is defined in the config and register a FileTransformation to inject the referenced file as in place of the CCM_RESOURCES envSubst variable.
	Expect(config.Variables).To(HaveKey(CiliumPath), "Missing %s variable in the config", CiliumPath)
	ciliumPath := config.GetVariableOrEmpty(CiliumPath)
	Expect(ciliumPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", CiliumPath)
	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(ciliumPath, CiliumResources)

	// Ensuring a CCM file is defined in the config and register a FileTransformation to inject the referenced file as in place of the CCM_RESOURCES envSubst variable.
	Expect(config.Variables).To(HaveKey(CCMPath), "Missing %s variable in the config", CCMPath)
	ccmPath := config.GetVariableOrEmpty(CCMPath)
	Expect(ccmPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", CCMPath)
	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(ccmPath, CCMResources)

	// Ensuring a CCM file is defined for clusters with networks in the config and register a FileTransformation to inject the referenced file as in place of the CCM_RESOURCES envSubst variable.
	Expect(config.Variables).To(HaveKey(CCMNetworkPath), "Missing %s variable in the config", CCMNetworkPath)
	ccmNetworkPath := config.GetVariableOrEmpty(CCMNetworkPath)
	Expect(ccmNetworkPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", CCMNetworkPath)
	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(ccmNetworkPath, CCMNetworkResources)

	// Ensuring a CCM file is defined for clusters with networks in the config and register a FileTransformation to inject the referenced file as in place of the CCM_RESOURCES envSubst variable.
	Expect(config.Variables).To(HaveKey(CCMHetznerPath), "Missing %s variable in the config", CCMHetznerPath)
	ccmHetznerPath := config.GetVariableOrEmpty(CCMHetznerPath)
	Expect(ccmHetznerPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", CCMHetznerPath)
	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(ccmHetznerPath, CCMHetznerResources)

	clusterctlConfig := clusterctl.CreateRepository(ctx, createRepositoryInput)
	Expect(clusterctlConfig).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)

	return clusterctlConfig
}

func setupBootstrapCluster(config *clusterctl.E2EConfig, scheme *runtime.Scheme, useExistingCluster bool) (bootstrap.ClusterProvider, framework.ClusterProxy) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               config.ManagementClusterName,
			RequiresDockerSock: config.HasDockerProvider(),
			Images:             config.Images,
		})
		Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")
	}

	clusterProxy := framework.NewClusterProxy("bootstrap", kubeconfigPath, scheme, framework.WithMachineLogCollector(logCollector{}))
	Expect(clusterProxy).ToNot(BeNil(), "Failed to get a bootstrap cluster proxy")

	return clusterProvider, clusterProxy
}

func logStatusContinuously(ctx context.Context, restConfig *restclient.Config, c client.Client) {
	for {
		select {
		case <-ctx.Done():
			log("Context canceled, stopping logStatusContinuously")
			return
		case <-time.After(30 * time.Second):
			err := logStatus(ctx, restConfig, c)
			if err != nil {
				log(fmt.Sprintf("Error logging caph Deployment: %v", err))
			}
		}
	}
}

func logStatus(ctx context.Context, restConfig *restclient.Config, c client.Client) error {
	log(fmt.Sprintf("%s <<< Start logging status", time.Now().Format("2006-01-02 15:04:05")))

	if err := logCaphDeployment(ctx, c); err != nil {
		return err
	}
	if err := logBareMetalHostStatus(ctx, c); err != nil {
		return err
	}
	if err := logHCloudMachineStatus(ctx, c); err != nil {
		return err
	}
	if err := logConditions(ctx, "mgt-cluster", restConfig); err != nil {
		return err
	}

	clusterList := &clusterv1.ClusterList{}
	err := c.List(ctx, clusterList)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}
	for _, cluster := range clusterList.Items {
		secretName := cluster.Name + "-kubeconfig"
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: cluster.Namespace,
			},
		}
		err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret)
		if err != nil {
			log(fmt.Sprintf("Failed to get Secret %s/%s: %v", cluster.Namespace, secretName, err))
			continue
		} else {
			log(fmt.Sprintf("Found Secret %s/%s", cluster.Namespace, secretName))
		}
		data := secret.Data["value"]
		if len(data) == 0 {
			log(fmt.Sprintf("Failed to get Secret %s/%s: content is empty", cluster.Namespace, secretName))
			continue
		}
		restConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
		if err != nil {
			log(fmt.Sprintf("Failed to create REST config from Secret %s/%s: %v", cluster.Namespace, secretName, err))
			continue
		}
		err = logConditions(ctx, "wl-cluster "+cluster.Name, restConfig)
		if err != nil {
			log(fmt.Sprintf("Failed to log Conditions %s/%s: %v", cluster.Namespace, secretName, err))
			continue
		}
	}
	log(fmt.Sprintf("%s End logging status >>>", time.Now().Format("2006-01-02 15:04:05")))

	return nil
}

func logConditions(ctx context.Context, clusterName string, restConfig *restclient.Config) error {
	restConfig.QPS = -1 // Since Kubernetes 1.29 "API Priority and Fairness" handles that.
	counter, err := checkconditions.RunAndGetCounter(ctx, restConfig, &checkconditions.Arguments{})
	if err != nil {
		return fmt.Errorf("failed to get check conditions: %w", err)
	}
	log(fmt.Sprintf("----------------------------------------------- %s ---- Unhealthy Conditions: %d",
		clusterName,
		len(counter.Lines)))

	for _, line := range counter.Lines {
		log(line)
	}
	return nil
}

func logHCloudMachineStatus(ctx context.Context, c client.Client) error {
	hmList := &infrav1.HCloudMachineList{}
	err := c.List(ctx, hmList)
	if err != nil {
		return err
	}

	if len(hmList.Items) == 0 {
		return nil
	}

	caphDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "caph-controller-manager",
			Namespace: "caph-system",
		},
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(&caphDeployment), &caphDeployment)
	if err != nil {
		return fmt.Errorf("failed to get caph-controller-manager deployment: %w", err)
	}

	log(fmt.Sprintf("--------------------------------------------------- HCloudMachines %s",
		caphDeployment.Spec.Template.Spec.Containers[0].Image))

	for i := range hmList.Items {
		hm := &hmList.Items[i]
		if hm.Status.InstanceState == nil || *hm.Status.InstanceState == "" {
			continue
		}
		addresses := make([]string, 0)
		for _, a := range hm.Status.Addresses {
			addresses = append(addresses, a.Address)
		}

		id := ""
		if *hm.Spec.ProviderID != "" {
			id = *hm.Spec.ProviderID
		}
		log("HCloudMachine: " + hm.Name + " " + id + " " + strings.Join(addresses, " "))
		log("  ProvisioningState: " + string(*hm.Status.InstanceState))
		l := make([]string, 0)
		if hm.Status.FailureMessage != nil {
			l = append(l, *hm.Status.FailureMessage)
		}
		if hm.Status.FailureMessage != nil {
			l = append(l, *hm.Status.FailureMessage)
		}
		if len(l) > 0 {
			log("  Error: " + strings.Join(l, ", "))
		}
		readyC := conditions.Get(hm, clusterv1.ReadyCondition)
		msg := ""
		reason := ""
		state := "?"
		if readyC != nil {
			msg = readyC.Message
			reason = readyC.Reason
			state = string(readyC.Status)
		}
		log("  Ready Condition: " + state + " " + reason + " " + msg)
	}
	return nil
}

func logCaphDeployment(ctx context.Context, c client.Client) error {
	caphDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "caph-controller-manager",
			Namespace: "caph-system",
		},
	}
	err := c.Get(ctx, client.ObjectKeyFromObject(&caphDeployment), &caphDeployment)
	if err != nil {
		return fmt.Errorf("failed to get caph-controller-manager deployment: %w", err)
	}

	log(fmt.Sprintf("--------------------------------------------------- Caph deployment %s",
		caphDeployment.Spec.Template.Spec.Containers[0].Image))

	return nil
}

func logBareMetalHostStatus(ctx context.Context, c client.Client) error {
	hbmhList := &infrav1.HetznerBareMetalHostList{}
	err := c.List(ctx, hbmhList)
	if err != nil {
		return err
	}

	if len(hbmhList.Items) == 0 {
		return nil
	}

	caphDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "caph-controller-manager",
			Namespace: "caph-system",
		},
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(&caphDeployment), &caphDeployment)
	if err != nil {
		return fmt.Errorf("failed to get caph-controller-manager deployment: %w", err)
	}

	log(fmt.Sprintf("--------------------------------------------------- BareMetalHosts %s",
		caphDeployment.Spec.Template.Spec.Containers[0].Image))

	for i := range hbmhList.Items {
		hbmh := &hbmhList.Items[i]
		if hbmh.Spec.Status.ProvisioningState == "" {
			continue
		}
		log("BareMetalHost: " + hbmh.Name + " " + fmt.Sprint(hbmh.Spec.ServerID))
		log("  ProvisioningState: " + string(hbmh.Spec.Status.ProvisioningState))
		eMsg := string(hbmh.Spec.Status.ErrorType) + " " + hbmh.Spec.Status.ErrorMessage
		eMsg = strings.TrimSpace(eMsg)
		if eMsg != "" {
			log("  Error: " + eMsg)
		}
		readyC := conditions.Get(hbmh, clusterv1.ReadyCondition)
		msg := ""
		reason := ""
		state := "?"
		if readyC != nil {
			msg = readyC.Message
			reason = readyC.Reason
			state = string(readyC.Status)
		}
		log("  Ready Condition: " + state + " " + reason + " " + msg)
	}
	return nil
}

func initBootstrapCluster(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig, clusterctlConfig, artifactFolder string) {
	go logStatusContinuously(ctx, bootstrapClusterProxy.GetRESTConfig(), bootstrapClusterProxy.GetClient())
	clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            bootstrapClusterProxy,
		ClusterctlConfigPath:    clusterctlConfig,
		InfrastructureProviders: config.InfrastructureProviders(),
		LogFolder:               filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
}

func tearDown(ctx context.Context, bootstrapClusterProvider bootstrap.ClusterProvider, bootstrapClusterProxy framework.ClusterProxy) {
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(ctx)
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(ctx)
	}
}

func log(msg string) {
	fmt.Fprintln(GinkgoWriter, msg)
}
