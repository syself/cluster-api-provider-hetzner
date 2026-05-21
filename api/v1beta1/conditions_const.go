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

import clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

const (
	// LoadBalancerReadyCondition reports on whether a control plane load balancer was successfully reconciled.
	LoadBalancerReadyCondition clusterv1beta1.ConditionType = "LoadBalancerReady"
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
	ServerCreateSucceededCondition clusterv1beta1.ConditionType = "ServerCreateSucceeded"
	// InstanceHasNonExistingPlacementGroupReason instance has a placement group name that does not exist.
	InstanceHasNonExistingPlacementGroupReason = "InstanceHasNonExistingPlacementGroup"
	// SSHKeyNotFoundReason indicates that ssh key could not be found.
	SSHKeyNotFoundReason = "SSHKeyNotFound"
	// ImageNotFoundReason indicates that the image could not be found.
	ImageNotFoundReason = "ImageNotFound"
	// ImageAmbiguousReason indicates that there are multiple images with the required properties.
	ImageAmbiguousReason = "ImageAmbiguous"
	// ServerTypeNotFoundReason indicates that server type could not be found.
	ServerTypeNotFoundReason = "ServerTypeNotFound"
	// ServerCreateFailedReason indicates that server could not get created.
	ServerCreateFailedReason = "ServerCreateFailedReason"
	// ServerCreateFailedIrrecoverableErrorReason indicates that server creation failed with an irrecoverable error.
	ServerCreateFailedIrrecoverableErrorReason = "ServerCreateFailedIrrecoverableError"
)

const (
	// ServerProvisionedCondition reports on whether the HCloud server has completed
	// boot-time provisioning (rescue boot, image install, OS startup).
	ServerProvisionedCondition clusterv1beta1.ConditionType = "ServerProvisioned"
	// ServerOffReason instance is off.
	ServerOffReason = "ServerOff"
)

const (
	// ServerAvailableCondition indicates the instance is in a Running state.
	ServerAvailableCondition clusterv1beta1.ConditionType = "ServerAvailable"
	// ServerTerminatingReason instance is in a terminated state.
	ServerTerminatingReason = "InstanceTerminated"
)

const (
	// SSHPrivateKeyAvailableCondition indicates that the SSH private key is available which is used to SSH into a server.
	SSHPrivateKeyAvailableCondition clusterv1beta1.ConditionType = "SSHPrivateKeyAvailable"
	// SSHPrivateKeyNotFoundReason indicates that the ssh private key could not be found.
	SSHPrivateKeyNotFoundReason = "SSHPrivateKeyNotFound"
	// SSHPrivateKeySecretRefNotConfiguredReason indicates that HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name is empty.
	SSHPrivateKeySecretRefNotConfiguredReason = "SSHPrivateKeySecretRefNotConfigured" //nolint:gosec
	// SSHPrivateKeySecretNotFoundReason indicates that the referenced secret does not exist.
	SSHPrivateKeySecretNotFoundReason = "SSHPrivateKeySecretNotFound" //nolint:gosec
	// SSHPrivateKeyFieldEmptyReason indicates that the private key field referenced in the secret is missing or empty.
	SSHPrivateKeyFieldEmptyReason = "SSHPrivateKeyFieldEmpty"
)

const (
	// NetworkAttachFailedReason is used when server could not be attached to network.
	NetworkAttachFailedReason = "NetworkAttachFailed"
	// LoadBalancerAttachFailedReason is used when server could not be attached to network.
	LoadBalancerAttachFailedReason = "LoadBalancerAttachFailed"
)

const (
	// BootstrapReadyCondition  indicates that bootstrap is ready.
	BootstrapReadyCondition clusterv1beta1.ConditionType = "BootstrapReady"
	// BootstrapNotReadyReason bootstrap not ready yet.
	BootstrapNotReadyReason = "BootstrapNotReady"
)

const (
	// NetworkReadyCondition reports on whether the network is ready.
	NetworkReadyCondition clusterv1beta1.ConditionType = "NetworkReady"
	// NetworkReconcileFailedReason indicates that reconciling the network failed.
	NetworkReconcileFailedReason = "NetworkReconcileFailed"
)

