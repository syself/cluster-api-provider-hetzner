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

// v1beta2 conditions.

// common conditions used across resource types.

const (
	// HCloudRateLimitExceededV1Beta2Condition reports on whether the HCloud API rate limit has been exceeded.
	HCloudRateLimitExceededV1Beta2Condition = "HCloudRateLimitExceeded"
	// HCloudRateLimitExceededV1Beta2Reason indicates that the HCloud API rate limit has been exceeded.
	HCloudRateLimitExceededV1Beta2Reason = "RateLimitExceeded"
)

const (
	// HCloudTokenAvailableV1Beta2Condition reports on whether the HCloud Token is available.
	HCloudTokenAvailableV1Beta2Condition = "HCloudTokenAvailable"
	// HCloudTokenAvailableV1Beta2Reason indicates that the HCloudToken is available.
	HCloudTokenAvailableV1Beta2Reason = clusterv1beta1.AvailableV1Beta2Reason
	// HCloudTokenInvalidV1Beta2Reason indicates that the HCloudToken is invalid.
	HCloudTokenInvalidV1Beta2Reason = "Invalid"
	// SecretUnreachableV1Beta2Reason indicates that the Hetzner secret is unreachable.
	SecretUnreachableV1Beta2Reason = "SecretUnreachable" // #nosec
)

const (
	// InternalErrorV1Beta2Reason indicates an internal error in reconciler.
	InternalErrorV1Beta2Reason = "InternalError"
)

const (
	// HCloudMachineServerCreatedV1Beta2Condition reports on whether the HCloud server was created.
	HCloudMachineServerCreatedV1Beta2Condition = "ServerCreated"
	// HCloudMachineServerCreatedV1Beta2Reason surfaces when the HCloud server has been created.
	HCloudMachineServerCreatedV1Beta2Reason = "Created"
	// HCloudMachineServerWaitingForBootstrapDataV1Beta2Reason surfaces when the server cannot be created because bootstrap data is not yet available.
	HCloudMachineServerWaitingForBootstrapDataV1Beta2Reason = clusterv1beta1.WaitingForBootstrapDataV1Beta2Reason
	// HCloudMachineServerCreateFailedIrrecoverablyV1Beta2Reason surfaces an irrecoverable create failure.
	HCloudMachineServerCreateFailedIrrecoverablyV1Beta2Reason = "CreateFailedIrrecoverably"
	// HCloudMachineServerImageNotFoundV1Beta2Reason surfaces when the specified image cannot be found.
	HCloudMachineServerImageNotFoundV1Beta2Reason = "ImageNotFound"
	// HCloudMachineServerImageAmbiguousV1Beta2Reason surfaces when multiple images match the specified name.
	HCloudMachineServerImageAmbiguousV1Beta2Reason = "ImageAmbiguous"
	// HCloudMachineServerTypeNotFoundV1Beta2Reason surfaces when the specified server type cannot be found.
	HCloudMachineServerTypeNotFoundV1Beta2Reason = "ServerTypeNotFound"
	// HCloudMachineServerSSHKeyNotFoundV1Beta2Reason surfaces when a required SSH key is not present in HCloud.
	HCloudMachineServerSSHKeyNotFoundV1Beta2Reason = "SSHKeyNotFound"
	// HCloudMachineServerPlacementGroupNotFoundV1Beta2Reason surfaces when the specified placement group does not exist.
	HCloudMachineServerPlacementGroupNotFoundV1Beta2Reason = "PlacementGroupNotFound"
	// HCloudMachineServerCreateFailedV1Beta2Reason surfaces when the HCloud API CreateServer call fails.
	HCloudMachineServerCreateFailedV1Beta2Reason = "CreateFailed"
)

