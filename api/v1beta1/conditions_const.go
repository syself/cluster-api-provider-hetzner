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
	// LoadBalancerReadyCondition reports on whether a control plane load balancer was successfully reconciled.
	LoadBalancerReadyCondition clusterv1.ConditionType = "LoadBalancerReady"
	// LoadBalancerCreateFailedReason used when an error occurs during load balancer create.
	LoadBalancerCreateFailedReason = "LoadBalancerCreateFailed"
	// LoadBalancerUpdateFailedReason used when an error occurs during load balancer update.
	LoadBalancerUpdateFailedReason = "LoadBalancerUpdateFailed"
	// LoadBalancerDeleteFailedReason used when an error occurs during load balancer delete.
	LoadBalancerDeleteFailedReason = "LoadBalancerDeleteFailed"
	// LoadBalancerServiceSyncFailedReason used when an error occurs while syncing services of load balancer.
	LoadBalancerServiceSyncFailedReason = "LoadBalancerServiceSyncFailed"
	// LoadBalancerFailedToOwnReason used when no owned label could be set on a load balancer.
	LoadBalancerFailedToOwnReason = "LoadBalancerFailedToOwn"
)

const (
	// ServerCreateSucceededCondition reports on current status of the instance. Ready indicates the instance is in a Running state.
	ServerCreateSucceededCondition clusterv1.ConditionType = "ServerCreateSucceeded"
	// InstanceHasNonExistingPlacementGroupReason instance has a placement group name that does not exist.
	InstanceHasNonExistingPlacementGroupReason = "InstanceHasNonExistingPlacementGroup"
	// SSHKeyNotFoundReason indicates that ssh key could not be found.
	SSHKeyNotFoundReason = "SSHKeyNotFound"
	// ImageNotFoundReason indicates that the image could not be found.
	ImageNotFoundReason = "ImageNotFound"
	// ImageAmbiguousReason indicates that there are multiple images with the required properties.
	ImageAmbiguousReason = "ImageAmbiguous"
	// ServerLimitExceededReason indicates that server resource rate limit is hit.
	ServerLimitExceededReason = "ServerLimitExceeded"
	// ServerTypeNotFoundReason indicates that server type could not be found.
	ServerTypeNotFoundReason = "ServerTypeNotFound"
)

const (
	// ServerAvailableCondition indicates the instance is in a Running state.
	ServerAvailableCondition clusterv1.ConditionType = "ServerAvailable"
	// ServerTerminatingReason instance is in a terminated state.
	ServerTerminatingReason = "InstanceTerminated"
	// ServerStartingReason instance is in a terminated state.
	ServerStartingReason = "ServerStarting"
	// ServerOffReason instance is off.
	ServerOffReason = "ServerOff"
)

const (
	// NetworkAttachFailedReason is used when server could not be attached to network.
	NetworkAttachFailedReason = "NetworkAttachFailed"
	// LoadBalancerAttachFailedReason is used when server could not be attached to network.
	LoadBalancerAttachFailedReason = "LoadBalancerAttachFailed"
)

const (
	// BootstrapReadyCondition  indicates that bootstrap is ready.
	BootstrapReadyCondition clusterv1.ConditionType = "BootstrapReady"
	// BootstrapNotReadyReason bootstrap not ready yet.
	BootstrapNotReadyReason = "BootstrapNotReady"
)

const (
	// NetworkReadyCondition reports on whether the network is ready.
	NetworkReadyCondition clusterv1.ConditionType = "NetworkReady"
	// NetworkReconcileFailedReason indicates that reconciling the network failed.
	NetworkReconcileFailedReason = "NetworkReconcileFailed"
)

const (
	// PlacementGroupsSyncedCondition reports on whether the placement groups are successfully synced.
	PlacementGroupsSyncedCondition clusterv1.ConditionType = "PlacementGroupsSynced"
	// PlacementGroupsSyncFailedReason indicates that syncing the placement groups failed.
	PlacementGroupsSyncFailedReason = "PlacementGroupsSyncFailed"
)

const (
	// HCloudTokenAvailableCondition reports on whether the HCloud Token is available.
	HCloudTokenAvailableCondition clusterv1.ConditionType = "HCloudTokenAvailable"
	// HetznerSecretUnreachableReason indicates that Hetzner secret is unreachable.
	HetznerSecretUnreachableReason = "HetznerSecretUnreachable" // #nosec
	// HCloudCredentialsInvalidReason indicates that credentials for HCloud are invalid.
	HCloudCredentialsInvalidReason = "HCloudCredentialsInvalid" // #nosec
)

