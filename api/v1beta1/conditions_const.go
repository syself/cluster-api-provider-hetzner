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
	ServerProvisionedCondition clusterv1.ConditionType = "ServerProvisioned"
	// ServerOffReason instance is off.
	ServerOffReason = "ServerOff"
)

const (
	// ServerAvailableCondition indicates the instance is in a Running state.
	ServerAvailableCondition clusterv1.ConditionType = "ServerAvailable"
	// ServerTerminatingReason instance is in a terminated state.
	ServerTerminatingReason = "InstanceTerminated"
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
	// HostReadyCondition reports on whether the HetznerBareMetalHost is ready or not. The hbmm
	// reconciler reads the clusterv1.ReadyCondition condition from the host (if the host exists),
	// and mirrors the Reason and Message on the HostReadyCondition of the hbmm.
	HostReadyCondition clusterv1.ConditionType = "HostReady"

	// HostNotFoundReason indicates that the HetznerBaremetalHost associated with the HetznerBaremetalMachine
	// was not found.
	HostNotFoundReason = "HostNotFound"
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
	RobotCredentialsAvailableCondition clusterv1.ConditionType = "RobotCredentialsAvailable"
	// RobotCredentialsInvalidReason indicates that credentials for Robot are invalid.
	RobotCredentialsInvalidReason = "RobotCredentialsInvalid" // #nosec
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

const (
	// RebootSucceededCondition indicates that the machine got rebooted successfully.
	RebootSucceededCondition clusterv1.ConditionType = "RebootSucceeded"
)

// HCloudMachine v1beta2 conditions and reasons.
const (
	// HCloudMachineReadyV1Beta2Condition summarizes the readiness of the HCloudMachine.
	HCloudMachineReadyV1Beta2Condition = clusterv1.ReadyV1Beta2Condition
	// HCloudMachineReadyV1Beta2Reason surfaces when the HCloudMachine is ready.
	HCloudMachineReadyV1Beta2Reason = clusterv1.ReadyV1Beta2Reason
	// HCloudMachineNotReadyV1Beta2Reason surfaces when the HCloudMachine is not ready.
	HCloudMachineNotReadyV1Beta2Reason = clusterv1.NotReadyV1Beta2Reason
	// HCloudMachineReadyUnknownV1Beta2Reason surfaces when the HCloudMachine readiness is unknown.
	HCloudMachineReadyUnknownV1Beta2Reason = clusterv1.ReadyUnknownV1Beta2Reason
)

const (
	// HCloudMachineHCloudTokenAvailableV1Beta2Condition reports on whether the HCloud token is available.
	HCloudMachineHCloudTokenAvailableV1Beta2Condition = "HCloudTokenAvailable"
	// HCloudMachineTokenAvailableV1Beta2Reason surfaces when the HCloud token is available.
	HCloudMachineTokenAvailableV1Beta2Reason = "TokenAvailable"
	// HCloudMachineTokenSecretUnreachableV1Beta2Reason surfaces when the Hetzner secret cannot be reached.
	HCloudMachineTokenSecretUnreachableV1Beta2Reason = "SecretUnreachable"
	// HCloudMachineTokenInvalidV1Beta2Reason surfaces when the HCloud token is invalid.
	HCloudMachineTokenInvalidV1Beta2Reason = "CredentialsInvalid"
)

const (
	// HCloudMachineBootstrapReadyV1Beta2Condition reports on whether bootstrap data is ready.
	HCloudMachineBootstrapReadyV1Beta2Condition = "BootstrapReady"
	// HCloudMachineBootstrapReadyV1Beta2Reason surfaces when bootstrap data is ready.
	HCloudMachineBootstrapReadyV1Beta2Reason = "BootstrapReady"
	// HCloudMachineBootstrapNotReadyV1Beta2Reason surfaces when bootstrap data is not ready yet.
	HCloudMachineBootstrapNotReadyV1Beta2Reason = clusterv1.WaitingForBootstrapDataV1Beta2Reason
)

const (
	// HCloudMachineServerCreatedV1Beta2Condition reports on whether the HCloud server was created.
	HCloudMachineServerCreatedV1Beta2Condition = "ServerCreated"
	// HCloudMachineServerCreatedV1Beta2Reason surfaces when the HCloud server is provisioned.
	HCloudMachineServerCreatedV1Beta2Reason = clusterv1.ProvisionedV1Beta2Reason
	// HCloudMachineServerNotCreatedV1Beta2Reason surfaces when the HCloud server is not provisioned.
	HCloudMachineServerNotCreatedV1Beta2Reason = clusterv1.NotProvisionedV1Beta2Reason
	// HCloudMachineServerCreateFailedIrrecoverableV1Beta2Reason surfaces an irrecoverable create failure.
	HCloudMachineServerCreateFailedIrrecoverableV1Beta2Reason = "CreateFailedIrrecoverable"
)

const (
	// HCloudMachineServerProvisionedV1Beta2Condition reports on whether the HCloud server has completed
	// boot-time provisioning (rescue boot, image install, OS startup).
	HCloudMachineServerProvisionedV1Beta2Condition = "ServerProvisioned"
	// HCloudMachineServerProvisionedV1Beta2Reason surfaces when the boot state machine has completed.
	HCloudMachineServerProvisionedV1Beta2Reason = clusterv1.ProvisionedV1Beta2Reason
	// HCloudMachineServerNotProvisionedV1Beta2Reason surfaces when the boot state machine has not completed.
	HCloudMachineServerNotProvisionedV1Beta2Reason = clusterv1.NotProvisionedV1Beta2Reason
	// HCloudMachineServerProvisioningV1Beta2Reason surfaces when the server is progressing through the boot state machine.
	HCloudMachineServerProvisioningV1Beta2Reason = "Provisioning"
)

const (
	// HCloudMachineServerAvailableV1Beta2Condition reports on whether the HCloud server is available.
	HCloudMachineServerAvailableV1Beta2Condition = "ServerAvailable"
	// HCloudMachineServerAvailableV1Beta2Reason surfaces when the HCloud server is available.
	HCloudMachineServerAvailableV1Beta2Reason = clusterv1.AvailableV1Beta2Reason
	// HCloudMachineServerNotAvailableV1Beta2Reason surfaces when the HCloud server is not available.
	HCloudMachineServerNotAvailableV1Beta2Reason = clusterv1.NotAvailableV1Beta2Reason
	// HCloudMachineServerNotFoundV1Beta2Reason surfaces when the HCloud server cannot be found.
	HCloudMachineServerNotFoundV1Beta2Reason = "ServerNotFound"
	// HCloudMachineNetworkAttachFailedV1Beta2Reason surfaces a network attachment failure.
	HCloudMachineNetworkAttachFailedV1Beta2Reason = "NetworkAttachFailed"
	// HCloudMachineLoadBalancerAttachFailedV1Beta2Reason surfaces a load balancer attachment failure.
	HCloudMachineLoadBalancerAttachFailedV1Beta2Reason = "LoadBalancerAttachFailed"
)

const (
	// HCloudMachineDeletingV1Beta2Condition reports on HCloudMachine deletion progress.
	HCloudMachineDeletingV1Beta2Condition = clusterv1.DeletingV1Beta2Condition
	// HCloudMachineDeletingV1Beta2Reason surfaces when the HCloudMachine is deleting.
	HCloudMachineDeletingV1Beta2Reason = clusterv1.DeletingV1Beta2Reason
	// HCloudMachineNotDeletingV1Beta2Reason surfaces when the HCloudMachine is not deleting.
	HCloudMachineNotDeletingV1Beta2Reason = clusterv1.NotDeletingV1Beta2Reason
)

const (
	// HCloudMachineHetznerAPIReachableV1Beta2Condition reports whether the Hetzner API is reachable for the HCloudMachine.
	HCloudMachineHetznerAPIReachableV1Beta2Condition = "HetznerAPIReachable"
	// HCloudMachineHetznerAPIReachableV1Beta2Reason surfaces when the Hetzner API is reachable.
	HCloudMachineHetznerAPIReachableV1Beta2Reason = "Reachable"
	// HCloudMachineHetznerAPIRateLimitExceededV1Beta2Reason surfaces when requests hit the Hetzner API rate limit.
	HCloudMachineHetznerAPIRateLimitExceededV1Beta2Reason = "RateLimitExceeded"
)

// HCloudMachineV1Beta2OwnedConditions returns a fresh copy of the v1beta2 conditions
// owned by the HCloudMachine controller.
func HCloudMachineV1Beta2OwnedConditions() []string {
	return []string{
		HCloudMachineReadyV1Beta2Condition,
		HCloudMachineHCloudTokenAvailableV1Beta2Condition,
		HCloudMachineBootstrapReadyV1Beta2Condition,
		HCloudMachineServerCreatedV1Beta2Condition,
		HCloudMachineServerProvisionedV1Beta2Condition,
		HCloudMachineServerAvailableV1Beta2Condition,
		HCloudMachineDeletingV1Beta2Condition,
		HCloudMachineHetznerAPIReachableV1Beta2Condition,
	}
}

// HCloudMachineV1Beta2SummaryConditionTypes returns the condition types used for computing
// the Ready summary condition (all owned conditions except Ready itself).
func HCloudMachineV1Beta2SummaryConditionTypes() []string {
	owned := HCloudMachineV1Beta2OwnedConditions()
	types := make([]string, 0, len(owned)-1)
	for _, c := range owned {
		if c != HCloudMachineReadyV1Beta2Condition {
			types = append(types, c)
		}
	}
	return types
}
