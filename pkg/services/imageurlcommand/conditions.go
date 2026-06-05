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

// Package imageurlcommand contains shared logic for the image-url-command protocol.
package imageurlcommand

import (
	"encoding/json"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// ApplyNodeProvisioningConditions sets NodeProvisioningSucceededCondition based on
// the top-level status field of the image-url-command output.json.
func ApplyNodeProvisioningConditions(obj v1beta1conditions.Setter, output OutputV2) {
	switch output.Status {
	case "Succeeded":
		v1beta1conditions.MarkTrue(obj, infrav1.NodeProvisioningSucceededCondition)
	case "Failed":
		v1beta1conditions.MarkFalse(obj, infrav1.NodeProvisioningSucceededCondition,
			infrav1.NodeProvisioningFailedReason, clusterv1beta1.ConditionSeverityError,
			"%s", output.Message)
	default:
		v1beta1conditions.MarkUnknown(obj, infrav1.NodeProvisioningSucceededCondition,
			infrav1.NodeProvisioningInProgressReason, "provisioning in progress")
	}
}

// ParseAndApply unmarshals content into OutputV2 and updates conditions on obj.
// It is a no-op if content is empty, not valid JSON, or has no status field.
func ParseAndApply(obj v1beta1conditions.Setter, content string) {
	var output OutputV2
	if err := json.Unmarshal([]byte(content), &output); err == nil && output.Status != "" {
		ApplyNodeProvisioningConditions(obj, output)
	}
}
