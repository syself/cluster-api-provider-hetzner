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
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
)

// This file holds the condition and token helpers for the controllers. util_v1beta1.go holds the
// counterparts for the controllers that have not been migrated yet, and can be deleted once they
// are.

// reconcileRateLimit checks whether the rate limit has been reached and returns whether the
// controller should wait a bit more. When the wait is over it clears the rate-limit conditions
// (HetznerAPIReachable marked reachable again, HCloudRateLimitExceeded deleted, since we cannot know
// the limit is gone until the next API call).
func reconcileRateLimit(cluster *infrav2.HetznerCluster, rateLimitWaitTime time.Duration) bool {
	condition := conditions.Get(cluster, infrav2.HCloudRateLimitExceededCondition)
	if condition != nil && condition.Status == metav1.ConditionTrue {
		if time.Now().Before(condition.LastTransitionTime.Add(rateLimitWaitTime)) {
			// Rate limit wait has not elapsed yet, so signal the caller to requeue.
			// The caller requeues after a fixed interval rather than the exact remaining
			// wait, so that objects rate-limited together do not all reconcile at once.
			return true
		}
		// Wait time is over, we continue.
		deprecatedv1beta1conditions.MarkTrue(cluster, infrav2.HetznerAPIReachableV1Beta1Condition)
		conditions.Delete(cluster, infrav2.HCloudRateLimitExceededCondition)
	}

	return false
}

// getAndValidateHCloudToken acquires the Hetzner secret referenced by the cluster and returns the
// HCloud token from it. It returns a *ResolveSecretRefError if the secret is missing and a
// *HCloudTokenValidationError if the token is empty, which the hcloudTokenErrorResult helpers map to
// the right conditions.
func getAndValidateHCloudToken(ctx context.Context, namespace string, hetznerCluster *infrav2.HetznerCluster, secretManager *secretutil.SecretManager) (string, *corev1.Secret, error) {
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

// hcloudTokenErrorResult handles errors from getAndValidateHCloudToken. It sets the
// HCloudTokenAvailable condition, computes the Ready summary, and writes the status once.
func hcloudTokenErrorResult(
	ctx context.Context,
	inerr error,
	cluster *infrav2.HetznerCluster,
	crClient client.Client,
	summaryOpts []conditions.SummaryOption,
) (ctrl.Result, error) {
	res := ctrl.Result{}

	switch inerr.(type) {
	// In the event that the reference to the secret is defined, but we cannot find it
	// we requeue the host as we will not know if they create the secret
	// at some point in the future.
	case *secretutil.ResolveSecretRefError:
		deprecatedv1beta1conditions.MarkFalse(cluster,
			infrav2.HCloudTokenAvailableV1Beta1Condition,
			infrav2.HetznerSecretUnreachableV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"could not find HetznerSecret",
		)
		conditions.Set(cluster, metav1.Condition{
			Type:    infrav2.HCloudTokenAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HCloudTokenSecretUnreachableReason,
			Message: "could not find HetznerSecret",
		})
		res = ctrl.Result{RequeueAfter: secretErrorRetryDelay}
		inerr = nil

	// No need to reconcile again, as it will be triggered as soon as the secret is updated.
	case *secretutil.HCloudTokenValidationError:
		deprecatedv1beta1conditions.MarkFalse(cluster,
			infrav2.HCloudTokenAvailableV1Beta1Condition,
			infrav2.HCloudCredentialsInvalidV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"invalid or not specified hcloud token in Hetzner secret",
		)
		conditions.Set(cluster, metav1.Condition{
			Type:    infrav2.HCloudTokenAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HCloudTokenInvalidReason,
			Message: "invalid or not specified hcloud token in Hetzner secret",
		})

	default:
		deprecatedv1beta1conditions.MarkFalse(cluster,
			infrav2.HCloudTokenAvailableV1Beta1Condition,
			infrav2.HCloudCredentialsInvalidV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"%s",
			inerr.Error(),
		)
		conditions.Set(cluster, metav1.Condition{
			Type:    infrav2.HCloudTokenAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HCloudTokenInvalidReason,
			Message: inerr.Error(),
		})
		return reconcile.Result{}, fmt.Errorf("an unhandled failure occurred with the Hetzner secret: %w", inerr)
	}

	deprecatedv1beta1conditions.SetSummary(cluster)

	if len(summaryOpts) > 0 {
		if readyCondition, err := conditions.NewSummaryCondition(
			cluster,
			clusterv1.ReadyCondition,
			summaryOpts...,
		); err == nil {
			conditions.Set(cluster, *readyCondition)
		}
	}

	if err := crClient.Status().Update(ctx, cluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("hcloudTokenErrorResult: failed to update: %w", err)
	}
	if inerr != nil {
		return reconcile.Result{}, inerr
	}
	return res, nil
}
