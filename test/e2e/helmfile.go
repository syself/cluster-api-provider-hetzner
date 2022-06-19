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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/yaml"
)

// HelmfileClusterDeploymentSpecInput is the input for HelmfileClusterDeploymentSpec.
type HelmfileClusterDeploymentSpecInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string
	SkipCleanup           bool
	Flavor                string
	WorkerMachineCount    int64
}

type HelmfileApp struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Environment string `json:"environment"`
	Selector    string `json:"selector"`
}

type HelmfileApps struct {
	Apps []HelmfileApp `json:"apps"`
}

// HelmfileClusterDeploymentSpec implements a test that verifies that MachineDeployment rolling updates are successful.
func HelmfileClusterDeploymentSpec(ctx context.Context, inputGetter func() HelmfileClusterDeploymentSpecInput) {
	var (
		specName         = "helmfile"
		input            HelmfileClusterDeploymentSpecInput
		namespace        *corev1.Namespace
		cancelWatches    context.CancelFunc
		clusterResources *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName      string
	)

	ginkgo.BeforeEach(func() {
		gomega.Expect(ctx).NotTo(gomega.BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		gomega.Expect(input.E2EConfig).ToNot(gomega.BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		gomega.Expect(input.ClusterctlConfigPath).To(gomega.BeAnExistingFile(), "Invalid argument. input.ClusterctlConfigPath must be an existing file when calling %s spec", specName)
		gomega.Expect(input.BootstrapClusterProxy).ToNot(gomega.BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		gomega.Expect(os.MkdirAll(input.ArtifactFolder, 0750)).To(gomega.Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)
		gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(KubernetesVersion))
		gomega.Expect(input.E2EConfig.Variables).To(HaveValidVersion(input.E2EConfig.GetVariable(KubernetesVersion)))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		clusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
	})

	ginkgo.It("Should successfully create a cluster with three control planes", func() {
		ginkgo.By("Creating a workload cluster")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: input.BootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(input.ArtifactFolder, "clusters", input.BootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     input.ClusterctlConfigPath,
				KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   input.Flavor,
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        input.E2EConfig.GetVariable(KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(3),
				WorkerMachineCount:       pointer.Int64Ptr(1),
			},
			WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, clusterResources)

		workloadProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterResources.Cluster.Name)

		helmfileConfigPath := input.E2EConfig.GetVariable(HelmFileConfigPath)

		// Read values from config into struct
		helmfileApps, err := getHelmfileApps(helmfileConfigPath)
		gomega.Expect(err).To(gomega.Succeed())

		// Create parent directory
		input.ArtifactFolder = framework.ResolveArtifactsDirectory(input.ArtifactFolder)
		reportDir := path.Join(input.ArtifactFolder, "helmfile")
		parentDir := path.Join(reportDir, "e2e-output")
		gomega.Expect(os.MkdirAll(parentDir, 0o750)).To(gomega.Succeed())
		// Loop

		for _, app := range helmfileApps.Apps {
			outputDir := path.Join(parentDir, app.Name)
			gomega.Expect(os.MkdirAll(outputDir, 0o750)).To(gomega.Succeed())
			cmdString := fmt.Sprintf(
				"KUBECONFIG=%s helmfile -f %s --environment %s --selector %s sync",
				workloadProxy.GetKubeconfigPath(),
				app.Path,
				app.Environment,
				app.Selector,
			)

			cmd := exec.Command(cmdString)

			// Open the output file
			outfile, err := os.Create(outputDir + "/info.txt")
			gomega.Expect(err).To(gomega.Succeed())
			defer outfile.Close()

			// Send stdout to the outfile
			cmd.Stdout = outfile

			// Open the error file
			errfile, err := os.Create(outputDir + "error.txt")
			gomega.Expect(err).To(gomega.Succeed())
			defer errfile.Close()

			// Send stderr to the errfile
			cmd.Stderr = errfile

			Byf("Start command %s", cmdString)
			gomega.Expect(cmd.Run()).To(gomega.Succeed())
			gomega.Expect(errfile)

			f, err := os.Open(outputDir + "error.txt")
			gomega.Expect(err).To(gomega.Succeed())
			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				ginkgo.Fail(fmt.Sprintf("stderr is non-empty. Failed helmfile command %s", app.Name))
			}
		}
		ginkgo.By("PASSED!")
	})

	ginkgo.AfterEach(func() {
		// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
		dumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder, namespace, cancelWatches, clusterResources.Cluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
		redactLogs(input.E2EConfig.GetVariable)
	})
}

func getHelmfileApps(path string) (*HelmfileApps, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("unable to read helmfile config file %s: %w", path, err)
	}

	jsonBytes, err := yaml.YAMLToJSON(data)

	helmfileApps := &HelmfileApps{}
	if err := json.Unmarshal(jsonBytes, helmfileApps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data into struct. Data: %s", string(data))
	}

	return helmfileApps, err
}
