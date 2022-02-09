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

package helpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

// Util functions to interact with the clusterctl e2e framework

// LoadE2EConfig loads the e2e config of CAPI e2e tests.
func LoadE2EConfig(ctx context.Context, configPath string) (*clusterctl.E2EConfig, error) {
	config := clusterctl.LoadE2EConfig(ctx, clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
	if config == nil {
		return nil, fmt.Errorf("cannot load E2E config found at %s", configPath)
	}
	return config, nil
}

// CreateClusterctlLocalRepository uses capi e2e functions to create a local repository.
func CreateClusterctlLocalRepository(ctx context.Context, config *clusterctl.E2EConfig, repositoryFolder string, cniEnabled bool) (string, error) {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}
	if cniEnabled {
		// Ensuring a CNI file is defined in the config and register a FileTransformation to inject the referenced file as in place of the CNI_RESOURCES envSubst variable.
		cniPath, ok := config.Variables[capi_e2e.CNIPath]
		if !ok {
			return "", fmt.Errorf("missing %s variable in the config", capi_e2e.CNIPath)
		}

		if _, err := os.Stat(cniPath); err != nil {
			return "", fmt.Errorf("the %s variable should resolve to an existing file", capi_e2e.CNIPath)
		}
		createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(cniPath, capi_e2e.CNIResources)
	}

	clusterctlConfig := clusterctl.CreateRepository(ctx, createRepositoryInput)
	if _, err := os.Stat(clusterctlConfig); err != nil {
		return "", fmt.Errorf("the clusterctl config file does not exists in the local repository %s", repositoryFolder)
	}
	return clusterctlConfig, nil
}

// SetupBootstrapCluster sets up a bootstrap cluster using CAPI e2e functions.
func SetupBootstrapCluster(ctx context.Context, config *clusterctl.E2EConfig, scheme *runtime.Scheme, useExistingCluster bool, artifactFolder string) (bootstrap.ClusterProvider, framework.ClusterProxy, error) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               config.ManagementClusterName,
			RequiresDockerSock: config.HasDockerProvider(),
			Images:             config.Images,
			LogFolder:          filepath.Join(artifactFolder, "kind"),
		})

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		if _, err := os.Stat(kubeconfigPath); err != nil {
			return nil, nil, errors.New("failed to get the kubeconfig file for the bootstrap cluster")
		}
	}

	clusterProxy := framework.NewClusterProxy("bootstrap", kubeconfigPath, scheme)

	return clusterProvider, clusterProxy, nil
}

// InitBootstrapCluster initializes the management cluster using CAPI e2e.
func InitBootstrapCluster(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig, clusterctlConfig, artifactFolder string) {
	clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            bootstrapClusterProxy,
		ClusterctlConfigPath:    clusterctlConfig,
		InfrastructureProviders: config.InfrastructureProviders(),
		LogFolder:               filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
}

// TearDown tears down the bootstrap cluster proxy and provider.
func TearDown(ctx context.Context, bootstrapClusterProvider bootstrap.ClusterProvider, bootstrapClusterProxy framework.ClusterProxy) {
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(ctx)
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(ctx)
	}
}
