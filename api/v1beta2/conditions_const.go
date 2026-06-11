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

import clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

const (
	// LoadBalancerReadyV1Beta1Condition reports on whether a control plane load balancer was successfully reconciled.
	LoadBalancerReadyV1Beta1Condition clusterv1.ConditionType = "LoadBalancerReady"
	// LoadBalancerCreateFailedV1Beta1Reason used when an error occurs during load balancer create.
	LoadBalancerCreateFailedV1Beta1Reason = "LoadBalancerCreateFailed"
	// LoadBalancerUpdateFailedV1Beta1Reason used when an error occurs during load balancer update.
	LoadBalancerUpdateFailedV1Beta1Reason = "LoadBalancerUpdateFailed"
	// LoadBalancerDeleteFailedV1Beta1Reason used when an error occurs during load balancer delete.
	LoadBalancerDeleteFailedV1Beta1Reason = "LoadBalancerDeleteFailed"
	// LoadBalancerServiceSyncFailedV1Beta1Reason used when an error occurs while syncing services of load balancer.
	LoadBalancerServiceSyncFailedV1Beta1Reason = "LoadBalancerServiceSyncFailed"
	// LoadBalancerFailedToOwnV1Beta1Reason used when no owned label could be set on a load balancer.
	LoadBalancerFailedToOwnV1Beta1Reason = "LoadBalancerFailedToOwn"
)

const (
	// ServerCreateSucceededV1Beta1Condition reports on current status of the instance. Ready indicates the instance is in a Running state.
	ServerCreateSucceededV1Beta1Condition clusterv1.ConditionType = "ServerCreateSucceeded"
	// InstanceHasNonExistingPlacementGroupV1Beta1Reason instance has a placement group name that does not exist.
	InstanceHasNonExistingPlacementGroupV1Beta1Reason = "InstanceHasNonExistingPlacementGroup"
	// SSHKeyNotFoundV1Beta1Reason indicates that ssh key could not be found.
	SSHKeyNotFoundV1Beta1Reason = "SSHKeyNotFound"
	// ImageNotFoundV1Beta1Reason indicates that the image could not be found.
	ImageNotFoundV1Beta1Reason = "ImageNotFound"
	// ImageAmbiguousV1Beta1Reason indicates that there are multiple images with the required properties.
	ImageAmbiguousV1Beta1Reason = "ImageAmbiguous"
	// ServerTypeNotFoundV1Beta1Reason indicates that server type could not be found.
	ServerTypeNotFoundV1Beta1Reason = "ServerTypeNotFound"
	// ServerCreateFailedV1Beta1Reason indicates that server could not get created.
	ServerCreateFailedV1Beta1Reason = "ServerCreateFailedReason"
	// ServerCreateFailedIrrecoverableErrorV1Beta1Reason indicates that server creation failed with an irrecoverable error.
	ServerCreateFailedIrrecoverableErrorV1Beta1Reason = "ServerCreateFailedIrrecoverableError"
)

const (
	// ServerProvisionedV1Beta1Condition reports on whether the HCloud server has completed
	// boot-time provisioning (rescue boot, image install, OS startup).
	ServerProvisionedV1Beta1Condition clusterv1.ConditionType = "ServerProvisioned"
	// ServerOffV1Beta1Reason instance is off.
	ServerOffV1Beta1Reason = "ServerOff"
)

const (
	// ServerAvailableV1Beta1Condition indicates the instance is in a Running state.
	ServerAvailableV1Beta1Condition clusterv1.ConditionType = "ServerAvailable"
	// ServerTerminatingV1Beta1Reason instance is in a terminated state.
	ServerTerminatingV1Beta1Reason = "InstanceTerminated"
)

const (
	// SSHPrivateKeyAvailableV1Beta1Condition indicates that the SSH private key is available which is used to SSH into a server.
	SSHPrivateKeyAvailableV1Beta1Condition clusterv1.ConditionType = "SSHPrivateKeyAvailable"
	// SSHPrivateKeyNotFoundV1Beta1Reason indicates that the ssh private key could not be found.
	SSHPrivateKeyNotFoundV1Beta1Reason = "SSHPrivateKeyNotFound"
	// SSHPrivateKeySecretRefNotConfiguredV1Beta1Reason indicates that HetznerCluster.Spec.SSHKeys.RescueSecretRef.Name is empty.
	SSHPrivateKeySecretRefNotConfiguredV1Beta1Reason = "SSHPrivateKeySecretRefNotConfigured" //nolint:gosec
	// SSHPrivateKeySecretNotFoundV1Beta1Reason indicates that the referenced secret does not exist.
	SSHPrivateKeySecretNotFoundV1Beta1Reason = "SSHPrivateKeySecretNotFound" //nolint:gosec
	// SSHPrivateKeyFieldEmptyV1Beta1Reason indicates that the private key field referenced in the secret is missing or empty.
	SSHPrivateKeyFieldEmptyV1Beta1Reason = "SSHPrivateKeyFieldEmpty"
)

