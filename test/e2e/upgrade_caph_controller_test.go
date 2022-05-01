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
	. "github.com/onsi/ginkgo"
)

var _ = Describe("[Upgrade CAPH][Slow] Testing Upgrade of Caph Controller", func() {
	Context("[Needs Published Image] Running tests that require published images", func() {
		Context("Testing the upgrade from the latest published caph version to a rc version", func() {
			ClusterctlUpgradeSpec(ctx, func() ClusterctlUpgradeSpecInput {
				return ClusterctlUpgradeSpecInput{
					E2EConfig:             e2eConfig,
					ClusterctlConfigPath:  clusterctlConfigPath,
					BootstrapClusterProxy: bootstrapClusterProxy,
					ArtifactFolder:        artifactFolder,
					SkipCleanup:           skipCleanup,
					WorkloadFlavor:        "",
				}
			})
		})
	})
})
