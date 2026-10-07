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

package v1beta2

const (
	// ResourceLifecycleOwned is the value we use when tagging resources to indicate
	// that the resource is considered owned and managed by the cluster,
	// and in particular that the lifecycle is tied to the lifecycle of the cluster.
	ResourceLifecycleOwned = ResourceLifecycle("owned")

	// NameHetznerProviderPrefix is the prefix of the tags that CAPH sets on HCloud resources.
	NameHetznerProviderPrefix = "caph-"
	// NameHetznerProviderOwned is the prefix of the tag key that marks an HCloud resource as owned by a
	// cluster. The cluster name follows the prefix.
	NameHetznerProviderOwned = NameHetznerProviderPrefix + "cluster-"

	// MachineNameTagKey is the tag key that has the name of the HCloudMachine of a server.
	MachineNameTagKey = "machine." + NameHetznerProviderPrefix + "name"
)
