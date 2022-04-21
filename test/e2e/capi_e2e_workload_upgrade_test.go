package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	"k8s.io/utils/pointer"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var _ = Describe("[Workload Upgrade] Running the Cluster API E2E Workload Cluster Upgrade tests", func() {
	ctx := context.TODO()

	// The following upstream tests are not implemented because they are subsets of
	// capi_e2e.ClusterUpgradeConformanceSpec:
	// - capi_e2e.MachineDeploymentScaleSpec
	// - capi_e2e.MachineDeploymentRolloutSpec

	Context("Running the cluster-upgrade spec with single control plane instance", func() {
		capi_e2e.ClusterUpgradeConformanceSpec(ctx, func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				SkipConformanceTests:     true,
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
				Flavor:                   pointer.String(clusterctl.DefaultFlavor),
			}
		})
	})

	Context("Running the cluster-upgrade spec with HA control plane", func() {
		capi_e2e.ClusterUpgradeConformanceSpec(ctx, func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				SkipConformanceTests:     true,
				ControlPlaneMachineCount: pointer.Int64(3),
				WorkerMachineCount:       pointer.Int64(1),
				Flavor:                   pointer.String(clusterctl.DefaultFlavor),
			}
		})
	})

	Context("Running the cluster-upgrade spec with HA control plane and scale-in", func() {
		capi_e2e.ClusterUpgradeConformanceSpec(ctx, func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				SkipConformanceTests:     true,
				ControlPlaneMachineCount: pointer.Int64(3),
				WorkerMachineCount:       pointer.Int64(1),
				Flavor:                   pointer.String("kcp-scale-in"),
			}
		})
	})
})