const (
	// HCloudMachineServerProvisionedV1Beta2Condition reports on whether the HCloud server has completed
	// boot-time provisioning (rescue boot, image install, OS startup).
	HCloudMachineServerProvisionedV1Beta2Condition = "ServerProvisioned"
	// HCloudMachineServerProvisionedV1Beta2Reason surfaces when the boot state machine has completed.
	HCloudMachineServerProvisionedV1Beta2Reason = clusterv1beta1.ProvisionedV1Beta2Reason
	// HCloudMachineBootStateInitializingV1Beta2Reason indicates the boot state is being initialized.
	HCloudMachineBootStateInitializingV1Beta2Reason = "BootStateInitializing"
	// HCloudMachineBootStateInitializingTimedOutV1Beta2Reason indicates the boot state initialization timed out.
	HCloudMachineBootStateInitializingTimedOutV1Beta2Reason = "BootStateInitializingTimedOut"
	// HCloudMachineProvisioningServerV1Beta2Reason indicates the server is being provisioned.
	HCloudMachineProvisioningServerV1Beta2Reason = "Provisioning"
	// HCloudMachineServerNotRunningYetV1Beta2Reason indicates the server is not running yet.
	HCloudMachineServerNotRunningYetV1Beta2Reason = "NotRunningYet"
	// HCloudMachineUnknownHCloudStatusV1Beta2Reason indicates an unknown HCloud server status.
	HCloudMachineUnknownHCloudStatusV1Beta2Reason = "UnknownHCloudStatus"
	// HCloudMachineUnknownServerStatusV1Beta2Reason indicates an unknown server status.
	HCloudMachineUnknownServerStatusV1Beta2Reason = "UnknownServerStatus"
	// HCloudMachineImageURLSetButNoCommandProvidedV1Beta2Reason indicates imageURL is set but no command is provided.
	HCloudMachineImageURLSetButNoCommandProvidedV1Beta2Reason = "ImageURLSetButNoCommandProvided"

	// HCloudMachineWaitForRescueSystemV1Beta2Reason indicates waiting for the rescue system to be enabled.
	HCloudMachineWaitForRescueSystemV1Beta2Reason = "WaitForRescueSystem"
	// HCloudMachineEnablingRescueTimedOutV1Beta2Reason indicates enabling rescue system timed out.
	HCloudMachineEnablingRescueTimedOutV1Beta2Reason = "EnablingRescueTimedOut"
	// HCloudMachineActionIDForEnablingRescueSystemNotSetV1Beta2Reason indicates the action ID for enabling rescue is not set.
	HCloudMachineActionIDForEnablingRescueSystemNotSetV1Beta2Reason = "ActionIDForEnablingRescueSystemNotSet"
	// HCloudMachineEnablingRescueGetActionFailedV1Beta2Reason indicates getting the rescue enable action failed.
	HCloudMachineEnablingRescueGetActionFailedV1Beta2Reason = "EnablingRescueGetActionFailed"
	// HCloudMachineWaitingForEnablingRescueActionV1Beta2Reason indicates waiting for the rescue enable action to finish.
	HCloudMachineWaitingForEnablingRescueActionV1Beta2Reason = "WaitingForEnablingRescueAction"
	// HCloudMachineEnablingRescueActionFailedV1Beta2Reason indicates the rescue enable action failed.
	HCloudMachineEnablingRescueActionFailedV1Beta2Reason = "EnablingRescueActionFailed"
	// HCloudMachineEnablingRescueActionDoneV1Beta2Reason indicates the rescue enable action is done.
	HCloudMachineEnablingRescueActionDoneV1Beta2Reason = "EnablingRescueActionDone"
	// HCloudMachineRescueNotEnabledYetV1Beta2Reason indicates the rescue system is not enabled yet.
	HCloudMachineRescueNotEnabledYetV1Beta2Reason = "RescueNotEnabledYet"

	// HCloudMachineGetSSHPrivateKeyFailedV1Beta2Reason indicates getting the SSH private key failed.
	HCloudMachineGetSSHPrivateKeyFailedV1Beta2Reason = "GetSSHPrivateKeyFailed"
	// HCloudMachineGetSSHClientFailedV1Beta2Reason indicates getting the SSH client failed.
	HCloudMachineGetSSHClientFailedV1Beta2Reason = "GetSSHClientFailed"
	// HCloudMachineRetryingSSHConnectionV1Beta2Reason indicates the SSH connection is being retried.
	HCloudMachineRetryingSSHConnectionV1Beta2Reason = "RetryingSSHConnection"
	// HCloudMachineRebootViaSSHFailedV1Beta2Reason indicates rebooting via SSH failed.
	HCloudMachineRebootViaSSHFailedV1Beta2Reason = "RebootViaSSHFailed"
	// HCloudMachineGetHostnameFailedV1Beta2Reason indicates getting the hostname failed.
	HCloudMachineGetHostnameFailedV1Beta2Reason = "GetHostnameFailed"
	// HCloudMachineUnexpectedHostnameV1Beta2Reason indicates the remote hostname was unexpected.
	HCloudMachineUnexpectedHostnameV1Beta2Reason = "UnexpectedHostname"

	// HCloudMachineBootingToRescueV1Beta2Reason indicates the server is booting to rescue mode.
	HCloudMachineBootingToRescueV1Beta2Reason = "BootingToRescue"
	// HCloudMachineBootingToRescueTimedOutV1Beta2Reason indicates booting to rescue mode timed out.
	HCloudMachineBootingToRescueTimedOutV1Beta2Reason = "BootingToRescueTimedOut"
	// HCloudMachineWaitForRescueEnabledToBeFalseV1Beta2Reason indicates waiting for rescue enabled to become false.
	HCloudMachineWaitForRescueEnabledToBeFalseV1Beta2Reason = "WaitForRescueEnabledToBeFalse"

	// HCloudMachineImageURLCommandNotAccessibleV1Beta2Reason indicates the image URL command is not accessible.
	HCloudMachineImageURLCommandNotAccessibleV1Beta2Reason = "ImageURLCommandNotAccessible"
	// HCloudMachineStartImageURLCommandFailedV1Beta2Reason indicates starting the image URL command failed.
	HCloudMachineStartImageURLCommandFailedV1Beta2Reason = "StartImageURLCommandFailed"
	// HCloudMachineStartImageURLCommandNoZeroExitCodeV1Beta2Reason indicates the image URL command returned a non-zero exit code.
	HCloudMachineStartImageURLCommandNoZeroExitCodeV1Beta2Reason = "StartImageURLCommandNoZeroExitCode"
	// HCloudMachineHCloudImageURLCommandRunningV1Beta2Reason indicates the image URL command is running.
	HCloudMachineHCloudImageURLCommandRunningV1Beta2Reason = "HCloudImageURLCommandRunning"
	// HCloudMachineRunningImageCommandTimedOutV1Beta2Reason indicates the running image command timed out.
	HCloudMachineRunningImageCommandTimedOutV1Beta2Reason = "RunningImageCommandTimedOut"
	// HCloudMachineImageCommandFailedV1Beta2Reason indicates the image command failed.
	HCloudMachineImageCommandFailedV1Beta2Reason = "ImageCommandFailed"
	// HCloudMachineBootingToRealOSV1Beta2Reason indicates the server is booting to the real OS.
	HCloudMachineBootingToRealOSV1Beta2Reason = "BootingToRealOS"
	// HCloudMachineBootingToRealOSTimedOutV1Beta2Reason indicates booting to the real OS timed out.
	HCloudMachineBootingToRealOSTimedOutV1Beta2Reason = "BootingToRealOSTimedOut"

	// HCloudMachineGetServerImageFailedV1Beta2Reason indicates getting the server image failed.
	HCloudMachineGetServerImageFailedV1Beta2Reason = "GetServerImageFailed"
	// HCloudMachineGetRawBootstrapDataFailedV1Beta2Reason indicates getting the raw bootstrap data failed.
	HCloudMachineGetRawBootstrapDataFailedV1Beta2Reason = "GetRawBootstrapDataFailed"
	// HCloudMachinePowerOnServerFailedV1Beta2Reason indicates powering on the server failed.
	HCloudMachinePowerOnServerFailedV1Beta2Reason = "PowerOnServerFailed"
	// HCloudMachineServerOffV1Beta2Reason indicates the server is off.
	HCloudMachineServerOffV1Beta2Reason = "ServerOff"
	// HCloudMachineServerOffTimeoutV1Beta2Reason indicates the server off timeout was reached.
	HCloudMachineServerOffTimeoutV1Beta2Reason = "ServerOffTimeout"
)

