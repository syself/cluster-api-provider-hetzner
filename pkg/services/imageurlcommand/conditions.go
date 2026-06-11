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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// conditionSetter accepts objects that implement both v1beta1 and v1beta2 condition setters.
type conditionSetter interface {
	v1beta1conditions.Setter
	v1beta2conditions.Setter
}

// ApplyNodeProvisioningConditions sets NodeProvisioningSucceededCondition based on
// the top-level status field of the image-url-command output.json.
func ApplyNodeProvisioningConditions(obj conditionSetter, output Output) {
	switch output.Status {
	case "Succeeded":
		v1beta1conditions.MarkTrue(obj, infrav1.NodeProvisioningSucceededCondition)
		v1beta2conditions.Set(obj, metav1.Condition{
			Type:   infrav1.NodeProvisioningSucceededV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.NodeProvisioningSucceededV1Beta2Reason,
		})
	case "Failed":
		v1beta1conditions.MarkFalse(obj, infrav1.NodeProvisioningSucceededCondition,
			infrav1.NodeProvisioningFailedReason, clusterv1beta1.ConditionSeverityError,
			"%s", output.Message)
		v1beta2conditions.Set(obj, metav1.Condition{
			Type:    infrav1.NodeProvisioningSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.NodeProvisioningFailedV1Beta2Reason,
			Message: output.Message,
		})
	default:
		v1beta1conditions.MarkUnknown(obj, infrav1.NodeProvisioningSucceededCondition,
			infrav1.NodeProvisioningInProgressReason, "provisioning in progress")
		v1beta2conditions.Set(obj, metav1.Condition{
			Type:    infrav1.NodeProvisioningSucceededV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  infrav1.NodeProvisioningInProgressV1Beta2Reason,
			Message: "provisioning in progress",
		})
	}
}

// Parse unmarshals content into an Output struct without applying conditions.
func Parse(content string) (Output, error) {
	var output Output
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return Output{}, fmt.Errorf("output.json: %w", err)
	}
	if output.Status == "" {
		return Output{}, fmt.Errorf("output.json: no status field")
	}
	return output, nil
}

// ParseAndApply unmarshals content into Output and updates conditions on obj.
// Returns an error if content is not valid JSON or has no status field.
func ParseAndApply(obj conditionSetter, content string) error {
	output, err := Parse(content)
	if err != nil {
		return err
	}
	ApplyNodeProvisioningConditions(obj, output)
	return nil
}
