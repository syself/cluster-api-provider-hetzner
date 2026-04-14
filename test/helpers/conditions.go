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

package helpers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type conditionedObject interface {
	client.Object
	conditions.Getter
}

func conditionFalseWithReason(getter conditions.Getter, condition clusterv1.ConditionType, reason string) error {
	if !conditions.Has(getter, condition) {
		return fmt.Errorf("%s not set", condition)
	}

	objectCondition := conditions.Get(getter, condition)
	if objectCondition.Status != corev1.ConditionFalse || objectCondition.Reason != reason {
		return fmt.Errorf(
			"expected %s to be False with reason %q, got status=%q reason=%q message=%q",
			condition,
			reason,
			objectCondition.Status,
			objectCondition.Reason,
			objectCondition.Message,
		)
	}

	return nil
}

// ConditionFalseWithReasonAtKey fetches an object and checks that a condition is false with the expected reason.
func ConditionFalseWithReasonAtKey(
	ctx context.Context,
	reader client.Reader,
	key client.ObjectKey,
	obj conditionedObject,
	condition clusterv1.ConditionType,
	reason string,
) error {
	if err := reader.Get(ctx, key, obj); err != nil {
		return err
	}

	return conditionFalseWithReason(obj, condition, reason)
}
