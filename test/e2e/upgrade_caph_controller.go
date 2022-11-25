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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/discovery"
	"k8s.io/utils/pointer"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	c "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	initWithBinaryVariableName            = "INIT_WITH_BINARY"
	initWithProvidersContract             = "INIT_WITH_PROVIDERS_CONTRACT"
	initWithKubernetesVersion             = "INIT_WITH_KUBERNETES_VERSION"
	initWithInfrastructureProviderVersion = "INIT_WITH_INFRASTRUCTURE_PROVIDER_VERSION"
)

// ClusterctlUpgradeSpecInput is the input for ClusterctlUpgradeSpec.
type ClusterctlUpgradeSpecInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string
	// InitWithBinary can be used to override the INIT_WITH_BINARY e2e config variable with the URL of the clusterctl binary of the old version of Cluster API. The spec will interpolate the
	// strings `{OS}` and `{ARCH}` to `runtime.GOOS` and `runtime.GOARCH` respectively, e.g. https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.3.23/clusterctl-{OS}-{ARCH}
	InitWithBinary string
	// InitWithInfrastructureProviderVersion can be used to define the initialised version of the infrastructure provider.
	InitWithInfrastructureProviderVersion string
	// InitWithProvidersContract can be used to override the INIT_WITH_PROVIDERS_CONTRACT e2e config variable with a specific
	// provider contract to use to initialise the secondary management cluster, e.g. `v1alpha3`
	InitWithProvidersContract string
	SkipCleanup               bool
	PreInit                   func(managementClusterProxy framework.ClusterProxy)
	PreUpgrade                func(managementClusterProxy framework.ClusterProxy)
	PostUpgrade               func(managementClusterProxy framework.ClusterProxy)
	MgmtFlavor                string
	WorkloadFlavor            string
}

