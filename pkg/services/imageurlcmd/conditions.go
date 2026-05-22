/*
Copyright 2022 The Kubernetes Authors.

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

// Package imageurlcmd contains shared logic for the image-url-command v2 protocol.
package imageurlcmd

import (
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// ApplyNodeProvisioningConditions parses an ImageURLCommandOutputV2 and sets phase conditions
// on obj. Returns true if all four phases succeeded.
func ApplyNodeProvisioningConditions(obj v1beta1conditions.Setter, output sshclient.ImageURLCommandOutputV2) bool {
	type phaseMapping struct {
		name      string
		condition clusterv1beta1.ConditionType
	}
	phases := []phaseMapping{
		{"Preparation", infrav1.PreparationSucceededCondition},
		{"ImageDeployment", infrav1.ImageDeploymentSucceededCondition},
		{"BootstrapDelivery", infrav1.BootstrapDeliverySucceededCondition},
		{"Handover", infrav1.HandoverSucceededCondition},
	}

	allSucceeded := true
	anyFailed := false
	for _, pm := range phases {
		phase, ok := output.Phases[pm.name]
		if !ok {
			v1beta1conditions.MarkUnknown(obj, pm.condition,
				infrav1.ProvisioningPhaseNotStartedReason, "phase not present in output.json")
			allSucceeded = false
			continue
		}
		switch phase.Status {
		case "Succeeded":
			v1beta1conditions.MarkTrue(obj, pm.condition)
		case "Failed":
			reason := infrav1.ProvisioningPhaseFailedReason
			message := ""
			for _, step := range phase.Steps {
				if step.Name == phase.FailedStep {
					reason = step.Name
					message = step.Message
					break
				}
			}
			v1beta1conditions.MarkFalse(obj, pm.condition, reason,
				clusterv1beta1.ConditionSeverityError, "%s", message)
			allSucceeded = false
			anyFailed = true
		default: // "NotStarted" or unknown
			v1beta1conditions.MarkUnknown(obj, pm.condition,
				infrav1.ProvisioningPhaseNotStartedReason, "phase was not reached")
			allSucceeded = false
		}
	}

	if allSucceeded {
		v1beta1conditions.MarkTrue(obj, infrav1.NodeProvisioningSucceededCondition)
	} else if anyFailed {
		v1beta1conditions.MarkFalse(obj, infrav1.NodeProvisioningSucceededCondition,
			infrav1.ProvisioningPhaseFailedReason, clusterv1beta1.ConditionSeverityError,
			"A provisioning phase failed.")
	} else {
		v1beta1conditions.MarkUnknown(obj, infrav1.NodeProvisioningSucceededCondition,
			infrav1.ProvisioningPhaseNotStartedReason, "Not all phases completed.")
	}

	return allSucceeded
}