const (
	// NetworkAttachFailedV1Beta1Reason is used when server could not be attached to network.
	NetworkAttachFailedV1Beta1Reason = "NetworkAttachFailed"
	// LoadBalancerAttachFailedV1Beta1Reason is used when server could not be attached to network.
	LoadBalancerAttachFailedV1Beta1Reason = "LoadBalancerAttachFailed"
)

const (
	// BootstrapReadyV1Beta1Condition  indicates that bootstrap is ready.
	BootstrapReadyV1Beta1Condition clusterv1.ConditionType = "BootstrapReady"
	// BootstrapNotReadyV1Beta1Reason bootstrap not ready yet.
	BootstrapNotReadyV1Beta1Reason = "BootstrapNotReady"
)

const (
	// NetworkReadyV1Beta1Condition reports on whether the network is ready.
	NetworkReadyV1Beta1Condition clusterv1.ConditionType = "NetworkReady"
	// NetworkReconcileFailedV1Beta1Reason indicates that reconciling the network failed.
	NetworkReconcileFailedV1Beta1Reason = "NetworkReconcileFailed"
)

const (
	// PlacementGroupsSyncedV1Beta1Condition reports on whether the placement groups are successfully synced.
	PlacementGroupsSyncedV1Beta1Condition clusterv1.ConditionType = "PlacementGroupsSynced"
	// PlacementGroupsSyncFailedV1Beta1Reason indicates that syncing the placement groups failed.
	PlacementGroupsSyncFailedV1Beta1Reason = "PlacementGroupsSyncFailed"
)

const (
	// HCloudTokenAvailableV1Beta1Condition reports on whether the HCloud Token is available.
	HCloudTokenAvailableV1Beta1Condition clusterv1.ConditionType = "HCloudTokenAvailable"
	// HetznerSecretUnreachableV1Beta1Reason indicates that Hetzner secret is unreachable.
	HetznerSecretUnreachableV1Beta1Reason = "HetznerSecretUnreachable" // #nosec
	// HCloudCredentialsInvalidV1Beta1Reason indicates that credentials for HCloud are invalid.
	HCloudCredentialsInvalidV1Beta1Reason = "HCloudCredentialsInvalid" // #nosec
)

const (
	// HostReadyV1Beta1Condition reports on whether the HetznerBareMetalHost is ready or not. The hbmm
	// reconciler reads the Ready condition from the host (if the host exists),
	// and mirrors the Reason and Message on the HostReadyV1Beta1Condition of the hbmm.
	HostReadyV1Beta1Condition clusterv1.ConditionType = "HostReady"

	// HostNotFoundV1Beta1Reason indicates that the HetznerBaremetalHost associated with the HetznerBaremetalMachine
	// was not found.
	HostNotFoundV1Beta1Reason = "HostNotFound"
)

const (
	// RootDeviceHintsValidatedV1Beta1Condition reports on whether the root device hints could be validated.
	RootDeviceHintsValidatedV1Beta1Condition clusterv1.ConditionType = "RootDeviceHintsValidated"
	// ValidationFailedV1Beta1Reason indicates that the specified root device hints could not be successfully validated.
	ValidationFailedV1Beta1Reason = "ValidationFailed"
	// StorageDeviceNotFoundV1Beta1Reason indicates that the storage device specified in the root device hints could not be found.
	StorageDeviceNotFoundV1Beta1Reason = "StorageDeviceNotFound"
)

const (
	// TargetClusterReadyV1Beta1Condition reports on whether the kubeconfig in the target cluster is ready.
	TargetClusterReadyV1Beta1Condition clusterv1.ConditionType = "TargetClusterReady"
	// KubeConfigNotFoundV1Beta1Reason indicates that the Kubeconfig could not be found.
	KubeConfigNotFoundV1Beta1Reason = "KubeConfigNotFound"
	// KubeAPIServerNotRespondingV1Beta1Reason indicates that the api server cannot be reached.
	KubeAPIServerNotRespondingV1Beta1Reason = "KubeAPIServerNotResponding"
	// TargetClusterCreateFailedV1Beta1Reason indicates that the target cluster could not be created.
	TargetClusterCreateFailedV1Beta1Reason = "TargetClusterCreateFailed"
	// TargetClusterControlPlaneNotReadyV1Beta1Reason indicates that the target cluster's control plane is not ready yet.
	TargetClusterControlPlaneNotReadyV1Beta1Reason = "TargetClusterControlPlaneNotReady"
	// ControlPlaneEndpointSetV1Beta1Condition indicates that the control plane is set.
	ControlPlaneEndpointSetV1Beta1Condition = "ControlPlaneEndpointSet"
)

