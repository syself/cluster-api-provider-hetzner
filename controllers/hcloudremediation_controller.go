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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1" // deprecated conditions on the v1beta2 HCloudRemediation
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"           // conditions on the still-v1beta1 HCloudMachine
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	hcloudremediation "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/remediation"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// HCloudRemediationReconciler reconciles a HCloudRemediation object.
type HCloudRemediationReconciler struct {
	client.Client
	RateLimitWaitTime   time.Duration
	APIReader           client.Reader
	HCloudClientFactory hcloudclient.Factory
	WatchFilterValue    string

	// Reconcile only this namespace. Only needed for testing
	Namespace string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudremediations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudremediations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudremediations/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;update;patch

// Reconcile reconciles the hetznerHCloudRemediation object.
func (r *HCloudRemediationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (res reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	if r.Namespace != "" && req.Namespace != r.Namespace {
		// Just for testing, skip reconciling objects from finished tests.
		return ctrl.Result{}, nil
	}
	skipReconciliation, err := shouldSkipReconciliationForNamespace(ctx, r.Client, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if skipReconciliation {
		log.Info("Skipping reconciliation for namespace", "namespace", req.Namespace, "annotation", infrav2.SkipNamespaceAnnotation)
		return ctrl.Result{}, nil
	}

	hcloudRemediation := &infrav2.HCloudRemediation{}
	err = r.Get(ctx, req.NamespacedName, hcloudRemediation)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// ----------------------------------------------------------------
	// Start: avoid conflict errors. Wait until local cache is up-to-date
	// Won't be needed once this was implemented:
	// https://github.com/kubernetes-sigs/controller-runtime/issues/3320
	initialHCloudRemediation := hcloudRemediation.DeepCopy()
	defer func() {
		// We can potentially optimize this further by ensuring that the cache is up to date only in
		// the cases where an outdated cache would lead to problems. Currently, we ensure that the
		// cache is up to date in all cases, i.e. for all possible changes to the
		// HCloudRemediation object.
		if cmp.Equal(initialHCloudRemediation, hcloudRemediation) {
			// Nothing has changed. No need to wait.
			return
		}

		// The object changed. Wait until the new version is in the local cache

		// Get the latest version from the apiserver.
		apiserverHCloudRemediation := &infrav2.HCloudRemediation{}

		// Use uncached APIReader
		err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(hcloudRemediation), apiserverHCloudRemediation)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// resource was deleted. No need to reconcile again.
				reterr = nil
				res = reconcile.Result{}
				return
			}
			reterr = errors.Join(reterr,
				fmt.Errorf("failed get HCloudRemediation via uncached APIReader: %w", err))
			return
		}

		apiserverRV := apiserverHCloudRemediation.ResourceVersion

		err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 3*time.Second, true, func(ctx context.Context) (done bool, err error) {
			// new resource, read from local cache
			latestFromLocalCache := &infrav2.HCloudRemediation{}
			getErr := r.Get(ctx, client.ObjectKeyFromObject(apiserverHCloudRemediation), latestFromLocalCache)
			if apierrors.IsNotFound(getErr) {
				// the object was deleted. All is fine.
				return true, nil
			}
			if getErr != nil {
				return false, getErr
			}
			return utils.IsLocalCacheUpToDate(latestFromLocalCache.ResourceVersion, apiserverRV), nil
		})
		if err != nil {
			log.Error(err, "cache sync failed")
		}
	}()
	// End: avoid conflict errors. Wait until local cache is up-to-date
	// ----------------------------------------------------------------

	log = log.WithValues("HCloudRemediation", klog.KObj(hcloudRemediation))

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r, hcloudRemediation.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Machine", klog.KObj(machine))

	// Fetch the HCloudMachine instance.
	hcloudMachine := &infrav1.HCloudMachine{}

	key := client.ObjectKey{
		Name:      machine.Spec.InfrastructureRef.Name,
		Namespace: hcloudRemediation.Namespace,
	}

	if err := r.Get(ctx, key, hcloudMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("HCloudMachine", klog.KObj(hcloudMachine))

	// Skip remediation for machines that failed to create with irrecoverable errors (e.g. invalid_input, resource_unavailable).
	// These errors cannot be fixed by rebooting or replacing the machine.
	// We return without error so the MHC does not keep retrying remediation.
	if v1beta1conditions.IsFalse(hcloudMachine, infrav1.ServerCreateSucceededCondition) &&
		v1beta1conditions.GetReason(hcloudMachine, infrav1.ServerCreateSucceededCondition) == infrav1.ServerCreateFailedIrrecoverableErrorReason {
		irrecoverableMsg := v1beta1conditions.GetMessage(hcloudMachine, infrav1.ServerCreateSucceededCondition)
		log.Info("Skipping remediation for machine with irrecoverable creation failure",
			"reason", irrecoverableMsg,
		)

		patchHelper, err := patch.NewHelper(hcloudRemediation, r.Client)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to create patch helper for HCloudRemediation: %w", err)
		}
		skippedMsg := fmt.Sprintf(
			"Remediation skipped: HCloudMachine has an irrecoverable server creation error. Delete the Machine to trigger a new creation attempt. Error: %s",
			irrecoverableMsg,
		)
		deprecatedv1beta1conditions.MarkFalse(
			hcloudRemediation,
			infrav2.RemediationSkippedV1Beta1Condition,
			infrav2.IrrecoverableServerCreateFailureV1Beta1Reason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			skippedMsg,
		)
		// negative polarity: status=True means remediation IS skipped.
		conditions.Set(hcloudRemediation, metav1.Condition{
			Type:    infrav2.HCloudRemediationSkippedCondition,
			Status:  metav1.ConditionTrue,
			Reason:  infrav2.HCloudRemediationIrrecoverableServerCreateFailureReason,
			Message: skippedMsg,
		})

		// This is an early-exit path that bypasses the scope, so compute the Ready summary
		// here using the shared SummaryOpts.
		if readyCondition, err := conditions.NewSummaryCondition(
			hcloudRemediation,
			clusterv1.ReadyCondition,
			infrav2.HCloudRemediationSummaryOpts()...,
		); err == nil {
			conditions.Set(hcloudRemediation, *readyCondition)
		} else {
			log.Error(err, "Failed to set Ready condition")
			conditions.Set(hcloudRemediation, metav1.Condition{
				Type:   clusterv1.ReadyCondition,
				Status: metav1.ConditionUnknown,
				Reason: clusterv1.InternalErrorReason,
			})
		}

		if err := patchHelper.Patch(ctx, hcloudRemediation, scope.HCloudRemediationPatchOpts()...); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to patch HCloudRemediation status: %w", err)
		}

		return reconcile.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	if annotations.IsPaused(cluster, hcloudMachine) {
		log.Info("HCloudMachine or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	hetznerCluster := &infrav2.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: hcloudMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		return reconcile.Result{}, nil
	}

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Create the scope.
	secretManager := secretutil.NewSecretManager(log, r, r.APIReader)
	hcloudToken, _, err := getAndValidateHCloudToken(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hcloudTokenErrorResult(ctx, err, hcloudRemediation, r, infrav2.HCloudRemediationSummaryOpts())
	}

	hcc := r.HCloudClientFactory.NewClient(hcloudToken)

	remediationScope, err := scope.NewHCloudRemediationScope(scope.HCloudRemediationScopeParams{
		Client:            r,
		Logger:            log,
		Machine:           machine,
		HCloudMachine:     hcloudMachine,
		HetznerCluster:    hetznerCluster,
		HCloudRemediation: hcloudRemediation,
		HCloudClient:      hcc,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Always close the scope when exiting this function so we can persist any HCloudRemediation
	// changes. The deferred block also sets the HCloudTokenAvailable condition and its deprecated
	// v1beta1 counterpart, based on whether the reconcile hit an unauthorized error.
	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			deprecatedv1beta1conditions.MarkFalse(hcloudRemediation, infrav2.HCloudTokenAvailableV1Beta1Condition, infrav2.HCloudCredentialsInvalidV1Beta1Reason, clusterv1.ConditionSeverityError, "wrong hcloud token")
			conditions.Set(hcloudRemediation, metav1.Condition{
				Type:    infrav2.HCloudTokenAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudTokenInvalidReason,
				Message: "wrong hcloud token",
			})
		} else {
			deprecatedv1beta1conditions.MarkTrue(hcloudRemediation, infrav2.HCloudTokenAvailableV1Beta1Condition)
			conditions.Set(hcloudRemediation, metav1.Condition{
				Type:   infrav2.HCloudTokenAvailableCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HCloudTokenAvailableReason,
			})
		}

		// Always attempt to Patch the Remediation object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{patch.WithStatusObservedGeneration{}}

		if err := remediationScope.Close(ctx, patchOpts...); err != nil {
			res = reconcile.Result{}
			reterr = errors.Join(reterr, err)
		}
	}()

	// Check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(hcloudRemediation, r.RateLimitWaitTime); wait {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !hcloudRemediation.DeletionTimestamp.IsZero() {
		// Nothing to do
		return reconcile.Result{}, nil
	}

	return r.reconcileNormal(ctx, remediationScope)
}

func (r *HCloudRemediationReconciler) reconcileNormal(ctx context.Context, remediationScope *scope.HCloudRemediationScope) (reconcile.Result, error) {
	hcloudRemediation := remediationScope.HCloudRemediation

	// reconcile hcloud remediation
	result, err := hcloudremediation.NewService(remediationScope).Reconcile(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile server for HCloudRemediation %s/%s: %w",
			hcloudRemediation.Namespace, hcloudRemediation.Name, err)
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HCloudRemediationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav2.HCloudRemediation{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}
