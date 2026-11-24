/*
Copyright 2026 The Kubernetes Authors.

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

package controllers

import (
	"fmt"
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DefaultBackoffBaseDelay is the default of the flag --backoff-base-delay.
const (
	DefaultBackoffBaseDelay = time.Second
	MaxBackoffDelay         = 5 * time.Minute
)

// ValidateBackoffBaseDelay validates the value of the flag --backoff-base-delay.
func ValidateBackoffBaseDelay(baseDelay time.Duration) error {
	if baseDelay <= 0 || baseDelay > MaxBackoffDelay {
		return fmt.Errorf("must be greater than 0 and at most %s, got %s", MaxBackoffDelay, baseDelay)
	}
	return nil
}

// NewReconcileBackoff returns the backoff for one controller. The backoff counts failures by object
// name and namespace, so two controllers that share one backoff mix their counts.
func NewReconcileBackoff(baseDelay time.Duration) workqueue.TypedRateLimiter[reconcile.Request] {
	return workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](baseDelay, MaxBackoffDelay)
}