const (
	// TargetClusterSecretReadyV1Beta1Condition reports on whether the hetzner secret in the target cluster is ready.
	TargetClusterSecretReadyV1Beta1Condition clusterv1.ConditionType = "TargetClusterSecretReady"
	// TargetSecretSyncFailedV1Beta1Reason indicates that the target secret could not be synced.
	TargetSecretSyncFailedV1Beta1Reason = "TargetSecretSyncFailed"
	// ControlPlaneEndpointNotSetV1Beta1Reason indicates that the control plane endpoint is not set.
	ControlPlaneEndpointNotSetV1Beta1Reason = "ControlPlaneEndpointNotSet"
)

const (
	// HetznerAPIReachableV1Beta1Condition reports whether the Hetzner APIs are reachable.
	HetznerAPIReachableV1Beta1Condition clusterv1.ConditionType = "HetznerAPIReachable"
	// RateLimitExceededV1Beta1Reason indicates that a rate limit has been exceeded.
	RateLimitExceededV1Beta1Reason = "RateLimitExceeded"
)

const (
	// CredentialsAvailableV1Beta1Condition reports on whether the Hetzner cluster is in ready state.
	CredentialsAvailableV1Beta1Condition clusterv1.ConditionType = "CredentialsAvailable"
	// SSHCredentialsInSecretInvalidV1Beta1Reason indicates that ssh credentials are invalid.
	SSHCredentialsInSecretInvalidV1Beta1Reason = "SSHCredentialsInSecretInvalid" // #nosec
	// SSHKeyAlreadyExistsV1Beta1Reason indicates that the ssh key which is specified in the host spec exists already under a different name in Hetzner robot.
	SSHKeyAlreadyExistsV1Beta1Reason = "SSHKeyAlreadyExists"
	// OSSSHSecretMissingV1Beta1Reason indicates that secret with the os ssh key is missing.
	OSSSHSecretMissingV1Beta1Reason = "OSSSHSecretMissing"
	// RescueSSHSecretMissingV1Beta1Reason indicates that secret with the rescue ssh key is missing.
	RescueSSHSecretMissingV1Beta1Reason = "RescueSSHSecretMissing"
)

const (
	// RobotCredentialsAvailableV1Beta1Condition indicates that the robot credentials are available and valid.
	RobotCredentialsAvailableV1Beta1Condition clusterv1.ConditionType = "RobotCredentialsAvailable"
	// RobotCredentialsInvalidV1Beta1Reason indicates that credentials for Robot are invalid.
	RobotCredentialsInvalidV1Beta1Reason = "RobotCredentialsInvalid" // #nosec
)

const (
	// ProvisionSucceededV1Beta1Condition indicates that a host has been provisioned.
	ProvisionSucceededV1Beta1Condition clusterv1.ConditionType = "ProvisionSucceeded"
	// StillProvisioningV1Beta1Reason indicates that the server is still provisioning.
	StillProvisioningV1Beta1Reason = "StillProvisioning"
	// SSHConnectionRefusedV1Beta1Reason indicates that the server cannot be reached via SSH.
	SSHConnectionRefusedV1Beta1Reason = "SSHConnectionRefused"
	// RescueSystemUnavailableV1Beta1Reason indicates that the server has no rescue system.
	RescueSystemUnavailableV1Beta1Reason = "RescueSystemUnavailable"
	// ImageSpecInvalidV1Beta1Reason indicates that the information specified about the image of the host are invalid.
	ImageSpecInvalidV1Beta1Reason = "ImageSpecInvalid"
	// ImageDownloadFailedV1Beta1Reason indicates that downloading the machine image (http or OCI) failed.
	ImageDownloadFailedV1Beta1Reason = "ImageDownloadFailed"
	// NoStorageDeviceFoundV1Beta1Reason indicates that no suitable storage device could be found.
	NoStorageDeviceFoundV1Beta1Reason = "NoStorageDeviceFound"
	// CloudInitNotInstalledV1Beta1Reason indicates that cloud init is not installed.
	CloudInitNotInstalledV1Beta1Reason = "CloudInitNotInstalled"
	// ServerNotFoundV1Beta1Reason indicates that a bare metal server could not be found.
	ServerNotFoundV1Beta1Reason = "ServerNotFound"
	// ServerHasNoIPv4V1Beta1Reason indicates that a bare metal server has no IPv4 address assigned.
	ServerHasNoIPv4V1Beta1Reason = "ServerHasNoIPv4"
	// LinuxOnOtherDiskFoundV1Beta1Reason indicates that the server can't be provisioned on the given WWN, since the reboot would fail.
	LinuxOnOtherDiskFoundV1Beta1Reason = "LinuxOnOtherDiskFound"
	// WipeDiskFailedV1Beta1Reason indicates that erasing the disks before provisioning failed.
	WipeDiskFailedV1Beta1Reason = "WipeDiskFailed"
	// SSHToRescueSystemFailedV1Beta1Reason indicates that the rescue system can't be reached via ssh.
	SSHToRescueSystemFailedV1Beta1Reason = "SSHToRescueSystemFailed"
	// RebootTimedOutV1Beta1Reason indicates that the reboot timed out.
	RebootTimedOutV1Beta1Reason = "RebootTimedOut"
	// CheckDiskFailedV1Beta1Reason indicates that checking the health of the disk was not successful.
	CheckDiskFailedV1Beta1Reason = "CheckDiskFailed"
)

