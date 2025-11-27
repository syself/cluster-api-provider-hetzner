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

// Package conditions provides a wrapper for the new beta2 conditions of CAPI. Having it reduces the
// changes needed for the update from capi v1.10 to v1.11
package conditions

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
)

const readyConditionType = clusterv1.ReadyCondition

func MarkTrue[T ~string](targetObj capiconditions.Setter, conditionType T) {
	capiconditions.Set(targetObj, metav1.Condition{
		Type:    string(conditionType),
		Status:  metav1.ConditionTrue,
		Reason:  "",
		Message: "",
	})
}

func MarkFalse[T ~string, R ~string](to capiconditions.Setter, conditionType T, reason R, severity clusterv1.ConditionSeverity, messageFormat string, messageArgs ...any) {
	// Severity is not part of metav1.Condition anymore, but we keep the parameter to avoid touching call sites.
	_ = severity

	message := messageFormat
	if len(messageArgs) > 0 {
		message = fmt.Sprintf(messageFormat, messageArgs...)
	}
	capiconditions.Set(to, metav1.Condition{
		Type:    string(conditionType),
		Status:  metav1.ConditionFalse,
		Reason:  string(reason),
		Message: message,
	})
}

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