const (
	// PlacementGroupsSyncedCondition reports on whether the placement groups are successfully synced.
	PlacementGroupsSyncedCondition clusterv1beta1.ConditionType = "PlacementGroupsSynced"
	// PlacementGroupsSyncFailedReason indicates that syncing the placement groups failed.
	PlacementGroupsSyncFailedReason = "PlacementGroupsSyncFailed"
)

const (
	// HCloudTokenAvailableCondition reports on whether the HCloud Token is available.
	HCloudTokenAvailableCondition clusterv1beta1.ConditionType = "HCloudTokenAvailable"
	// HetznerSecretUnreachableReason indicates that Hetzner secret is unreachable.
	HetznerSecretUnreachableReason = "HetznerSecretUnreachable" // #nosec
	// HCloudCredentialsInvalidReason indicates that credentials for HCloud are invalid.
	HCloudCredentialsInvalidReason = "HCloudCredentialsInvalid" // #nosec
)

const (
	// HostReadyCondition reports on whether the HetznerBareMetalHost is ready or not. The hbmm
	// reconciler reads the clusterv1beta1.ReadyCondition condition from the host (if the host exists),
	// and mirrors the Reason and Message on the HostReadyCondition of the hbmm.
	HostReadyCondition clusterv1beta1.ConditionType = "HostReady"

	// HostNotFoundReason indicates that the HetznerBaremetalHost associated with the HetznerBaremetalMachine
	// was not found.
	HostNotFoundReason = "HostNotFound"
)

const (
	// RootDeviceHintsValidatedCondition reports on whether the root device hints could be validated.
	RootDeviceHintsValidatedCondition clusterv1beta1.ConditionType = "RootDeviceHintsValidated"
	// ValidationFailedReason indicates that the specified root device hints could not be successfully validated.
	ValidationFailedReason = "ValidationFailed"
	// StorageDeviceNotFoundReason indicates that the storage device specified in the root device hints could not be found.
	StorageDeviceNotFoundReason = "StorageDeviceNotFound"
)

const (
	// TargetClusterReadyCondition reports on whether the kubeconfig in the target cluster is ready.
	TargetClusterReadyCondition clusterv1beta1.ConditionType = "TargetClusterReady"
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
	TargetClusterSecretReadyCondition clusterv1beta1.ConditionType = "TargetClusterSecretReady"
	// TargetSecretSyncFailedReason indicates that the target secret could not be synced.
	TargetSecretSyncFailedReason = "TargetSecretSyncFailed"
	// ControlPlaneEndpointNotSetReason indicates that the control plane endpoint is not set.
	ControlPlaneEndpointNotSetReason = "ControlPlaneEndpointNotSet"
)

const (
	// HetznerAPIReachableCondition reports whether the Hetzner APIs are reachable.
	HetznerAPIReachableCondition clusterv1beta1.ConditionType = "HetznerAPIReachable"
	// RateLimitExceededReason indicates that a rate limit has been exceeded.
	RateLimitExceededReason = "RateLimitExceeded"
)

const (
	// CredentialsAvailableCondition reports on whether the Hetzner cluster is in ready state.
	CredentialsAvailableCondition clusterv1beta1.ConditionType = "CredentialsAvailable"
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
	// RobotCredentialsAvailableCondition indicates that the robot credentials are available and valid.
	RobotCredentialsAvailableCondition clusterv1beta1.ConditionType = "RobotCredentialsAvailable"
	// RobotCredentialsInvalidReason indicates that credentials for Robot are invalid.
	RobotCredentialsInvalidReason = "RobotCredentialsInvalid" // #nosec
)

const (
	// ProvisionSucceededCondition indicates that a host has been provisioned.
	ProvisionSucceededCondition clusterv1beta1.ConditionType = "ProvisionSucceeded"
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
	// ServerHasNoIPv4Reason indicates that a bare metal server has no IPv4 address assigned.
	ServerHasNoIPv4Reason = "ServerHasNoIPv4"
	// LinuxOnOtherDiskFoundReason indicates that the server can't be provisioned on the given WWN, since the reboot would fail.
	LinuxOnOtherDiskFoundReason = "LinuxOnOtherDiskFound"
	// WipeDiskFailedReason indicates that erasing the disks before provisioning failed.
	WipeDiskFailedReason = "WipeDiskFailed"
	// SSHToRescueSystemFailedReason indicates that the rescue system can't be reached via ssh.
	SSHToRescueSystemFailedReason = "SSHToRescueSystemFailed"
	// RebootTimedOutReason indicates that the reboot timed out.
	RebootTimedOutReason = "RebootTimedOut"
	// CheckDiskFailedReason indicates that checking the health of the disk was not successful.
	CheckDiskFailedReason = "CheckDiskFailed"
)