const (
	// HCloudMachineServerAvailableV1Beta2Condition reports on whether the HCloud server is available.
	HCloudMachineServerAvailableV1Beta2Condition = "ServerAvailable"
	// HCloudMachineServerAvailableV1Beta2Reason surfaces when the HCloud server is available.
	HCloudMachineServerAvailableV1Beta2Reason = clusterv1beta1.AvailableV1Beta2Reason
	// HCloudMachineServerNotFoundV1Beta2Reason surfaces when the HCloud server cannot be found.
	HCloudMachineServerNotFoundV1Beta2Reason = "NotFound"
	// HCloudMachineNetworkAttachFailedV1Beta2Reason surfaces a network attachment failure.
	HCloudMachineNetworkAttachFailedV1Beta2Reason = "NetworkAttachFailed"
	// HCloudMachineWaitingForAPIServerV1Beta2Reason indicates waiting for the API server to be healthy.
	HCloudMachineWaitingForAPIServerV1Beta2Reason = "WaitingForAPIServer"
	// HCloudMachineLoadBalancerAttachFailedV1Beta2Reason surfaces a load balancer attachment failure.
	HCloudMachineLoadBalancerAttachFailedV1Beta2Reason = "LoadBalancerAttachFailed"
	// HCloudMachineDeletingV1Beta2Reason surfaces when the HCloudMachine is being deleted.
	HCloudMachineDeletingV1Beta2Reason = clusterv1beta1.DeletingV1Beta2Reason
)