// ClusterctlUpgradeSpec implements a test that verifies clusterctl upgrade of a management cluster.
// It tests an older infrastructure provider against a newer one.
func ClusterctlUpgradeSpec(ctx context.Context, inputGetter func() ClusterctlUpgradeSpecInput) {
	var (
		specName = "upgrade-caph"
		input    ClusterctlUpgradeSpecInput

		testNamespace     *corev1.Namespace
		testCancelWatches context.CancelFunc

		managementClusterName          string
		clusterctlBinaryURL            string
		managementClusterNamespace     *corev1.Namespace
		managementClusterCancelWatches context.CancelFunc
		managementClusterResources     *clusterctl.ApplyClusterTemplateAndWaitResult
		managementClusterProxy         framework.ClusterProxy

		workLoadClusterName string

		desiredInfrastructureVersion string
	)

	ginkgo.BeforeEach(func() {
		gomega.Expect(ctx).NotTo(gomega.BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		gomega.Expect(input.E2EConfig).ToNot(gomega.BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		gomega.Expect(input.ClusterctlConfigPath).To(gomega.BeAnExistingFile(), "Invalid argument. input.ClusterctlConfigPath must be an existing file when calling %s spec", specName)
		gomega.Expect(input.BootstrapClusterProxy).ToNot(gomega.BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		var clusterctlBinaryURLTemplate string
		if input.InitWithBinary == "" {
			gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(initWithBinaryVariableName), "Invalid argument. %s variable must be defined when calling %s spec", initWithBinaryVariableName, specName)
			gomega.Expect(input.E2EConfig.Variables[initWithBinaryVariableName]).ToNot(gomega.BeEmpty(), "Invalid argument. %s variable can't be empty when calling %s spec", initWithBinaryVariableName, specName)
			clusterctlBinaryURLTemplate = input.E2EConfig.GetVariable(initWithBinaryVariableName)
		} else {
			clusterctlBinaryURLTemplate = input.InitWithBinary
		}
		if input.InitWithInfrastructureProviderVersion == "" {
			gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(initWithInfrastructureProviderVersion), "Invalid argument. %s variable must be defined when calling %s spec", initWithBinaryVariableName, specName)
			gomega.Expect(input.E2EConfig.Variables[initWithInfrastructureProviderVersion]).ToNot(gomega.BeEmpty(), "Invalid argument. %s variable can't be empty when calling %s spec", initWithBinaryVariableName, specName)
			desiredInfrastructureVersion = input.E2EConfig.GetVariable(initWithInfrastructureProviderVersion)
		} else {
			desiredInfrastructureVersion = input.InitWithInfrastructureProviderVersion
		}

		clusterctlBinaryURLReplacer := strings.NewReplacer("{OS}", runtime.GOOS, "{ARCH}", runtime.GOARCH)
		clusterctlBinaryURL = clusterctlBinaryURLReplacer.Replace(clusterctlBinaryURLTemplate)
		gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(initWithKubernetesVersion))
		gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(KubernetesVersion))
		gomega.Expect(os.MkdirAll(input.ArtifactFolder, 0750)).To(gomega.Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		managementClusterNamespace, managementClusterCancelWatches = setupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)
		managementClusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	})

	ginkgo.It("Should create a management cluster and then upgrade all the providers", func() {
		ginkgo.By("Creating a workload cluster to be used as a new management cluster")
		// NOTE: given that the bootstrap cluster could be shared by several tests, it is not practical to use it for testing clusterctl upgrades.
		// So we are creating a workload cluster that will be used as a new management cluster where to install older version of providers
		managementClusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: input.BootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(input.ArtifactFolder, "clusters", input.BootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     input.ClusterctlConfigPath,
				KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   input.MgmtFlavor,
				Namespace:                managementClusterNamespace.Name,
				ClusterName:              managementClusterName,
				KubernetesVersion:        input.E2EConfig.GetVariable(initWithKubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(1),
				WorkerMachineCount:       pointer.Int64Ptr(1),
			},
			WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, managementClusterResources)

		ginkgo.By("Turning the workload cluster into a management cluster with older versions of providers")

		// If the cluster is a DockerCluster, we should load controller images into the nodes.
		// Nb. this can be achieved also by changing the DockerMachine spec, but for the time being we are using
		// this approach because this allows to have a single source of truth for images, the e2e config
		// Nb. the images for official version of the providers will be pulled from internet, but the latest images must be
		// built locally and loaded into kind
		cluster := managementClusterResources.Cluster
		if cluster.Spec.InfrastructureRef.Kind == "DockerCluster" {
			gomega.Expect(bootstrap.LoadImagesToKindCluster(ctx, bootstrap.LoadImagesToKindClusterInput{
				Name:   cluster.Name,
				Images: input.E2EConfig.Images,
			})).To(gomega.Succeed())
		}

		// Get a ClusterProxy so we can interact with the workload cluster
		managementClusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name)

		// Download the older clusterctl version to be used for setting up the management cluster to be upgraded

		fmt.Fprintf(ginkgo.GinkgoWriter, "Downloading clusterctl binary from %s", clusterctlBinaryURL)
		clusterctlBinaryPath := downloadToTmpFile(ctx, clusterctlBinaryURL)
		defer os.Remove(clusterctlBinaryPath) // clean up

		err := os.Chmod(clusterctlBinaryPath, 0744) //nolint:gosec
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to chmod temporary file")

		ginkgo.By("Initializing the workload cluster with older versions of providers")

		// NOTE: by default we are considering all the providers, no matter of the contract.
		// However, given that we want to test both v1alpha3 --> v1beta1 and v1alpha4 --> v1beta1, the INIT_WITH_PROVIDERS_CONTRACT
		// variable can be used to select versions with a specific contract.
		contract := "*"
		if input.E2EConfig.HasVariable(initWithProvidersContract) {
			contract = input.E2EConfig.GetVariable(initWithProvidersContract)
		}
		if input.InitWithProvidersContract != "" {
			contract = input.InitWithProvidersContract
		}

		if input.PreInit != nil {
			ginkgo.By("Running Pre-init steps against the management cluster")
			input.PreInit(managementClusterProxy)
		}

		clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterctlBinaryPath:    clusterctlBinaryPath, // use older version of clusterctl to init the management cluster
			ClusterProxy:            managementClusterProxy,
			ClusterctlConfigPath:    input.ClusterctlConfigPath,
			CoreProvider:            input.E2EConfig.GetProviderLatestVersionsByContract(contract, config.ClusterAPIProviderName)[0],
			BootstrapProviders:      input.E2EConfig.GetProviderLatestVersionsByContract(contract, config.KubeadmBootstrapProviderName),
			ControlPlaneProviders:   input.E2EConfig.GetProviderLatestVersionsByContract(contract, config.KubeadmControlPlaneProviderName),
			InfrastructureProviders: getProviderWithSpecifiedVersionByContract(input.E2EConfig, contract, desiredInfrastructureVersion, input.E2EConfig.InfrastructureProviders()...),
			LogFolder:               filepath.Join(input.ArtifactFolder, "clusters", cluster.Name),
		}, input.E2EConfig.GetIntervals(specName, "wait-controllers")...)

		ginkgo.By("THE MANAGEMENT CLUSTER WITH THE OLDER VERSION OF PROVIDERS IS UP & RUNNING!")

		Byf("Creating a namespace for hosting the %s test workload cluster", specName)
		testNamespace, testCancelWatches = framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
			Creator:   managementClusterProxy.GetClient(),
			ClientSet: managementClusterProxy.GetClientSet(),
			Name:      specName,
			LogFolder: filepath.Join(input.ArtifactFolder, "clusters", "bootstrap"),
		})

		ginkgo.By("Creating a test workload cluster")

		// NOTE: This workload cluster is used to check the old management cluster works fine.
		// In this case ApplyClusterTemplateAndWait can't be used because this helper is linked to the last version of the API;
		// so we are getting a template using the downloaded version of clusterctl, applying it, and wait for machines to be provisioned.

		workLoadClusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
		kubernetesVersion := input.E2EConfig.GetVariable(KubernetesVersion)
		controlPlaneMachineCount := pointer.Int64Ptr(1)
		workerMachineCount := pointer.Int64Ptr(1)

		fmt.Fprintf(ginkgo.GinkgoWriter, "Creating the workload cluster with name %q using the %q template (Kubernetes %s, %d control-plane machines, %d worker machines)",
			workLoadClusterName, "(default)", kubernetesVersion, *controlPlaneMachineCount, *workerMachineCount)

		fmt.Fprintf(ginkgo.GinkgoWriter, "Getting the cluster template yaml")
		workloadClusterTemplate := clusterctl.ConfigClusterWithBinary(ctx, clusterctlBinaryPath, clusterctl.ConfigClusterInput{
			// pass reference to the management cluster hosting this test
			KubeconfigPath: managementClusterProxy.GetKubeconfigPath(),
			// pass the clusterctl config file that points to the local provider repository created for this test,
			ClusterctlConfigPath: input.ClusterctlConfigPath,
			// select template
			Flavor: input.WorkloadFlavor,
			// define template variables
			Namespace:                testNamespace.Name,
			ClusterName:              workLoadClusterName,
			KubernetesVersion:        kubernetesVersion,
			ControlPlaneMachineCount: controlPlaneMachineCount,
			WorkerMachineCount:       workerMachineCount,
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			// setup clusterctl logs folder
			LogFolder: filepath.Join(input.ArtifactFolder, "clusters", managementClusterProxy.GetName()),
		})
		gomega.Expect(workloadClusterTemplate).ToNot(gomega.BeNil(), "Failed to get the cluster template")

		fmt.Fprintf(ginkgo.GinkgoWriter, "Applying the cluster template yaml to the cluster")
		gomega.Expect(managementClusterProxy.Apply(ctx, workloadClusterTemplate)).To(gomega.Succeed())

		ginkgo.By("Waiting for the machines to exists")
		gomega.Eventually(func() (int64, error) {
			var n int64
			machineList := &clusterv1alpha3.MachineList{}
			if err := managementClusterProxy.GetClient().List(ctx, machineList, c.InNamespace(testNamespace.Name), c.MatchingLabels{clusterv1.ClusterLabelName: workLoadClusterName}); err == nil {
				for _, machine := range machineList.Items {
					if machine.Status.NodeRef != nil {
						n++
					}
				}
			}
			return n, nil
		}, input.E2EConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(gomega.Equal(*controlPlaneMachineCount + *workerMachineCount))

		ginkgo.By("THE MANAGEMENT CLUSTER WITH THE OLDER VERSION OF CAPH WORKS!")

		if input.PreUpgrade != nil {
			ginkgo.By("Running Pre-upgrade steps against the management cluster")
			input.PreUpgrade(managementClusterProxy)
		}

		ginkgo.By("Upgrading providers to the latest version available")
		clusterctl.UpgradeManagementClusterAndWait(ctx, clusterctl.UpgradeManagementClusterAndWaitInput{
			ClusterctlConfigPath: input.ClusterctlConfigPath,
			ClusterProxy:         managementClusterProxy,
			Contract:             clusterv1.GroupVersion.Version,
			LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", cluster.Name),
		}, input.E2EConfig.GetIntervals(specName, "wait-controllers")...)

		ginkgo.By("THE MANAGEMENT CLUSTER WAS SUCCESSFULLY UPGRADED!")

		if input.PostUpgrade != nil {
			ginkgo.By("Running Post-upgrade steps against the management cluster")
			input.PostUpgrade(managementClusterProxy)
		}

		// After upgrading we are sure the version is the latest version of the API,
		// so it is possible to use the standard helpers

		testMachineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
			Lister:      managementClusterProxy.GetClient(),
			ClusterName: workLoadClusterName,
			Namespace:   testNamespace.Name,
		})

		framework.ScaleAndWaitMachineDeployment(ctx, framework.ScaleAndWaitMachineDeploymentInput{
			ClusterProxy:              managementClusterProxy,
			Cluster:                   &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name}},
			MachineDeployment:         testMachineDeployments[0],
			Replicas:                  2,
			WaitForMachineDeployments: input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
		})

		ginkgo.By("THE UPGRADED MANAGEMENT CLUSTER WORKS!")

		ginkgo.By("PASSED!")
	})

	ginkgo.AfterEach(func() {
		if testNamespace != nil {
			// Dump all the logs from the workload cluster before deleting them.
			managementClusterProxy.CollectWorkloadClusterLogs(ctx, testNamespace.Name, managementClusterName, filepath.Join(input.ArtifactFolder, "clusters", managementClusterName, "machines"))

			framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
				Lister:    managementClusterProxy.GetClient(),
				Namespace: testNamespace.Name,
				LogPath:   filepath.Join(input.ArtifactFolder, "clusters", managementClusterResources.Cluster.Name, "resources"),
			})

			if !input.SkipCleanup {
				switch {
				case discovery.ServerSupportsVersion(managementClusterProxy.GetClientSet().DiscoveryClient, clusterv1.GroupVersion) == nil:
					Byf("Deleting all %s clusters in namespace %s in management cluster %s", clusterv1.GroupVersion, testNamespace.Name, managementClusterName)
					framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
						Client:    managementClusterProxy.GetClient(),
						Namespace: testNamespace.Name,
					}, input.E2EConfig.GetIntervals(specName, "wait-delete-cluster")...)
				default:
					fmt.Fprintf(ginkgo.GinkgoWriter, "Management Cluster does not appear to support CAPI resources.")
				}

				Byf("Deleting cluster %s/%s", testNamespace.Name, managementClusterName)
				framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
					Client:    managementClusterProxy.GetClient(),
					Namespace: testNamespace.Name,
				}, input.E2EConfig.GetIntervals(specName, "wait-delete-cluster")...)

				Byf("Deleting namespace %s used for hosting the %q test", testNamespace.Name, specName)
				framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
					Deleter: managementClusterProxy.GetClient(),
					Name:    testNamespace.Name,
				})

				Byf("Deleting providers")
				clusterctl.Delete(ctx, clusterctl.DeleteInput{
					LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", managementClusterResources.Cluster.Name),
					ClusterctlConfigPath: input.ClusterctlConfigPath,
					KubeconfigPath:       managementClusterProxy.GetKubeconfigPath(),
				})
			}
			testCancelWatches()
		}

		// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
		dumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder, managementClusterNamespace, managementClusterCancelWatches, managementClusterResources.Cluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}