const (
	// HostAssociateSucceededV1Beta1Condition indicates that a host has been associated.
	HostAssociateSucceededV1Beta1Condition clusterv1.ConditionType = "HostAssociateSucceeded"
	// NoAvailableHostV1Beta1Reason indicates that there is no available host.
	NoAvailableHostV1Beta1Reason = "NoAvailableHost"
	// HostAssociateFailedV1Beta1Reason indicates that asssociating a host failed.
	HostAssociateFailedV1Beta1Reason = "HostAssociateFailed"
)

const (
	// DeletionInProgressV1Beta1Reason indicates that a host is being deleted.
	DeletionInProgressV1Beta1Reason = "DeletionInProgress"
)

// deprecated conditions.

const (
	// DeprecatedHostProvisionSucceededV1Beta1Condition indicates that a host has been provisioned.
	DeprecatedHostProvisionSucceededV1Beta1Condition clusterv1.ConditionType = "HostProvisionSucceeded"

	// DeprecatedInstanceReadyV1Beta1Condition reports on current status of the instance. Ready indicates the instance is in a Running state.
	DeprecatedInstanceReadyV1Beta1Condition clusterv1.ConditionType = "InstanceReady"

	// DeprecatedInstanceBootstrapReadyV1Beta1Condition reports on current status of the instance. BootstrapReady indicates the bootstrap is ready.
	DeprecatedInstanceBootstrapReadyV1Beta1Condition clusterv1.ConditionType = "InstanceBootstrapReady"

	// DeprecatedHetznerClusterTargetClusterReadyV1Beta1Condition reports on whether the kubeconfig in the target cluster is ready.
	DeprecatedHetznerClusterTargetClusterReadyV1Beta1Condition clusterv1.ConditionType = "HetznerClusterTargetClusterReady"

	// DeprecatedNetworkAttachedV1Beta1Condition reports on whether there is a network attached to the cluster.
	DeprecatedNetworkAttachedV1Beta1Condition clusterv1.ConditionType = "NetworkAttached"

	// DeprecatedLoadBalancerAttachedToNetworkV1Beta1Condition reports on whether the load balancer is attached to a network.
	DeprecatedLoadBalancerAttachedToNetworkV1Beta1Condition clusterv1.ConditionType = "LoadBalancerAttachedToNetwork"

	// DeprecatedHetznerBareMetalHostReadyV1Beta1Condition reports on whether the Hetzner cluster is in ready state.
	DeprecatedHetznerBareMetalHostReadyV1Beta1Condition clusterv1.ConditionType = "HetznerBareMetalHostReady"

	// DeprecatedAssociateBMHV1Beta1Condition reports on whether the Hetzner cluster is in ready state.
	DeprecatedAssociateBMHV1Beta1Condition clusterv1.ConditionType = "AssociateBMHCondition"

	// DeprecatedRateLimitExceededV1Beta1Condition reports whether the rate limit has been reached.
	DeprecatedRateLimitExceededV1Beta1Condition clusterv1.ConditionType = "RateLimitExceeded"
)

const (
	// RebootSucceededV1Beta1Condition indicates that the machine got rebooted successfully.
	RebootSucceededV1Beta1Condition clusterv1.ConditionType = "RebootSucceeded"
)

const (
	// RemediationSkippedV1Beta1Condition reports that remediation was skipped because
	// the HCloudMachine has a state that makes remediation unnecessary or impossible.
	RemediationSkippedV1Beta1Condition clusterv1.ConditionType = "RemediationSkipped"
	// IrrecoverableServerCreateFailureV1Beta1Reason indicates remediation was skipped because
	// the HCloudMachine failed to create with an irrecoverable error (e.g. invalid_input, resource_unavailable).
	IrrecoverableServerCreateFailureV1Beta1Reason = "IrrecoverableServerCreateFailure"
	// RemediationCooldownTriggeredV1Beta1Reason indicates that the machine became unhealthy
	// again within the cooldown window following a prior remediation. Rather than
	// rebooting again, the controller sets MachineOwnerRemediated to False so CAPI
	// escalates by deleting the machine.
	RemediationCooldownTriggeredV1Beta1Reason = "RemediationCooldownTriggered"
)