const (
	// HostReadyCondition reports on whether the HetznerBareMetalHost is ready or not.
	HostReadyCondition clusterv1.ConditionType = "HostReady"
)

const (
	// RootDeviceHintsValidatedCondition reports on whether the root device hints could be validated.
	RootDeviceHintsValidatedCondition clusterv1.ConditionType = "RootDeviceHintsValidated"
	// ValidationFailedReason indicates that the specified root device hints could not be successfully validated.
	ValidationFailedReason = "ValidationFailed"
	// StorageDeviceNotFoundReason indicates that the storage device specified in the root device hints could not be found.
	StorageDeviceNotFoundReason = "StorageDeviceNotFound"
)

const (
	// TargetClusterReadyCondition reports on whether the kubeconfig in the target cluster is ready.
	TargetClusterReadyCondition clusterv1.ConditionType = "TargetClusterReady"
	// KubeConfigNotFoundReason indicates that the Kubeconfig could not be found.
	KubeConfigNotFoundReason = "KubeConfigNotFound"
	// KubeAPIServerNotRespondingReason indicates that the api server cannot be reached.
	KubeAPIServerNotRespondingReason = "KubeAPIServerNotResponding"
	// TargetClusterCreateFailedReason indicates that the target cluster could not be created.
	TargetClusterCreateFailedReason = "TargetClusterCreateFailed"
	// TargetClusterControlPlaneNotReadyReason indicates that the target cluster's control plane is not ready yet.
	TargetClusterControlPlaneNotReadyReason = "TargetClusterControlPlaneNotReady"
	// ControlPlaneEndpointSetCondition indicates that the control plane is set.
	ControlPlaneEndpointSetCondition = "ControlPlaneEndpointSet"
)

const (
	// TargetClusterSecretReadyCondition reports on whether the hetzner secret in the target cluster is ready.
	TargetClusterSecretReadyCondition clusterv1.ConditionType = "TargetClusterSecretReady"
	// TargetSecretSyncFailedReason indicates that the target secret could not be synced.
	TargetSecretSyncFailedReason = "TargetSecretSyncFailed"
	// ControlPlaneEndpointNotSetReason indicates that the control plane endpoint is not set.
	ControlPlaneEndpointNotSetReason = "ControlPlaneEndpointNotSet"
)

const (
	// HetznerAPIReachableCondition reports whether the Hetzner APIs are reachable.
	HetznerAPIReachableCondition clusterv1.ConditionType = "HetznerAPIReachable"
	// RateLimitExceededReason indicates that a rate limit has been exceeded.
	RateLimitExceededReason = "RateLimitExceeded"
)

const (
	// CredentialsAvailableCondition reports on whether the Hetzner cluster is in ready state.
	CredentialsAvailableCondition clusterv1.ConditionType = "CredentialsAvailable"
	// RobotCredentialsInvalidReason indicates that credentials for Robot are invalid.
	RobotCredentialsInvalidReason = "RobotCredentialsInvalid" // #nosec
	// SSHCredentialsInSecretInvalidReason indicates that ssh credentials are invalid.
	SSHCredentialsInSecretInvalidReason = "SSHCredentialsInSecretInvalid" // #nosec
	// SSHKeyAlreadyExistsReason indicates that the ssh key which is specified in the host spec exists already under a different name in Hetzner robot.
	SSHKeyAlreadyExistsReason = "SSHKeyAlreadyExists"
	// OSSSHSecretMissingReason indicates that secret with the os ssh key is missing.
	OSSSHSecretMissingReason = "OSSSHSecretMissing"
	// RescueSSHSecretMissingReason indicates that secret with the rescue ssh key is missing.
	RescueSSHSecretMissingReason = "RescueSSHSecretMissing"
)