const (
	// HostAssociateSucceededCondition indicates that a host has been associated.
	HostAssociateSucceededCondition clusterv1beta1.ConditionType = "HostAssociateSucceeded"
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
	DeprecatedHostProvisionSucceededCondition clusterv1beta1.ConditionType = "HostProvisionSucceeded"

	// DeprecatedInstanceReadyCondition reports on current status of the instance. Ready indicates the instance is in a Running state.
	DeprecatedInstanceReadyCondition clusterv1beta1.ConditionType = "InstanceReady"

	// DeprecatedInstanceBootstrapReadyCondition reports on current status of the instance. BootstrapReady indicates the bootstrap is ready.
	DeprecatedInstanceBootstrapReadyCondition clusterv1beta1.ConditionType = "InstanceBootstrapReady"

	// DeprecatedHetznerClusterTargetClusterReadyCondition reports on whether the kubeconfig in the target cluster is ready.
	DeprecatedHetznerClusterTargetClusterReadyCondition clusterv1beta1.ConditionType = "HetznerClusterTargetClusterReady"

	// DeprecatedNetworkAttachedCondition reports on whether there is a network attached to the cluster.
	DeprecatedNetworkAttachedCondition clusterv1beta1.ConditionType = "NetworkAttached"

	// DeprecatedLoadBalancerAttachedToNetworkCondition reports on whether the load balancer is attached to a network.
	DeprecatedLoadBalancerAttachedToNetworkCondition clusterv1beta1.ConditionType = "LoadBalancerAttachedToNetwork"

	// DeprecatedHetznerBareMetalHostReadyCondition reports on whether the Hetzner cluster is in ready state.
	DeprecatedHetznerBareMetalHostReadyCondition clusterv1beta1.ConditionType = "HetznerBareMetalHostReady"

	// DeprecatedAssociateBMHCondition reports on whether the Hetzner cluster is in ready state.
	DeprecatedAssociateBMHCondition clusterv1beta1.ConditionType = "AssociateBMHCondition"

	// DeprecatedRateLimitExceededCondition reports whether the rate limit has been reached.
	DeprecatedRateLimitExceededCondition clusterv1beta1.ConditionType = "RateLimitExceeded"
)

const (
	// RebootSucceededCondition indicates that the machine got rebooted successfully.
	RebootSucceededCondition clusterv1beta1.ConditionType = "RebootSucceeded"
)

const (
	// RemediationSkippedCondition reports that remediation was skipped because
	// the HCloudMachine has a state that makes remediation unnecessary or impossible.
	RemediationSkippedCondition clusterv1beta1.ConditionType = "RemediationSkipped"
	// IrrecoverableServerCreateFailureReason indicates remediation was skipped because
	// the HCloudMachine failed to create with an irrecoverable error (e.g. invalid_input, resource_unavailable).
	IrrecoverableServerCreateFailureReason = "IrrecoverableServerCreateFailure"
	// RemediationCooldownTriggeredReason indicates that the machine became unhealthy
	// again within the cooldown window following a prior remediation. Rather than
	// rebooting again, the controller sets MachineOwnerRemediated to False so CAPI
	// escalates by deleting the machine.
	RemediationCooldownTriggeredReason = "RemediationCooldownTriggered"
)

const (
	// NodeBootIDRetrievedCondition reports whether the boot ID of the node was retrieved.
	NodeBootIDRetrievedCondition clusterv1beta1.ConditionType = "NodeBootIDRetrieved"
	// GetWorkloadClusterClientFailedReason indicates failure in initializing the workload cluster client.
	GetWorkloadClusterClientFailedReason = "GetWorkloadClusterClientFailed"
	// GetNodeInWorkloadClusterFailedReason indicates failure in fetching the node object from the workload cluster.
	GetNodeInWorkloadClusterFailedReason = "GetNodeInWorkloadClusterFailed"
	// BootIDEmptyReason indicates that an empty boot ID is present on the node object.
	BootIDEmptyReason = "BootIDEmpty"
)

