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
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
)

func MarkTrue(targetObj capiconditions.Setter, conditionType string) {
	capiconditions.Set(targetObj, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionTrue,
		Reason:  "",
		Message: "",
	})
}

func MarkFalse(to capiconditions.Setter, conditionType string, reason string, messageFormat string, messageArgs ...any) {
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