const (
	// ProvisionSucceededCondition indicates that a host has been provisioned.
	ProvisionSucceededCondition clusterv1.ConditionType = "ProvisionSucceeded"
	// StillProvisioningReason indicates that the server is still provisioning.
	StillProvisioningReason = "StillProvisioning"
	// SSHConnectionRefusedReason indicates that the server cannot be reached via SSH.
	SSHConnectionRefusedReason = "SSHConnectionRefused"
	// RescueSystemUnavailableReason indicates that the server has no rescue system.
	RescueSystemUnavailableReason = "RescueSystemUnavailable"
	// ImageSpecInvalidReason indicates that the information specified about the image of the host are invalid.
	ImageSpecInvalidReason = "ImageSpecInvalid"
	// ImageDownloadFailedReason indicates that downloading the machine image (http or OCI) failed.
	ImageDownloadFailedReason = "ImageDownloadFailed"
	// NoStorageDeviceFoundReason indicates that no suitable storage device could be found.
	NoStorageDeviceFoundReason = "NoStorageDeviceFound"
	// CloudInitNotInstalledReason indicates that cloud init is not installed.
	CloudInitNotInstalledReason = "CloudInitNotInstalled"
	// ServerNotFoundReason indicates that a bare metal server could not be found.
	ServerNotFoundReason = "ServerNotFound"
	// LinuxOnOtherDiskFoundReason indicates that the server can't be provisioned on the given WWN, since the reboot would fail.
	LinuxOnOtherDiskFoundReason = "LinuxOnOtherDiskFound"
	// SSHToRescueSystemFailedReason indicates that the rescue system can't be reached via ssh.
	SSHToRescueSystemFailedReason = "SSHToRescueSystemFailed"
)

const (
	// SSHAfterInstallImageSucceededCondition indicates that the host is reachable via ssh after installImage.
	SSHAfterInstallImageSucceededCondition clusterv1.ConditionType = "SSHAfterInstallImageSucceeded"

	// SSHAfterInstallImageFailedReason indicates that the host was not reachable via ssh.
	SSHAfterInstallImageFailedReason = "SSHAfterInstallImageFailed"
)

const (
	// HostAssociateSucceededCondition indicates that a host has been associated.
	HostAssociateSucceededCondition clusterv1.ConditionType = "HostAssociateSucceeded"
	// NoAvailableHostReason indicates that there is no available host.
	NoAvailableHostReason = "NoAvailableHost"
	// HostAssociateFailedReason indicates that asssociating a host failed.
	HostAssociateFailedReason = "HostAssociateFailed"
)

const (
	// DeletionInProgressReason indicates that a host is being deleted.
	DeletionInProgressReason = "DeletionInProgress"
)

// deprecated conditions.

const (
	// DeprecatedHostProvisionSucceededCondition indicates that a host has been provisioned.
	DeprecatedHostProvisionSucceededCondition clusterv1.ConditionType = "HostProvisionSucceeded"

	// DeprecatedInstanceReadyCondition reports on current status of the instance. Ready indicates the instance is in a Running state.
	DeprecatedInstanceReadyCondition clusterv1.ConditionType = "InstanceReady"

	// DeprecatedInstanceBootstrapReadyCondition reports on current status of the instance. BootstrapReady indicates the bootstrap is ready.
	DeprecatedInstanceBootstrapReadyCondition clusterv1.ConditionType = "InstanceBootstrapReady"

	// DeprecatedHetznerClusterTargetClusterReadyCondition reports on whether the kubeconfig in the target cluster is ready.
	DeprecatedHetznerClusterTargetClusterReadyCondition clusterv1.ConditionType = "HetznerClusterTargetClusterReady"

	// DeprecatedNetworkAttachedCondition reports on whether there is a network attached to the cluster.
	DeprecatedNetworkAttachedCondition clusterv1.ConditionType = "NetworkAttached"

	// DeprecatedLoadBalancerAttachedToNetworkCondition reports on whether the load balancer is attached to a network.
	DeprecatedLoadBalancerAttachedToNetworkCondition clusterv1.ConditionType = "LoadBalancerAttachedToNetwork"

	// DeprecatedHetznerBareMetalHostReadyCondition reports on whether the Hetzner cluster is in ready state.
	DeprecatedHetznerBareMetalHostReadyCondition clusterv1.ConditionType = "HetznerBareMetalHostReady"

	// DeprecatedAssociateBMHCondition reports on whether the Hetzner cluster is in ready state.
	DeprecatedAssociateBMHCondition clusterv1.ConditionType = "AssociateBMHCondition"

	// DeprecatedRateLimitExceededCondition reports whether the rate limit has been reached.
	DeprecatedRateLimitExceededCondition clusterv1.ConditionType = "RateLimitExceeded"
)
