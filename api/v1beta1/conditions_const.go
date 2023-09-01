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

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// LoadBalancerAttached reports on whether the load balancer is attached.
	LoadBalancerAttached clusterv1.ConditionType = "LoadBalancerAttached"
	// LoadBalancerUnreachableReason is used when load balancer is unreachable.
	LoadBalancerUnreachableReason = "LoadBalancerUnreachable"
)

const (
	// LoadBalancerAttachedToNetworkCondition reports on whether the load balancer is attached to a network.
	LoadBalancerAttachedToNetworkCondition clusterv1.ConditionType = "LoadBalancerAttachedToNetwork"
	// LoadBalancerAttachFailedReason is used when load balancer could not be attached to network.
	LoadBalancerAttachFailedReason = "LoadBalancerAttachFailed"
	// LoadBalancerNoNetworkFoundReason is used when no network could be found.
	LoadBalancerNoNetworkFoundReason = "LoadBalancerNoNetworkFound"
)

const (
	// InstanceReadyCondition reports on current status of the instance. Ready indicates the instance is in a Running state.
	InstanceReadyCondition clusterv1.ConditionType = "InstanceReady"
	// InstanceTerminatedReason instance is in a terminated state.
	InstanceTerminatedReason = "InstanceTerminated"
	// InstanceHasNonExistingPlacementGroupReason instance has a placement group name that does not exist.
	InstanceHasNonExistingPlacementGroupReason = "InstanceHasNonExistingPlacementGroup"
	// ServerOffReason instance is off.
	ServerOffReason = "ServerOff"
	// InstanceAsControlPlaneUnreachableReason control plane is (not yet) reachable.
	InstanceAsControlPlaneUnreachableReason = "InstanceAsControlPlaneUnreachable"
)

const (
	// InstanceBootstrapReadyCondition reports on current status of the instance. BootstrapReady indicates the bootstrap is ready.
	InstanceBootstrapReadyCondition clusterv1.ConditionType = "InstanceBootstrapReady"
	// InstanceBootstrapNotReadyReason bootstrap not ready yet.
	InstanceBootstrapNotReadyReason = "InstanceBootstrapNotReady"
)

const (
	// NetworkAttached reports on whether there is a network attached to the cluster.
	NetworkAttached clusterv1.ConditionType = "NetworkAttached"
	// NetworkDisabledReason indicates that network is disabled.
	NetworkDisabledReason = "NetworkDisabled"
	// NetworkUnreachableReason indicates that network is unreachable.
	NetworkUnreachableReason = "NetworkUnreachable"
)

const (
	// PlacementGroupsSynced reports on whether the placement groups are successfully synced.
	PlacementGroupsSynced clusterv1.ConditionType = "PlacementGroupsSynced"
	// PlacementGroupsUnreachableReason indicates that network is disabled.
	PlacementGroupsUnreachableReason = "PlacementGroupsUnreachable"
)

const (
	// HetznerClusterReady reports on whether the Hetzner cluster is in ready state.
	HetznerClusterReady clusterv1.ConditionType = "HetznerClusterReady"
	// HetznerSecretUnreachableReason indicates that Hetzner secret is unreachable.
	HetznerSecretUnreachableReason = "HetznerSecretUnreachable" // #nosec
	// HCloudCredentialsInvalidReason indicates that credentials for HCloud are invalid.
	HCloudCredentialsInvalidReason = "HCloudCredentialsInvalid" // #nosec
)

const (
	// HetznerClusterTargetClusterReady reports on whether the kubeconfig in the target cluster is ready.
	HetznerClusterTargetClusterReady clusterv1.ConditionType = "HetznerClusterTargetClusterReady"
	// KubeConfigNotFound indicates that the Kubeconfig could not be found.
	KubeConfigNotFound = "KubeConfigNotFound"
	// KubeAPIServerNotResponding indicates that the api server cannot be reached.
	KubeAPIServerNotResponding = "KubeAPIServerNotResponding"
)

const (
	// HetznerAPIReachableCondition reports whether the Hetzner APIs are reachable.
	HetznerAPIReachableCondition clusterv1.ConditionType = "HetznerAPIReachable"
	// RateLimitExceededReason indicates that a rate limit has been exceeded.
	RateLimitExceededReason = "RateLimitExceeded"
)

const (
	// HetznerBareMetalHostReady reports on whether the Hetzner cluster is in ready state.
	HetznerBareMetalHostReady clusterv1.ConditionType = "HetznerBareMetalHostReady"
	// RobotCredentialsInvalidReason indicates that credentials for Robot are invalid.
	RobotCredentialsInvalidReason = "RobotCredentialsInvalid" // #nosec
	// SSHCredentialsInSecretInvalid indicates that ssh credentials are invalid.
	SSHCredentialsInSecretInvalid = "SSHCredentialsInSecretInvalid" // #nosec
	// SSHKeyAlreadyExists indicates that the ssh key which is specified in the host spec exists already under a different name in Hetzner robot.
	SSHKeyAlreadyExists = "SSHKeyAlreadyExists"
	// OSSSHSecretMissing indicates that secret with the os ssh key is missing.
	OSSSHSecretMissing = "OSSSHSecretMissing"
	// RescueSSHSecretMissing indicates that secret with the rescue ssh key is missing.
	RescueSSHSecretMissing = "RescueSSHSecretMissing"
)

const (
	// AssociateBMHCondition reports on whether the Hetzner cluster is in ready state.
	AssociateBMHCondition clusterv1.ConditionType = "AssociateBMHCondition"
)
