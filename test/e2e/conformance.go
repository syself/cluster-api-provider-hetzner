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
	"os"
	"path/filepath"
	"strconv"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/test/framework/kubetest"
	"sigs.k8s.io/cluster-api/util"
)

// ConformanceSpecInput is the input for ConformanceSpec.
type ConformanceSpecInput struct {
	E2EConfig              *clusterctl.E2EConfig
	ClusterctlConfigPath   string
	BootstrapClusterProxy  framework.ClusterProxy
	KubetestConfigFilePath string
	ArtifactFolder         string
	SkipCleanup            bool
	Flavor                 string
}

// ConformanceSpec implements a test that verifies that MachineDeployment rolling updates are successful.
func ConformanceSpec(ctx context.Context, inputGetter func() ConformanceSpecInput) {
	var (
		specName            = "conformance-tests"
		input               ConformanceSpecInput
		namespace           *corev1.Namespace
		cancelWatches       context.CancelFunc
		result              *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName         string
		clusterctlLogFolder string
	)

	ginkgo.BeforeEach(func() {
		gomega.Expect(ctx).NotTo(gomega.BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		gomega.Expect(input.E2EConfig).ToNot(gomega.BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		gomega.Expect(input.ClusterctlConfigPath).To(gomega.BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		gomega.Expect(input.BootstrapClusterProxy).ToNot(gomega.BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		gomega.Expect(os.MkdirAll(input.ArtifactFolder, 0755)).To(gomega.Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName) //#nosecs
		gomega.Expect(input.KubetestConfigFilePath).ToNot(gomega.BeNil(), "Invalid argument. kubetestConfigFilePath can't be nil")

		gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(KubernetesVersion))
		gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(capi_e2e.KubernetesVersion))
		gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(capi_e2e.CNIPath))

		clusterName = fmt.Sprintf("caph-cfm-%s", util.RandomString(6))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)

		clusterctlLogFolder = filepath.Join(input.ArtifactFolder, "clusters", input.BootstrapClusterProxy.GetName())

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	})

	ginkgo.AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder, namespace, cancelWatches, result.Cluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
		redactLogs(input.E2EConfig.GetVariable)
	})

	ginkgo.Measure(specName, func(b ginkgo.Benchmarker) {
		var err error

		workerMachineCount, err := strconv.ParseInt(input.E2EConfig.GetVariable("CONFORMANCE_WORKER_MACHINE_COUNT"), 10, 64)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		controlPlaneMachineCount, err := strconv.ParseInt(input.E2EConfig.GetVariable("CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT"), 10, 64)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		runtime := b.Time("cluster creation", func() {
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: input.BootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     input.ClusterctlConfigPath,
					KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   clusterctl.DefaultFlavor,
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        input.E2EConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(controlPlaneMachineCount),
					WorkerMachineCount:       pointer.Int64Ptr(workerMachineCount),
				},
				WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})

		b.RecordValue("cluster creation", runtime.Seconds())
		workloadProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)
		runtime = b.Time("conformance suite", func() {
			gomega.Expect(kubetest.Run(context.Background(),
				kubetest.RunInput{
					ClusterProxy:   workloadProxy,
					NumberOfNodes:  int(workerMachineCount),
					ConfigFilePath: input.KubetestConfigFilePath,
				},
			)).To(gomega.Succeed())
		})
		b.RecordValue("conformance suite run time", runtime.Seconds())
	}, 1)
}