func downloadToTmpFile(ctx context.Context, url string) string {
	tmpFile, err := os.CreateTemp("", "clusterctl")
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to get temporary file")
	defer tmpFile.Close()

	// Get the data
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to create new http request")

	resp, err := http.DefaultClient.Do(req)
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to get clusterctl")
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(tmpFile, resp.Body)
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to write temporary file")

	return tmpFile.Name()
}

func getProviderWithSpecifiedVersionByContract(c *clusterctl.E2EConfig, contract string, desiredInfrastructureVersion string, providers ...string) []string {
	ret := make([]string, 0, len(providers))
	for _, p := range providers {
		versions := getVersions(c, p, contract)
		for _, v := range versions {
			if v == desiredInfrastructureVersion {
				ret = append(ret, fmt.Sprintf("%s:%s", p, v))
			}
		}
	}
	return ret
}

func getVersions(c *clusterctl.E2EConfig, provider string, contract string) []string {
	versions := []string{}
	for _, p := range c.Providers {
		if p.Name == provider {
			for _, v := range p.Versions {
				if contract == "*" || v.Contract == contract {
					versions = append(versions, v.Name)
				}
			}
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		// NOTE: Ignoring errors because the validity of the format is ensured by Validation.
		vI, _ := version.ParseSemantic(versions[i])
		vJ, _ := version.ParseSemantic(versions[j])
		return vI.LessThan(vJ)
	})
	return versions
}