const (
	// NodeBootIDRetrievedV1Beta1Condition reports whether the boot ID of the node was retrieved.
	NodeBootIDRetrievedV1Beta1Condition clusterv1.ConditionType = "NodeBootIDRetrieved"
	// GetWorkloadClusterClientFailedV1Beta1Reason indicates failure in initializing the workload cluster client.
	GetWorkloadClusterClientFailedV1Beta1Reason = "GetWorkloadClusterClientFailed"
	// GetNodeInWorkloadClusterFailedV1Beta1Reason indicates failure in fetching the node object from the workload cluster.
	GetNodeInWorkloadClusterFailedV1Beta1Reason = "GetNodeInWorkloadClusterFailed"
	// BootIDEmptyV1Beta1Reason indicates that an empty boot ID is present on the node object.
	BootIDEmptyV1Beta1Reason = "BootIDEmpty"
)

// v1beta2 conditions.

// common conditions used across resource types.

const (
	// HCloudRateLimitExceededCondition reports on whether the HCloud API rate limit has been exceeded.
	HCloudRateLimitExceededCondition = "HCloudRateLimitExceeded"
	// HCloudRateLimitExceededReason indicates that the HCloud API rate limit has been exceeded.
	HCloudRateLimitExceededReason = "Exceeded"
)

const (
	// HCloudTokenAvailableCondition reports on whether the HCloud Token is available.
	HCloudTokenAvailableCondition = "HCloudTokenAvailable"
	// HCloudTokenAvailableReason indicates that the HCloudToken is available.
	HCloudTokenAvailableReason = clusterv1.AvailableReason
	// HCloudTokenInvalidReason indicates that the HCloudToken is invalid.
	HCloudTokenInvalidReason = "Invalid"
	// HCloudTokenSecretUnreachableReason indicates that secret containing the HCloudToken is unreachable.
	HCloudTokenSecretUnreachableReason = "SecretUnreachable" // #nosec
)

// HetznerCluster's v1beta2 conditions.

const (
	// HetznerClusterNetworkReadyCondition reports on whether the network is ready.
	HetznerClusterNetworkReadyCondition = "NetworkReady"
	// HetznerClusterNetworkReadyReason indicates that the network is ready.
	HetznerClusterNetworkReadyReason = clusterv1.ReadyReason
	// HetznerClusterNetworkReconcilingFailedReason indicates that reconciling the network failed.
	HetznerClusterNetworkReconcilingFailedReason = "ReconcilingFailed"
)

const (
	// HetznerClusterLoadBalancerReadyCondition reports on whether a control plane load balancer was successfully reconciled.
	HetznerClusterLoadBalancerReadyCondition = "LoadBalancerReady"
	// HetznerClusterLoadBalancerReadyReason indicates that a control plane load balancer is ready.
	HetznerClusterLoadBalancerReadyReason = clusterv1.ReadyReason
	// HetznerClusterLoadBalancerCreationFailedReason indicates that load balancer creation failed.
	HetznerClusterLoadBalancerCreationFailedReason = "CreationFailed"
	// HetznerClusterLoadBalancerMissingControlPlaneEndpointReason indicates that the control plane endpoint is not set.
	HetznerClusterLoadBalancerMissingControlPlaneEndpointReason = "MissingControlPlaneEndpoint"
	// HetznerClusterLoadBalancerSyncingServicesFailedReason indicates that an error occurred while syncing services of the load balancer.
	HetznerClusterLoadBalancerSyncingServicesFailedReason = "SyncingServicesFailed"
	// HetznerClusterLoadBalancerAttachingToNetworkFailedReason indicates that the server could not be attached to network.
	HetznerClusterLoadBalancerAttachingToNetworkFailedReason = "AttachingToNetworkFailed"
	// HetznerClusterLoadBalancerOwningFailedReason indicates no owned label could be set on a load balancer.
	HetznerClusterLoadBalancerOwningFailedReason = "OwningFailed"
	// HetznerClusterLoadBalancerUpdateFailedReason indicates that an error occurred during load balancer update.
	HetznerClusterLoadBalancerUpdateFailedReason = "UpdateFailed"
	// HetznerClusterLoadBalancerDeletionFailedReason indicates that an error occurred during load balancer delete.
	HetznerClusterLoadBalancerDeletionFailedReason = "DeletionFailed"
)

const (
	// HetznerClusterPlacementGroupsSyncedCondition reports on whether the placement groups are successfully synced.
	HetznerClusterPlacementGroupsSyncedCondition = "PlacementGroupsSynced"
	// HetznerClusterPlacementGroupsSyncingFailedReason indicates that syncing the placement groups failed.
	HetznerClusterPlacementGroupsSyncingFailedReason = "SyncingFailed"
	// HetznerClusterPlacementGroupsSyncedReason indicates that placement groups are synced successfully.
	HetznerClusterPlacementGroupsSyncedReason = "Synced"
)

