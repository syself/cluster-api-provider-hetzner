/*
Copyright 2025 The Kubernetes Authors.

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

// Package conditions contains CAPI condition helpers used by the Hetzner provider.
// changes needed for the update from capi v1.10 to v1.11.
package conditions

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
)

const readyConditionType = clusterv1.ReadyCondition

// MarkTrue sets the given condition to True on the target object.
func MarkTrue(targetObj capiconditions.Setter, conditionType string) {
	capiconditions.Set(targetObj, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionTrue,
		Reason:  "",
		Message: "",
	})
}

// MarkFalse sets the given condition to False with the provided reason and message.
func MarkFalse(to capiconditions.Setter, conditionType string, reason string, severity clusterv1.ConditionSeverity, messageFormat string, messageArgs ...any) {
	// Severity is not part of metav1.Condition anymore, but we keep the parameter to avoid touching call sites.
	_ = severity

	message := messageFormat
	if len(messageArgs) > 0 {
		message = fmt.Sprintf(messageFormat, messageArgs...)
	}
	capiconditions.Set(to, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

// SetSummary recomputes the Ready condition based on the other conditions present on the object.
func SetSummary(obj capiconditions.Setter) {
	getter, ok := any(obj).(capiconditions.Getter)
	if !ok {
		return
	}

	conditions := getter.GetConditions()
	conditionTypes := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		if condition.Type == readyConditionType {
			continue
		}
		conditionTypes = append(conditionTypes, condition.Type)
	}

	var (
		summary *metav1.Condition
		err     error
	)

	switch len(conditionTypes) {
	case 0:
		summary = &metav1.Condition{
			Type:    readyConditionType,
			Status:  metav1.ConditionUnknown,
			Reason:  capiconditions.NotYetReportedReason,
			Message: "No conditions reported yet",
		}
	default:
		summary, err = capiconditions.NewSummaryCondition(getter, readyConditionType, capiconditions.ForConditionTypes(conditionTypes))
		if err != nil {
			summary = &metav1.Condition{
				Type:    readyConditionType,
				Status:  metav1.ConditionUnknown,
				Reason:  "SummaryError",
				Message: err.Error(),
			}
		}
	}

	capiconditions.Set(obj, *summary)
}

// Delete removes a condition from the object.
func Delete(obj capiconditions.Setter, conditionType string) {
	// Work around bug which is not fixed in 1.11 yet:
	// https://github.com/kubernetes-sigs/cluster-api/pull/13048
	// Please delete that extra check later.
	c := obj.GetConditions()
	if len(c) == 0 {
		return
	}
	capiconditions.Delete(obj, conditionType)
}

// Get returns the condition of the given type if present.
func Get(obj capiconditions.Getter, conditionType string) *metav1.Condition {
	return capiconditions.Get(obj, conditionType)
}

// Has reports whether the object contains the given condition.
func Has(obj capiconditions.Getter, conditionType string) bool {
	return capiconditions.Has(obj, conditionType)
}

// IsFalse reports whether the named condition is set to False.
func IsFalse(obj capiconditions.Getter, conditionType string) bool {
	return capiconditions.IsFalse(obj, conditionType)
}

// IsTrue reports whether the named condition is set to True.
func IsTrue(obj capiconditions.Getter, conditionType string) bool {
	return capiconditions.IsTrue(obj, conditionType)
}

// GetReason returns the reason associated with the named condition.
func GetReason(obj capiconditions.Getter, conditionType string) string {
	return capiconditions.GetReason(obj, conditionType)
}

// GetMessage returns the message associated with the named condition.
func GetMessage(obj capiconditions.Getter, conditionType string) string {
	return capiconditions.GetMessage(obj, conditionType)
}
