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

// This file holds the still-v1beta1 counterparts of the condition and token helpers in util.go.
// They are used by controllers that still reconcile v1beta1 resources, through the deprecated v1beta1
// condition packages. The whole file can be deleted once the last controller is migrated, leaving
// only the helpers in util.go.

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
)

// reconcileRateLimitV1Beta1 is the still-v1beta1 counterpart of reconcileRateLimit. It detects the
// rate limit through the deprecated v1beta1 HetznerAPIReachable condition and, when the wait is over,
// marks it reachable again and removes the staged v1beta2 rate-limit condition if the object has one.
func reconcileRateLimitV1Beta1(setter v1beta1conditions.Setter, rateLimitWaitTime time.Duration) bool {
	condition := v1beta1conditions.Get(setter, infrav1.HetznerAPIReachableCondition)
	if condition != nil && condition.Status == corev1.ConditionFalse {
		if time.Now().Before(condition.LastTransitionTime.Add(rateLimitWaitTime)) {
			// Rate limit wait has not elapsed yet, so signal the caller to requeue.
			// The caller requeues after a fixed interval rather than the exact remaining
			// wait, so that objects rate-limited together do not all reconcile at once.
			return true
		}
		// Wait time is over, we continue.
		v1beta1conditions.MarkTrue(setter, infrav1.HetznerAPIReachableCondition)

		// Also remove the staged v1beta2 rate limit condition if the type supports it. We are not
		// marking it false here as we cannot guarantee the rate limit is gone until the next HCloud
		// API request.
		if v1beta2Setter, ok := setter.(v1beta2conditions.Setter); ok {
			if v1beta2conditions.Has(v1beta2Setter, infrav1.HCloudRateLimitExceededV1Beta2Condition) {
				v1beta2conditions.Delete(v1beta2Setter, infrav1.HCloudRateLimitExceededV1Beta2Condition)
			}
		}
	}

	return false
}

// getAndValidateHCloudTokenV1Beta1 is the still-v1beta1 counterpart of getAndValidateHCloudToken.
func getAndValidateHCloudTokenV1Beta1(ctx context.Context, namespace string, hetznerCluster *infrav1.HetznerCluster, secretManager *secretutil.SecretManager) (string, *corev1.Secret, error) {
	// retrieve Hetzner secret
	secretNamespacedName := types.NamespacedName{Namespace: namespace, Name: hetznerCluster.Spec.HetznerSecret.Name}

	hetznerSecret, err := secretManager.AcquireSecret(
		ctx,
		secretNamespacedName,
		hetznerCluster,
		false,
		hetznerCluster.DeletionTimestamp.IsZero(),
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil, &secretutil.ResolveSecretRefError{Message: fmt.Sprintf("The Hetzner secret %s does not exist", secretNamespacedName)}
		}
		return "", nil, err
	}

	hcloudToken := string(hetznerSecret.Data[hetznerCluster.Spec.HetznerSecret.Key.HCloudToken])

	// Validate token
	if hcloudToken == "" {
		return "", nil, &secretutil.HCloudTokenValidationError{}
	}

	return hcloudToken, hetznerSecret, nil
}

// hcloudTokenErrorResultV1Beta1 is the still-v1beta1 counterpart of hcloudTokenErrorResult. It sets
// the deprecated v1beta1 HCloudTokenAvailable condition and, when the object supports them, the
// staged v1beta2 conditions, then computes the Ready summary (v1beta1 always, v1beta2 from the
// passed opts) before writing the status once.
func hcloudTokenErrorResultV1Beta1(
	ctx context.Context,
	inerr error,
	setter v1beta1conditions.Setter,
	crClient client.Client,
	v1beta2SummaryOpts []v1beta2conditions.SummaryOption,
) (ctrl.Result, error) {
	res := ctrl.Result{}
	v1beta2Setter, hasV1Beta2 := setter.(v1beta2conditions.Setter)

	switch inerr.(type) {
	// In the event that the reference to the secret is defined, but we cannot find it
	// we requeue the host as we will not know if they create the secret
	// at some point in the future.
	case *secretutil.ResolveSecretRefError:
		v1beta1conditions.MarkFalse(setter,
			infrav1.HCloudTokenAvailableCondition,
			infrav1.HetznerSecretUnreachableReason,
			clusterv1beta1.ConditionSeverityError,
			"could not find HetznerSecret",
		)
		if hasV1Beta2 {
			v1beta2conditions.Set(v1beta2Setter, metav1.Condition{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenSecretUnreachableV1Beta2Reason,
				Message: "could not find HetznerSecret",
			})
		}
		res = ctrl.Result{RequeueAfter: secretErrorRetryDelay}
		inerr = nil

	// No need to reconcile again, as it will be triggered as soon as the secret is updated.
	case *secretutil.HCloudTokenValidationError:
		v1beta1conditions.MarkFalse(setter,
			infrav1.HCloudTokenAvailableCondition,
			infrav1.HCloudCredentialsInvalidReason,
			clusterv1beta1.ConditionSeverityError,
			"invalid or not specified hcloud token in Hetzner secret",
		)
		if hasV1Beta2 {
			v1beta2conditions.Set(v1beta2Setter, metav1.Condition{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
				Message: "invalid or not specified hcloud token in Hetzner secret",
			})
		}

	default:
		v1beta1conditions.MarkFalse(setter,
			infrav1.HCloudTokenAvailableCondition,
			infrav1.HCloudCredentialsInvalidReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			inerr.Error(),
		)
		if hasV1Beta2 {
			v1beta2conditions.Set(v1beta2Setter, metav1.Condition{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
				Message: inerr.Error(),
			})
		}
		return reconcile.Result{}, fmt.Errorf("an unhandled failure occurred with the Hetzner secret: %w", inerr)
	}

	v1beta1conditions.SetSummary(setter)

	if hasV1Beta2 && len(v1beta2SummaryOpts) > 0 {
		if readyCondition, err := v1beta2conditions.NewSummaryCondition(
			v1beta2Setter,
			clusterv1beta1.ReadyV1Beta2Condition,
			v1beta2SummaryOpts...,
		); err == nil {
			v1beta2conditions.Set(v1beta2Setter, *readyCondition)
		}
	}

	if err := crClient.Status().Update(ctx, setter); err != nil {
		return reconcile.Result{}, fmt.Errorf("hcloudTokenErrorResultV1Beta1: failed to update: %w", err)
	}
	if inerr != nil {
		return reconcile.Result{}, inerr
	}
	return res, nil
}
