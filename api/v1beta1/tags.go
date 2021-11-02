/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import "fmt"

const (
	// ResourceLifecycleOwned is the value we use when tagging resources to indicate
	// that the resource is considered owned and managed by the cluster,
	// and in particular that the lifecycle is tied to the lifecycle of the cluster.
	ResourceLifecycleOwned = ResourceLifecycle("owned")

	// ResourceLifecycleShared is the value we use when tagging resources to indicate
	// that the resource is shared between multiple clusters, and should not be destroyed
	// if the cluster is destroyed.
	ResourceLifecycleShared = ResourceLifecycle("shared")

	// NameKubernetesHetznerCloudProviderPrefix is the tag name used by the cloud provider to logically
	// separate independent cluster resources. We use it to identify which resources we expect
	// to be permissive about state changes.
	// logically independent clusters running in the same AZ.
	// The tag key = NameKubernetesHetznerCloudProviderPrefix + clusterID
	// The tag value is an ownership value.
	NameKubernetesHetznerCloudProviderPrefix = "caph"

	// NameHetznerProviderPrefix is the tag prefix we use to differentiate
	// cluster-api-provider-hetzner owned components from other tooling that
	// uses NameKubernetesClusterPrefix
	// NameHetznerProviderPrefix = "sigs.k8s.io/cluster-api-provider-hetzner/".
	NameHetznerProviderPrefix = "caph-"
	// NameHetznerProviderOwned is the tag name we use to differentiate
	// cluster-api-provider-hetzner owned components from other tooling that
	// uses NameKubernetesClusterPrefix.
	NameHetznerProviderOwned = NameHetznerProviderPrefix + "cluster-"

	// MachineNameTagKey tags related MachineNameTag.
	MachineNameTagKey = "machine." + NameHetznerProviderPrefix + "name"

	// MachineTemplateHashTagKey tags server resources.
	MachineTemplateHashTagKey = "machine." + NameHetznerProviderPrefix + "template"
)

// ClusterTagKey generates the key for resources associated with a cluster.
func ClusterTagKey(name string) string {
	return fmt.Sprintf("%s%s", NameHetznerProviderOwned, name)
}

// ClusterHetznerCloudProviderTagKey generates the key for resources associated a cluster's HCloud cloud provider.
func ClusterHetznerCloudProviderTagKey(name string) string {
	return fmt.Sprintf("%s%s", NameKubernetesHetznerCloudProviderPrefix, name)
}
