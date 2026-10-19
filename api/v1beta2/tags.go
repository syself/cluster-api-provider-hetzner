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

	// ServerRecycleLabelKey marks an HCloud server as recyclable. When ServerRecycling is enabled for an
	// HCloudMachine, a server with this label (value "true") that matches the HCloudMachine's type and location
	// can be claimed and rebuilt instead of creating a new server. On deletion the server keeps this
	// label and is returned to the recyclable set instead of being deleted.
	ServerRecycleLabelKey = NameHetznerProviderPrefix + "recycle"

	// ServerRecycleAvailableLabelKey marks a recyclable server as not currently claimed by an HCloudMachine
	// (value "true"). It is removed when an HCloudMachine claims the server and re-added when the
	// HCloudMachine is deleted.
	ServerRecycleAvailableLabelKey = NameHetznerProviderPrefix + "recycle-available"

	// ServerRecycleClaimantLabelKey holds the name of the HCloudMachine that is currently claiming a
	// recyclable server (value = HCloudMachine name). It is set while a server is being reserved and rebuilt,
	// after the available marker is dropped but before the machine identity is applied. Because this key
	// is not the one the server lookup matches on, a reserved-but-not-yet-rebuilt server is never
	// adopted as a provisioned HCloudMachine, which keeps the claim idempotent if the rebuild fails.
	ServerRecycleClaimantLabelKey = NameHetznerProviderPrefix + "recycle-claimant"

	// ServerRecycleClaimedAtLabelKey holds the Unix timestamp (seconds) at which the claim recorded in
	// ServerRecycleClaimantLabelKey was written. A claim is only ever short-lived: it is verified on the
	// next reconcile and then either completed or released. If the controller dies in between, the
	// claim would otherwise pin the server out of the pool forever, because a claimed server carries no
	// available marker and therefore matches no lookup. This timestamp lets any other HCloudMachine recognise
	// such an abandoned claim and return the server to the pool.
	ServerRecycleClaimedAtLabelKey = NameHetznerProviderPrefix + "recycle-claimed-at"
)