const (
	// HetznerClusterControlPlaneEndpointSetCondition reports on whether the control plane endpoint is set.
	HetznerClusterControlPlaneEndpointSetCondition = "ControlPlaneEndpointSet"
	// HetznerClusterControlPlaneEndpointSetReason indicates that the control plane endpoint is set.
	HetznerClusterControlPlaneEndpointSetReason = "Set"
	// HetznerClusterControlPlaneEndpointNotSetReason indicates that the control plane endpoint is not set.
	HetznerClusterControlPlaneEndpointNotSetReason = "NotSet"
)

const (
	// HetznerClusterTargetClusterReadyCondition reports on whether the kubeconfig in the target cluster is ready.
	HetznerClusterTargetClusterReadyCondition = "TargetClusterReady"
	// HetznerClusterTargetClusterReadyReason indicates that the kubeconfig in the target cluster is ready.
	HetznerClusterTargetClusterReadyReason = clusterv1.ReadyReason
	// HetznerClusterTargetClusterCreationFailedReason indicates that the target cluster could not be created.
	HetznerClusterTargetClusterCreationFailedReason = "CreationFailed"
)

const (
	// HetznerClusterTargetClusterSecretReadyCondition reports on whether the hetzner secret in the target cluster is ready.
	HetznerClusterTargetClusterSecretReadyCondition = "TargetClusterSecretReady"
	// HetznerClusterTargetClusterSecretReadyReason indicates that the hetzner secret in the target cluster is ready.
	HetznerClusterTargetClusterSecretReadyReason = clusterv1.ReadyReason
	// HetznerClusterTargetClusterControlPlaneNotReadyReason indicates that the target cluster's control plane is not ready yet.
	HetznerClusterTargetClusterControlPlaneNotReadyReason = "ControlPlaneNotReady"
	// HetznerClusterTargetClusterSyncingSecretFailedReason indicates that the secret could not be synced.
	HetznerClusterTargetClusterSyncingSecretFailedReason = "SyncingSecretFailed"
)

const (
	// HetznerClusterDeletingCondition surfaces details about ongoing deletion of the HetznerCluster.
	HetznerClusterDeletingCondition = clusterv1.DeletingCondition

	// HetznerClusterDeletingReason surfaces when the HetznerCluster is being deleted.
	HetznerClusterDeletingReason = clusterv1.DeletingReason
)

// HetznerBareMetalHost's v1beta2 conditions.

const (
	// HetznerBareMetalHostSSHKeysAvailableCondition reports whether SSH keys for the host are available.
	HetznerBareMetalHostSSHKeysAvailableCondition = "SSHKeysAvailable"
	// HetznerBareMetalHostSSHKeysAvailableReason indicates SSH keys are available.
	HetznerBareMetalHostSSHKeysAvailableReason = clusterv1.AvailableReason
	// HetznerBareMetalHostSSHKeysInvalidReason indicates SSH keys in the secret are invalid.
	HetznerBareMetalHostSSHKeysInvalidReason = "Invalid"
	// HetznerBareMetalHostSSHKeyAlreadyExistsReason indicates the SSH key already exists under a different name in Hetzner Robot.
	HetznerBareMetalHostSSHKeyAlreadyExistsReason = "AlreadyExists"
	// HetznerBareMetalHostOSSSHSecretMissingReason indicates the OS SSH secret is missing.
	HetznerBareMetalHostOSSSHSecretMissingReason = "OSSSHSecretMissing"
	// HetznerBareMetalHostRescueSSHSecretMissingReason indicates the rescue SSH secret is missing.
	HetznerBareMetalHostRescueSSHSecretMissingReason = "RescueSSHSecretMissing"
)

const (
	// HetznerBareMetalHostRobotCredentialsAvailableCondition reports whether Robot API credentials are valid and reachable.
	HetznerBareMetalHostRobotCredentialsAvailableCondition = "RobotCredentialsAvailable"
	// HetznerBareMetalHostRobotCredentialsAvailableReason indicates the Robot credentials are available.
	HetznerBareMetalHostRobotCredentialsAvailableReason = clusterv1.AvailableReason
	// HetznerBareMetalHostRobotCredentialsInvalidReason indicates Robot credentials are invalid.
	HetznerBareMetalHostRobotCredentialsInvalidReason = "Invalid" // #nosec
	// HetznerBareMetalHostSecretUnreachableReason indicates the secret holding the Robot credentials is unreachable.
	HetznerBareMetalHostSecretUnreachableReason = "SecretUnreachable" // #nosec
)

