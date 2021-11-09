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
	// NetworkReadyCondition reports on the successful reconciliation of a Network.
	NetworkReadyCondition clusterv1.ConditionType = "NetworkReady"
	// NetworkCreationStartedReason used when attempting to create a Network for a managed cluster.
	// Will not be applied to unmanaged clusters.
	NetworkCreationStartedReason = "NetworkCreationStarted"
	// NetworkReconciliationFailedReason used when errors occur during Network reconciliation.
	NetworkReconciliationFailedReason = "NetworkReconciliationFailed"
)

const (
	// SubnetsReadyCondition reports on the successful reconciliation of subnets.
	SubnetsReadyCondition clusterv1.ConditionType = "SubnetsReady"
	// SubnetsReconciliationFailedReason used to report failures while reconciling subnets.
	SubnetsReconciliationFailedReason = "SubnetsReconciliationFailed"
)

const (
	// LoadBalancerReadyCondition reports on whether a control plane load balancer was successfully reconciled.
	LoadBalancerReadyCondition clusterv1.ConditionType = "LoadBalancerReady"
	// WaitForDNSNameReason used while waiting for a DNS name for the API server to be populated.
	WaitForDNSNameReason = "WaitForDNSName"
	// WaitForDNSNameResolveReason used while waiting for DNS name to resolve.
	WaitForDNSNameResolveReason = "WaitForDNSNameResolve"
	// LoadBalancerFailedReason used when an error occurs during load balancer reconciliation.
	LoadBalancerFailedReason = "LoadBalancerFailed"
)

const (
	// LoadBalancerAttachedToNetworkCondition reports on whether the load balancer is attached to a network.
	LoadBalancerAttachedToNetworkCondition clusterv1.ConditionType = "LoadBalancerAttachedToNetwork"
	// LoadBalancerAttachFailedReason is used when load balancer could not be attached to network.
	LoadBalancerAttachFailedReason = "LoadBalancerAttachFailed"
	// LoadBalancerNoNetworkFoundReason is used when load balancer could not be attached to network.
	LoadBalancerNoNetworkFoundReason = "LoadBalancerNoNetworkFound"
)

const (
	// InstanceReadyCondition reports on current status of the instance. Ready indicates the instance is in a Running state.
	InstanceReadyCondition clusterv1.ConditionType = "InstanceReady"
	// InstanceNotFoundReason used when the instance couldn't be retrieved.
	InstanceNotFoundReason = "InstanceNotFound"
	// InstanceTerminatedReason instance is in a terminated state.
	InstanceTerminatedReason = "InstanceTerminated"
	// InstanceStoppedReason instance is in a stopped state.
	InstanceStoppedReason = "InstanceStopped"
	// InstanceNotReadyReason used when the instance is in a pending state.
	InstanceNotReadyReason = "InstanceNotReady"
	// InstanceProvisionStartedReason set when the provisioning of an instance started.
	InstanceProvisionStartedReason = "InstanceProvisionStarted"
	// InstanceProvisionFailedReason used for failures during instance provisioning.
	InstanceProvisionFailedReason = "InstanceProvisionFailed"
	// WaitingForClusterInfrastructureReason used when machine is waiting for cluster infrastructure to be ready before proceeding.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	// WaitingForBootstrapDataReason used when machine is waiting for bootstrap data to be ready before proceeding.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
)

const (
	// LBAttachedCondition will report true when a control plane is successfully registered with an LB.
	// Only applicable to control plane machines.
	LBAttachedCondition clusterv1.ConditionType = "LBAttached"
	// LBAttachFailedReason used when a control plane node fails to attach to the ELB.
	LBAttachFailedReason = "LBAttachFailed"
	// LBDetachFailedReason used when a control plane node fails to detach from an ELB.
	LBDetachFailedReason = "LBDetachFailed"
)
