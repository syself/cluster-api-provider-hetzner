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
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// CaphClusterDeploymentSpecInput is the input for CaphClusterDeploymentSpec.
type CaphClusterDeploymentSpecInput struct {
	E2EConfig                *clusterctl.E2EConfig
	ClusterctlConfigPath     string
	BootstrapClusterProxy    framework.ClusterProxy
	ArtifactFolder           string
	SkipCleanup              bool
	Flavor                   string
	WorkerMachineCount       int64
	ControlPlaneMachineCount int64
}

func logHCloudMachineStatusContinously(ctx context.Context, c client.Client) {
	for {
		t := time.After(30 * time.Second)
		select {
		case <-ctx.Done():
			return
		case <-t:
			err := logHCloudMachineStatus(ctx, c)
			if err != nil {
				ginkgo.By(fmt.Sprintf("Error logging HCloudMachine status: %v", err))
			}
		}
	}
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

	ginkgo.By(fmt.Sprintf("--------------------------------------------------- HCloudMachines %s",
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
		ginkgo.By("HCloudMachine: " + hm.Name + " " + id + " " + strings.Join(addresses, " "))
		ginkgo.By("  ProvisioningState: " + string(*hm.Status.InstanceState))
		l := make([]string, 0)
		if hm.Status.FailureMessage != nil {
			l = append(l, *hm.Status.FailureMessage)
		}
		if hm.Status.FailureMessage != nil {
			l = append(l, *hm.Status.FailureMessage)
		}
		if len(l) > 0 {
			ginkgo.By("  Error: " + strings.Join(l, ", "))
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
		ginkgo.By("  Ready Condition: " + state + " " + reason + " " + msg)
	}
	return nil
}

// CaphClusterDeploymentSpec implements a test that verifies that MachineDeployment rolling updates are successful.
func CaphClusterDeploymentSpec(ctx context.Context, inputGetter func() CaphClusterDeploymentSpecInput) {
	var (
		specName         = "ci" + os.Getenv("GITHUB_RUN_ID")
		input            CaphClusterDeploymentSpecInput
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
		gomega.Expect(os.MkdirAll(input.ArtifactFolder, 0o750)).To(gomega.Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)
		gomega.Expect(input.E2EConfig.Variables).To(gomega.HaveKey(KubernetesVersion))
		gomega.Expect(input.E2EConfig.Variables).To(HaveValidVersion(input.E2EConfig.GetVariable(KubernetesVersion)))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		clusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
	})

	ginkgo.It("Should successfully create a cluster with three control planes", func() {
		ginkgo.By("Creating a workload cluster")
		go logHCloudMachineStatusContinously(ctx, input.BootstrapClusterProxy.GetClient())
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
				ControlPlaneMachineCount: ptr.To(input.ControlPlaneMachineCount),
				WorkerMachineCount:       ptr.To(input.WorkerMachineCount),
			},
			WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, clusterResources)

		ginkgo.By("PASSED!")
	})

	ginkgo.AfterEach(func() {
		// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
		dumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder, namespace, cancelWatches, clusterResources.Cluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}