// v1beta2 conditions for HetznerBareMetalHost.

// InternalErrorV1Beta2Reason is a generic reason for internal errors during summary computation
// or other unexpected failures that do not have a more specific reason.
const InternalErrorV1Beta2Reason = "InternalError"

const (
	// HetznerBareMetalHostReadyV1Beta2Condition is the summary condition for HetznerBareMetalHost.
	HetznerBareMetalHostReadyV1Beta2Condition = "Ready"
	// HetznerBareMetalHostReadyV1Beta2Reason indicates that the HetznerBareMetalHost is ready.
	HetznerBareMetalHostReadyV1Beta2Reason = "Ready"
	// HetznerBareMetalHostNotReadyV1Beta2Reason indicates that the HetznerBareMetalHost is not ready.
	HetznerBareMetalHostNotReadyV1Beta2Reason = "NotReady"
	// HetznerBareMetalHostReadyUnknownV1Beta2Reason indicates that readiness of the HetznerBareMetalHost is unknown.
	HetznerBareMetalHostReadyUnknownV1Beta2Reason = "ReadyUnknown"
)

const (
	// HetznerBareMetalHostCredentialsAvailableV1Beta2Condition reports whether SSH credentials for the host are available.
	HetznerBareMetalHostCredentialsAvailableV1Beta2Condition = "CredentialsAvailable"
	// HetznerBareMetalHostCredentialsAvailableV1Beta2Reason indicates SSH credentials are available.
	HetznerBareMetalHostCredentialsAvailableV1Beta2Reason = "Available"
	// HetznerBareMetalHostSSHCredentialsInSecretInvalidV1Beta2Reason indicates SSH credentials in the secret are invalid.
	HetznerBareMetalHostSSHCredentialsInSecretInvalidV1Beta2Reason = "SSHCredentialsInSecretInvalid" // #nosec
	// HetznerBareMetalHostSSHKeyAlreadyExistsV1Beta2Reason indicates the SSH key already exists under a different name in Hetzner Robot.
	HetznerBareMetalHostSSHKeyAlreadyExistsV1Beta2Reason = "SSHKeyAlreadyExists"
	// HetznerBareMetalHostOSSSHSecretMissingV1Beta2Reason indicates the OS SSH secret is missing.
	HetznerBareMetalHostOSSSHSecretMissingV1Beta2Reason = "OSSSHSecretMissing"
	// HetznerBareMetalHostRescueSSHSecretMissingV1Beta2Reason indicates the rescue SSH secret is missing.
	HetznerBareMetalHostRescueSSHSecretMissingV1Beta2Reason = "RescueSSHSecretMissing"
	// HetznerBareMetalHostHetznerSecretUnreachableV1Beta2Reason indicates the Hetzner secret is unreachable.
	HetznerBareMetalHostHetznerSecretUnreachableV1Beta2Reason = "HetznerSecretUnreachable" // #nosec
)

const (
	// HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition reports whether Robot API credentials are valid and reachable.
	HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition = "RobotCredentialsAvailable"
	// HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Reason indicates the Robot credentials are available.
	HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Reason = "Available"
	// HetznerBareMetalHostRobotCredentialsInvalidV1Beta2Reason indicates Robot credentials are invalid.
	HetznerBareMetalHostRobotCredentialsInvalidV1Beta2Reason = "RobotCredentialsInvalid" // #nosec
)

const (
	// HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Condition reports whether the root device hints could be validated.
	HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Condition = "RootDeviceHintsValidated"
	// HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Reason indicates root device hints are validated.
	HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Reason = "Validated"
	// HetznerBareMetalHostValidationFailedV1Beta2Reason indicates the specified root device hints could not be validated.
	HetznerBareMetalHostValidationFailedV1Beta2Reason = "ValidationFailed"
)

