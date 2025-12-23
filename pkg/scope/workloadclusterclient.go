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

// Package scope defines cluster and machine scope as well as a repository for the Hetzner API.
package scope

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
)

// workloadClientConfigFromKubeconfigSecret creates a kubernetes client config from kubeconfig secret.
func workloadClientConfigFromKubeconfigSecret(ctx context.Context, logger logr.Logger, cl client.Client, apiReader client.Reader, cluster *clusterv1.Cluster, hetznerCluster *infrav1.HetznerCluster) (clientcmd.ClientConfig, error) {
	secretKey := client.ObjectKey{
		Name:      fmt.Sprintf("%s-%s", cluster.Name, secret.Kubeconfig),
		Namespace: cluster.Namespace,
	}

	secretManager := secretutil.NewSecretManager(logger, cl, apiReader)
	kubeconfigSecret, err := secretManager.AcquireSecret(ctx, secretKey, hetznerCluster, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire secret: %w", err)
	}
	kubeconfigBytes, ok := kubeconfigSecret.Data[secret.KubeconfigDataName]
	if !ok {
		return nil, fmt.Errorf("missing key %q in secret data (WorkloadClientConfigFromKubeconfigSecret)", secret.KubeconfigDataName)
	}
	return clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
}

// WorkloadClusterClientFactory is an interface to get a new controller-runtime Client to access a
// workload-cluster.
type WorkloadClusterClientFactory interface {
	// NewWorkloadClient returns a new client connected to the workload-cluster
	NewWorkloadClient(ctx context.Context) (client.Client, error)
}

type realWorkloadClusterClientFactory struct {
	logger         logr.Logger
	client         client.Client
	cluster        *clusterv1.Cluster
	hetznerCluster *infrav1.HetznerCluster
}

func (f *realWorkloadClusterClientFactory) NewWorkloadClient(ctx context.Context) (client.Client, error) {
	wlConfig, err := workloadClientConfigFromKubeconfigSecret(ctx, f.logger,
		f.client, f.client, f.cluster, f.hetznerCluster)
	if err != nil {
		return nil, fmt.Errorf("actionProvisioned (Reboot via Annotation),WorkloadClientConfigFromKubeconfigSecret failed: %w",
			err)
	}

	// getting client
	restConfig, err := wlConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("actionProvisioned (Reboot via Annotation), failed to get rest config: %w", err)
	}

	wlClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("client.New failed: %w", err)
	}
	return wlClient, nil
}
