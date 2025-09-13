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

// Package e2e provides methods to test CAPH provider integration e2e.
package e2e

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/blang/semver/v4"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

// Test suite constants for e2e config variables.
const (
	RedactLogScriptPath = "REDACT_LOG_SCRIPT"
	KubernetesVersion   = "KUBERNETES_VERSION"
	CiliumPath          = "CILIUM"
	CiliumResources     = "CILIUM_RESOURCES"

	// TODO: We should clean up here.
	// We only support the syself ccm.
	// To make this clear, we should use the term "syself".
	// Currently (in this context) "hetzner" means the syself-ccm,
	// and "hcloud" means the hcloud ccm (which now supports bare-metal, too)
	// Nevertheless, the hcloud/hetzner ccm is not supported.
	CCMPath             = "CCM"
	CCMResources        = "CCM_RESOURCES"
	CCMNetworkPath      = "CCM_NETWORK"
	CCMNetworkResources = "CCM_RESOURCES_NETWORK"
	CCMHetznerPath      = "CCM_HETZNER"
	CCMHetznerResources = "CCM_RESOURCES_HETZNER"
)

// Byf implements Ginkgo's By with fmt.Sprintf.
func Byf(format string, a ...interface{}) {
	ginkgo.By(fmt.Sprintf(format, a...))
}

func setupSpecNamespace(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string) (*corev1.Namespace, context.CancelFunc) {
	Byf("Creating a namespace for hosting the %q test spec", specName)
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   clusterProxy.GetClient(),
		ClientSet: clusterProxy.GetClientSet(),
		Name:      fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
		LogFolder: filepath.Join(artifactFolder, "clusters", clusterProxy.GetName()),
	})

	return namespace, cancelWatches
}

func dumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace *corev1.Namespace, cancelWatches context.CancelFunc, cluster *clusterv1.Cluster, intervalsGetter func(spec, key string) []interface{}, skipCleanup bool, kubeConfigPath, clusterctlConfigPath string) {
	var clusterName string
	var clusterNamespace string
	if cluster != nil {
		clusterName = cluster.Name
		clusterNamespace = cluster.Namespace
		Byf("Dumping logs from the %q workload cluster", clusterName)

		// Dump all the logs from the workload cluster before deleting them.
		clusterProxy.CollectWorkloadClusterLogs(ctx, clusterNamespace, clusterName, filepath.Join(artifactFolder, "clusters", clusterName))

		Byf("Dumping all the Cluster API resources in the %q namespace", namespace.Name)

		// Dump all Cluster API related resources to artifacts before deleting them.
		framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
			Lister:               clusterProxy.GetClient(),
			Namespace:            namespace.Name,
			LogPath:              filepath.Join(artifactFolder, "clusters", clusterProxy.GetName(), "resources"),
			KubeConfigPath:       kubeConfigPath,
			ClusterctlConfigPath: clusterctlConfigPath,
		})
	} else {
		clusterName = "empty"
		clusterNamespace = "empty"
	}

	if !skipCleanup {
		Byf("Deleting cluster %s/%s", clusterNamespace, clusterName)
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			ClusterProxy:         clusterProxy,
			Namespace:            namespace.Name,
			ClusterctlConfigPath: clusterctlConfigPath,
		}, intervalsGetter(specName, "wait-delete-cluster")...)

		Byf("Deleting namespace used for hosting the %q test spec", specName)
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: clusterProxy.GetClient(),
			Name:    namespace.Name,
		})
	}
	cancelWatches()
}

// HaveValidVersion succeeds if version is a valid semver version.
func HaveValidVersion(version string) types.GomegaMatcher {
	return &validVersionMatcher{version: version}
}

type validVersionMatcher struct{ version string }

func (m *validVersionMatcher) Match(_ interface{}) (success bool, err error) {
	if _, err := semver.ParseTolerant(m.version); err != nil {
		return false, err
	}
	return true, nil
}

func (m *validVersionMatcher) FailureMessage(_ interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\n%s", m.version, " to be a valid version ")
}

func (m *validVersionMatcher) NegatedFailureMessage(_ interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\n%s", m.version, " not to be a valid version ")
}