const (
	// HetznerBareMetalHostProvisionSucceededV1Beta2Condition reports whether the host has been provisioned.
	HetznerBareMetalHostProvisionSucceededV1Beta2Condition = "ProvisionSucceeded"
	// HetznerBareMetalHostProvisionSucceededV1Beta2Reason indicates the host has been provisioned.
	HetznerBareMetalHostProvisionSucceededV1Beta2Reason = "Provisioned"
	// HetznerBareMetalHostStillProvisioningV1Beta2Reason indicates the server is still provisioning.
	HetznerBareMetalHostStillProvisioningV1Beta2Reason = "StillProvisioning"
	// HetznerBareMetalHostSSHConnectionRefusedV1Beta2Reason indicates the server cannot be reached via SSH.
	HetznerBareMetalHostSSHConnectionRefusedV1Beta2Reason = "SSHConnectionRefused"
	// HetznerBareMetalHostRescueSystemUnavailableV1Beta2Reason indicates the server has no rescue system.
	HetznerBareMetalHostRescueSystemUnavailableV1Beta2Reason = "RescueSystemUnavailable"
	// HetznerBareMetalHostImageSpecInvalidV1Beta2Reason indicates the image specification is invalid.
	HetznerBareMetalHostImageSpecInvalidV1Beta2Reason = "ImageSpecInvalid"
	// HetznerBareMetalHostImageDownloadFailedV1Beta2Reason indicates downloading the machine image failed.
	HetznerBareMetalHostImageDownloadFailedV1Beta2Reason = "ImageDownloadFailed"
	// HetznerBareMetalHostNoStorageDeviceFoundV1Beta2Reason indicates no suitable storage device could be found.
	HetznerBareMetalHostNoStorageDeviceFoundV1Beta2Reason = "NoStorageDeviceFound"
	// HetznerBareMetalHostCloudInitNotInstalledV1Beta2Reason indicates cloud-init is not installed.
	HetznerBareMetalHostCloudInitNotInstalledV1Beta2Reason = "CloudInitNotInstalled"
	// HetznerBareMetalHostServerNotFoundV1Beta2Reason indicates a bare metal server could not be found.
	HetznerBareMetalHostServerNotFoundV1Beta2Reason = "ServerNotFound"
	// HetznerBareMetalHostServerHasNoIPv4V1Beta2Reason indicates a bare metal server has no IPv4 address.
	HetznerBareMetalHostServerHasNoIPv4V1Beta2Reason = "ServerHasNoIPv4"
	// HetznerBareMetalHostLinuxOnOtherDiskFoundV1Beta2Reason indicates the server cannot be provisioned on the given WWN.
	HetznerBareMetalHostLinuxOnOtherDiskFoundV1Beta2Reason = "LinuxOnOtherDiskFound"
	// HetznerBareMetalHostWipeDiskFailedV1Beta2Reason indicates erasing the disks before provisioning failed.
	HetznerBareMetalHostWipeDiskFailedV1Beta2Reason = "WipeDiskFailed"
	// HetznerBareMetalHostSSHToRescueSystemFailedV1Beta2Reason indicates the rescue system cannot be reached via SSH.
	HetznerBareMetalHostSSHToRescueSystemFailedV1Beta2Reason = "SSHToRescueSystemFailed"
	// HetznerBareMetalHostRebootTimedOutV1Beta2Reason indicates the reboot timed out.
	HetznerBareMetalHostRebootTimedOutV1Beta2Reason = "RebootTimedOut"
	// HetznerBareMetalHostCheckDiskFailedV1Beta2Reason indicates checking the health of the disk was not successful.
	HetznerBareMetalHostCheckDiskFailedV1Beta2Reason = "CheckDiskFailed"
)

const (
	// HetznerBareMetalHostDeletingV1Beta2Condition reports on whether the HetznerBareMetalHost is being deleted (negative polarity).
	HetznerBareMetalHostDeletingV1Beta2Condition = "Deleting"
	// HetznerBareMetalHostDeletingV1Beta2Reason indicates the HetznerBareMetalHost is being deleted.
	HetznerBareMetalHostDeletingV1Beta2Reason = "Deleting"
)

const (
	// HetznerBareMetalHostRobotRateLimitExceededV1Beta2Condition reports whether the Robot API rate limit has been exceeded (negative polarity).
	HetznerBareMetalHostRobotRateLimitExceededV1Beta2Condition = "RobotRateLimitExceeded"
	// HetznerBareMetalHostRobotRateLimitExceededV1Beta2Reason indicates the Robot API rate limit has been exceeded.
	HetznerBareMetalHostRobotRateLimitExceededV1Beta2Reason = "RateLimitExceeded"
)
