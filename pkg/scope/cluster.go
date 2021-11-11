/*
Copyright 2018 The Kubernetes Authors.
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

// Package scope defines cluster and machine scope as well as a repository for the Hetzner API
package scope

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultControlPlaneAPIEndpointPort = 6443

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	HCloudClient

	Ctx                 context.Context
	HCloudClientFactory HCloudClientFactory
	Client              client.Client
	Logger              logr.Logger
	Cluster             *clusterv1.Cluster
	HetznerCluster      *infrav1.HetznerCluster
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
	if params.Logger == nil {
		params.Logger = klogr.New()
	}
	if params.Ctx == nil {
		params.Ctx = context.TODO()
	}

	// setup client factory if nothing was set
	var hcloudToken string
	if params.HCloudClientFactory == nil {
		params.HCloudClientFactory = func(ctx context.Context) (HCloudClient, error) {
			// retrieve token secret
			var tokenSecret corev1.Secret
			tokenSecretName := types.NamespacedName{Namespace: params.HetznerCluster.Namespace, Name: params.HetznerCluster.Spec.HCloudTokenRef.Name}
			if err := params.Client.Get(ctx, tokenSecretName, &tokenSecret); err != nil {
				return nil, errors.Errorf("error getting referenced token secret/%s: %s", tokenSecretName, err)
			}

			tokenBytes, keyExists := tokenSecret.Data[params.HetznerCluster.Spec.HCloudTokenRef.Key]
			if !keyExists {
				return nil, errors.Errorf("error key %s does not exist in secret/%s", params.HetznerCluster.Spec.HCloudTokenRef.Key, tokenSecretName)
			}
			hcloudToken = string(tokenBytes)

			return &realHCloudClient{client: hcloud.NewClient(hcloud.WithToken(hcloudToken)), token: hcloudToken}, nil
		}
	}

	hcc, err := params.HCloudClientFactory(params.Ctx)
	if err != nil {
		return nil, err
	}
	helper, err := patch.NewHelper(params.HetznerCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ClusterScope{
		Ctx:            params.Ctx,
		Logger:         params.Logger,
		Client:         params.Client,
		Cluster:        params.Cluster,
		HetznerCluster: params.HetznerCluster,
		hcloudClient:   hcc,
		hcloudToken:    hcloudToken,
		patchHelper:    helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	Ctx context.Context
	logr.Logger
	Client       client.Client
	patchHelper  *patch.Helper
	hcloudClient HCloudClient
	hcloudToken  string

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

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close() error {
	return s.patchHelper.Patch(s.Ctx, s.HetznerCluster)
}

// PatchObject persists the machine spec and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.HetznerCluster)
}

// HCloudClient gives a hcloud client.
func (s *ClusterScope) HCloudClient() HCloudClient {
	return s.hcloudClient
}

// GetSpecRegion returns a region.
func (s *ClusterScope) GetSpecRegion() []infrav1.HCloudRegion {
	return s.HetznerCluster.Spec.ControlPlaneRegion
}

// SetStatusFailureDomain sets the region for the status.
func (s *ClusterScope) SetStatusFailureDomain(regions []infrav1.HCloudRegion) {
	s.HetznerCluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	for _, l := range regions {
		s.HetznerCluster.Status.FailureDomains[string(l)] = clusterv1.FailureDomainSpec{
			ControlPlane: true,
		}
	}
}

// ControlPlaneAPIEndpointPort returns the Port of the Kube-api server.
func (s *ClusterScope) ControlPlaneAPIEndpointPort() int32 {
	return defaultControlPlaneAPIEndpointPort
}

// ClientConfig return a kubernetes client config for the cluster context.
func (s *ClusterScope) ClientConfig() (clientcmd.ClientConfig, error) {
	var cluster = client.ObjectKey{
		Name:      s.Cluster.Name,
		Namespace: s.Cluster.Namespace,
	}
	kubeconfigBytes, err := kubeconfig.FromSecret(s.Ctx, s.Client, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving kubeconfig for cluster")
	}
	return clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
}

// ClientConfigWithAPIEndpoint returns a client config.
func (s *ClusterScope) ClientConfigWithAPIEndpoint(endpoint clusterv1.APIEndpoint) (clientcmd.ClientConfig, error) {
	c, err := s.ClientConfig()
	if err != nil {
		return nil, err
	}

	raw, err := c.RawConfig()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving rawConfig from clientConfig")
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
	var machineByHCloudMachineName = make(map[string]*clusterv1.Machine)
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

// IsControlPlaneReady returns if a machine is a control-plane.
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