const (
	// HetznerBareMetalHostRootDeviceHintsValidatedCondition reports whether the root device hints could be validated.
	HetznerBareMetalHostRootDeviceHintsValidatedCondition = "RootDeviceHintsValidated"
	// HetznerBareMetalHostRootDeviceHintsValidatedReason indicates root device hints are validated.
	HetznerBareMetalHostRootDeviceHintsValidatedReason = "Validated"
	// HetznerBareMetalHostValidationFailedReason indicates the specified root device hints could not be validated.
	HetznerBareMetalHostValidationFailedReason = "ValidationFailed"
)

const (
	// HetznerBareMetalHostProvisionSucceededCondition reports whether the host has been provisioned.
	HetznerBareMetalHostProvisionSucceededCondition = "ProvisionSucceeded"
	// HetznerBareMetalHostProvisionSucceededReason indicates the host has been provisioned.
	HetznerBareMetalHostProvisionSucceededReason = clusterv1.ProvisionedReason
	// HetznerBareMetalHostProvisioningReason indicates the server is provisioning.
	HetznerBareMetalHostProvisioningReason = "Provisioning"
	// HetznerBareMetalHostSSHConnectionRefusedReason indicates the server cannot be reached via SSH.
	HetznerBareMetalHostSSHConnectionRefusedReason = "SSHConnectionRefused"
	// HetznerBareMetalHostImageSpecInvalidReason indicates the image specification is invalid.
	HetznerBareMetalHostImageSpecInvalidReason = "ImageSpecInvalid"
	// HetznerBareMetalHostDownloadingImageFailedReason indicates downloading the machine image failed.
	HetznerBareMetalHostDownloadingImageFailedReason = "DownloadingImageFailed"
	// HetznerBareMetalHostNoStorageDeviceFoundReason indicates no suitable storage device could be found.
	HetznerBareMetalHostNoStorageDeviceFoundReason = "NoStorageDeviceFound"
	// HetznerBareMetalHostServerNotFoundReason indicates a bare metal server could not be found.
	HetznerBareMetalHostServerNotFoundReason = "ServerNotFound"
	// HetznerBareMetalHostServerHasNoIPv4Reason indicates a bare metal server has no IPv4 address.
	HetznerBareMetalHostServerHasNoIPv4Reason = "ServerHasNoIPv4"
	// HetznerBareMetalHostLinuxOnOtherDiskFoundReason indicates the server cannot be provisioned on the given WWN.
	HetznerBareMetalHostLinuxOnOtherDiskFoundReason = "LinuxOnOtherDiskFound"
	// HetznerBareMetalHostWipingDiskFailedReason indicates erasing the disks before provisioning failed.
	HetznerBareMetalHostWipingDiskFailedReason = "WipingDiskFailed"
	// HetznerBareMetalHostSSHToRescueSystemFailedReason indicates the rescue system cannot be reached via SSH.
	HetznerBareMetalHostSSHToRescueSystemFailedReason = "SSHToRescueSystemFailed"
	// HetznerBareMetalHostRebootTimeoutReachedReason indicates the reboot timeout was reached.
	HetznerBareMetalHostRebootTimeoutReachedReason = "RebootTimeoutReached"
	// HetznerBareMetalHostCheckingDiskFailedReason indicates checking the health of the disk was not successful.
	HetznerBareMetalHostCheckingDiskFailedReason = "CheckingDiskFailed"
)

const (
	// HetznerBareMetalHostDeletingCondition reports on whether the HetznerBareMetalHost is being deleted (negative polarity).
	HetznerBareMetalHostDeletingCondition = clusterv1.DeletingCondition
	// HetznerBareMetalHostDeletingReason indicates the HetznerBareMetalHost is being deleted.
	HetznerBareMetalHostDeletingReason = clusterv1.DeletingReason
)

const (
	// HetznerBareMetalHostNodeBootIDRetrievedCondition reports whether the boot ID of the node was retrieved.
	HetznerBareMetalHostNodeBootIDRetrievedCondition = "NodeBootIDRetrieved"
	// HetznerBareMetalHostNodeBootIDRetrievedReason indicates the boot ID was retrieved from the node.
	HetznerBareMetalHostNodeBootIDRetrievedReason = "Retrieved"
	// HetznerBareMetalHostGettingWorkloadClusterClientFailedReason indicates initializing the workload cluster client failed.
	HetznerBareMetalHostGettingWorkloadClusterClientFailedReason = "GettingWorkloadClusterClientFailed"
	// HetznerBareMetalHostGettingNodeInWorkloadClusterFailedReason indicates fetching the node object from the workload cluster failed.
	HetznerBareMetalHostGettingNodeInWorkloadClusterFailedReason = "GettingNodeInWorkloadClusterFailed"
	// HetznerBareMetalHostBootIDEmptyReason indicates the boot ID on the node object is empty.
	HetznerBareMetalHostBootIDEmptyReason = "BootIDEmpty"
)

