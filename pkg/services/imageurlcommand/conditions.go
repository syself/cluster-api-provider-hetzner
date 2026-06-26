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
)

// Status values written by the image-url-command binary into output.json.
const (
	OutputJSONSucceeded  = "Succeeded"
	OutputJSONFailed     = "Failed"
	OutputJSONInProgress = "InProgress"
)

// conditionSetter accepts objects that implement both v1beta1 and v1beta2 condition setters.
type conditionSetter interface {
	v1beta1conditions.Setter
	v1beta2conditions.Setter
}

// Apply updates the given conditions based on the image-url-command output.json status.
// On Succeeded: no-op; the caller handles final success state after post-install steps.
// On Failed: marks the condition False with failedReason.
// On InProgress/other: marks the condition False with progressReason.
func Apply(obj conditionSetter, output Output, v1beta1Cond clusterv1beta1.ConditionType, v1beta2Cond, failedReason, progressReason string) {
	switch output.Status {
	case OutputJSONSucceeded:
		// no-op
	case OutputJSONFailed:
		msg := output.Message
		v1beta1conditions.MarkFalse(obj, v1beta1Cond, failedReason, clusterv1beta1.ConditionSeverityError, "%s", msg)
		v1beta2conditions.Set(obj, metav1.Condition{
			Type:    v1beta2Cond,
			Status:  metav1.ConditionFalse,
			Reason:  failedReason,
			Message: msg,
		})
	default:
		msg := output.Message
		if msg == "" {
			msg = "imageURLCommand running"
		}
		v1beta1conditions.MarkFalse(obj, v1beta1Cond, progressReason, clusterv1beta1.ConditionSeverityInfo, "%s", msg)
		v1beta2conditions.Set(obj, metav1.Condition{
			Type:    v1beta2Cond,
			Status:  metav1.ConditionFalse,
			Reason:  progressReason,
			Message: msg,
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
