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
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// ClusterScopeParams defines the input parameters used to create a new scope.
type ClusterScopeParams struct {
	Client         client.Client
	APIReader      client.Reader
	Logger         logr.Logger
	HetznerSecret  *corev1.Secret
	HCloudClient   hcloudclient.Client
	Cluster        *clusterv1.Cluster
	HetznerCluster *infrav1.HetznerCluster
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.HetznerCluster == nil {
		return nil, errors.New("failed to generate new scope from nil HetznerCluster")
	}
	if params.HCloudClient == nil {
		return nil, errors.New("failed to generate new scope from nil HCloudClient")
	}
	if params.APIReader == nil {
		return nil, errors.New("failed to generate new scope from nil APIReader")
	}

	emptyLogger := logr.Logger{}
	if params.Logger == emptyLogger {
		return nil, errors.New("failed to generate new scope from nil Logger")
	}

	helper, err := patch.NewHelper(params.HetznerCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ClusterScope{
		Logger:         params.Logger,
		Client:         params.Client,
		APIReader:      params.APIReader,
		Cluster:        params.Cluster,
		HetznerCluster: params.HetznerCluster,
		HCloudClient:   params.HCloudClient,
		patchHelper:    helper,
		hetznerSecret:  params.HetznerSecret,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	Client        client.Client
	APIReader     client.Reader
	patchHelper   *patch.Helper
	hetznerSecret *corev1.Secret

	HCloudClient hcloudclient.Client

	Cluster        *clusterv1.Cluster
	HetznerCluster *infrav1.HetznerCluster
}

// Name returns the HetznerCluster name.
func (s *ClusterScope) Name() string {
	return s.HetznerCluster.Name
}

// Namespace returns the namespace name.
func (s *ClusterScope) Namespace() string {
	return s.HetznerCluster.Namespace
}

// HetznerSecret returns the hetzner secret.
func (s *ClusterScope) HetznerSecret() *corev1.Secret {
	return s.hetznerSecret
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	conditions.SetSummary(s.HetznerCluster)
	return s.patchHelper.Patch(ctx, s.HetznerCluster)
}

// PatchObject persists the machine spec and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.HetznerCluster)
}

// GetSpecRegion returns a region.
func (s *ClusterScope) GetSpecRegion() []infrav1.Region {
	return s.HetznerCluster.Spec.ControlPlaneRegions
}

// SetStatusFailureDomain sets the region for the status.
func (s *ClusterScope) SetStatusFailureDomain(regions []infrav1.Region) {
	s.HetznerCluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	for _, region := range regions {
		s.HetznerCluster.Status.FailureDomains[string(region)] = clusterv1.FailureDomainSpec{
			ControlPlane: true,
		}
	}
}

// ControlPlaneAPIEndpointPort returns the Port of the Kube-api server.
func (s *ClusterScope) ControlPlaneAPIEndpointPort() int32 {
	return int32(s.HetznerCluster.Spec.ControlPlaneLoadBalancer.Port) //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
}

// ClientConfig return a kubernetes client config for the cluster context.
func (s *ClusterScope) ClientConfig(ctx context.Context) (clientcmd.ClientConfig, error) {
	cluster := client.ObjectKey{
		Name:      fmt.Sprintf("%s-%s", s.Cluster.Name, secret.Kubeconfig),
		Namespace: s.Cluster.Namespace,
	}

	secretManager := secretutil.NewSecretManager(s.Logger, s.Client, s.APIReader)
	kubeconfigSecret, err := secretManager.AcquireSecret(ctx, cluster, s.HetznerCluster, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire secret: %w", err)
	}
	kubeconfigBytes, ok := kubeconfigSecret.Data[secret.KubeconfigDataName]
	if !ok {
		return nil, fmt.Errorf("missing key %q in secret data", secret.KubeconfigDataName)
	}
	return clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
}

// ClientConfigWithAPIEndpoint returns a client config.
func (s *ClusterScope) ClientConfigWithAPIEndpoint(ctx context.Context, endpoint clusterv1.APIEndpoint) (clientcmd.ClientConfig, error) {
	c, err := s.ClientConfig(ctx)
	if err != nil {
		return nil, err
	}

	raw, err := c.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("error retrieving rawConfig from clientConfig: %w", err)
	}
	// update cluster endpint in config
	for key := range raw.Clusters {
		raw.Clusters[key].Server = fmt.Sprintf("https://%s:%d", endpoint.Host, endpoint.Port)
	}

	return clientcmd.NewDefaultClientConfig(raw, &clientcmd.ConfigOverrides{}), nil
}

// ListMachines returns HCloudMachines.
func (s *ClusterScope) ListMachines(ctx context.Context) ([]*clusterv1.Machine, []*infrav1.HCloudMachine, error) {
	// get and index Machines by HCloudMachine name
	var machineListRaw clusterv1.MachineList
	machineByHCloudMachineName := make(map[string]*clusterv1.Machine)
	if err := s.Client.List(ctx, &machineListRaw, client.InNamespace(s.Namespace())); err != nil {
		return nil, nil, err
	}
	expectedGK := infrav1.GroupVersion.WithKind("HCloudMachine").GroupKind()
	for pos := range machineListRaw.Items {
		m := &machineListRaw.Items[pos]
		actualGK := m.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if m.Spec.ClusterName != s.Cluster.Name ||
			actualGK.String() != expectedGK.String() {
			continue
		}
		machineByHCloudMachineName[m.Spec.InfrastructureRef.Name] = m
	}

	// match HCloudMachines to Machines
	var hcloudMachineListRaw infrav1.HCloudMachineList
	if err := s.Client.List(ctx, &hcloudMachineListRaw, client.InNamespace(s.Namespace())); err != nil {
		return nil, nil, err
	}

	machineList := make([]*clusterv1.Machine, 0, len(hcloudMachineListRaw.Items))
	hcloudMachineList := make([]*infrav1.HCloudMachine, 0, len(hcloudMachineListRaw.Items))

	for pos := range hcloudMachineListRaw.Items {
		hm := &hcloudMachineListRaw.Items[pos]
		m, ok := machineByHCloudMachineName[hm.Name]
		if !ok {
			continue
		}

		machineList = append(machineList, m)
		hcloudMachineList = append(hcloudMachineList, hm)
	}

	return machineList, hcloudMachineList, nil
}

// IsControlPlaneReady returns nil if the control plane is ready.
func IsControlPlaneReady(ctx context.Context, c clientcmd.ClientConfig) error {
	restConfig, err := c.ClientConfig()
	if err != nil {
		return err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	_, err = clientSet.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(ctx)
	return err
}