const (
	// HetznerBareMetalHostRebootSucceededCondition reports whether the most recent reboot of the host succeeded.
	HetznerBareMetalHostRebootSucceededCondition = "RebootSucceeded"
	// HetznerBareMetalHostRebootSucceededReason indicates the most recent reboot succeeded.
	HetznerBareMetalHostRebootSucceededReason = "Succeeded"
	// HetznerBareMetalHostRebootingReason indicates the host is rebooting.
	HetznerBareMetalHostRebootingReason = "Rebooting"
	// HetznerBareMetalHostRebootSucceededTimeoutReachedOutReason indicates the reboot did not complete within the timeout.
	HetznerBareMetalHostRebootSucceededTimeoutReachedOutReason = "TimeoutReached"
	// HetznerBareMetalHostRebootingViaSSHFailedReason indicates triggering the reboot via SSH failed.
	HetznerBareMetalHostRebootingViaSSHFailedReason = "RebootingViaSSHFailed"
	// HetznerBareMetalHostRebootingBMServerViaAPIFailedReason indicates triggering the reboot via the Robot API failed.
	HetznerBareMetalHostRebootingBMServerViaAPIFailedReason = "RebootingBMServerViaAPIFailed"
)

const (
	// HetznerBareMetalHostRobotRateLimitExceededCondition reports whether the Robot API rate limit has been exceeded (negative polarity).
	HetznerBareMetalHostRobotRateLimitExceededCondition = "RobotRateLimitExceeded"
	// HetznerBareMetalHostRobotRateLimitExceededReason indicates the Robot API rate limit has been exceeded.
	HetznerBareMetalHostRobotRateLimitExceededReason = "Exceeded"
)

const (
	// HetznerBareMetalHostActionCompletedCondition surfaces the host's current provisioning or operational
	// action. It is present only while an action is in progress or the host is stuck (carrying the reason and
	// message for that state) and is removed once the action clears; it has no steady-state True.
	HetznerBareMetalHostActionCompletedCondition = "ActionCompleted"
)

// HetznerBareMetalMachine's v1beta2 conditions.
const (
	// HetznerBareMetalMachineHostAssociatedCondition is true when the host is associated.
	HetznerBareMetalMachineHostAssociatedCondition = "HostAssociated"

	// HetznerBareMetalMachineHostAssociatedReason surfaces when the host is associated.
	HetznerBareMetalMachineHostAssociatedReason = "Associated"

	// HetznerBareMetalMachineNoAvailableHostReason surfaces when no available host is found.
	HetznerBareMetalMachineNoAvailableHostReason = "NoAvailableHost"

	// HetznerBareMetalMachineHostAssociationFailedReason surfaces when host association failed.
	HetznerBareMetalMachineHostAssociationFailedReason = "AssociationFailed"

	// HetznerBareMetalMachineWaitingForBootstrapDataReason surfaces when waiting for bootstrap data.
	HetznerBareMetalMachineWaitingForBootstrapDataReason = clusterv1.WaitingForBootstrapDataReason
)

const (
	// HetznerBareMetalMachineDeletingCondition surfaces details about ongoing deletion of the HetznerBareMetalMachine.
	HetznerBareMetalMachineDeletingCondition = clusterv1.DeletingCondition

	// HetznerBareMetalMachineDeletingReason surfaces when the HetznerBareMetalMachine is being deleted.
	HetznerBareMetalMachineDeletingReason = clusterv1.DeletingReason
)

const (
	// HetznerBareMetalMachineHostReadyCondition is true when the associated host is ready.
	HetznerBareMetalMachineHostReadyCondition = "HostReady"

	// HetznerBareMetalMachineHostReadyReason surfaces when the host is ready.
	HetznerBareMetalMachineHostReadyReason = clusterv1.ReadyReason

	// HetznerBareMetalMachineNotFoundReason surfaces when the host is not found.
	HetznerBareMetalMachineNotFoundReason = "NotFound"

	// HetznerBareMetalMachineHostNotReadyReason surfaces when the host is not ready.
	HetznerBareMetalMachineHostNotReadyReason = "NotReady"
)


const (
	// NodeProvisioningSucceededCondition reports the result of the image-url-command.
	// True on success; False on permanent failure; Unknown while in progress.
	NodeProvisioningSucceededCondition clusterv1.ConditionType = "NodeProvisioningSucceeded"
	// NodeProvisioningSucceededReason indicates the image-url-command completed successfully.
	NodeProvisioningSucceededReason = "Succeeded"
	// NodeProvisioningFailedReason indicates the image-url-command failed permanently.
	NodeProvisioningFailedReason = "Failed"
	// NodeProvisioningInProgressReason indicates the image-url-command has not yet completed.
	NodeProvisioningInProgressReason = "InProgress"
)

const (
	// NodeProvisioningSucceededV1Beta1Condition is the legacy v1beta1 condition kept for migration.
	NodeProvisioningSucceededV1Beta1Condition clusterv1.ConditionType = "NodeProvisioningSucceeded"
)
