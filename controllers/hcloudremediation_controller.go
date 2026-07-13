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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	hcloudremediation "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/remediation"
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
		log.Info("Skipping reconciliation for namespace", "namespace", req.Namespace, "annotation", infrav1.SkipNamespaceAnnotation)
		return ctrl.Result{}, nil
	}

	hcloudRemediation := &infrav1.HCloudRemediation{}
	err = r.Get(ctx, req.NamespacedName, hcloudRemediation)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

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

		patchHelper, err := v1beta1patch.NewHelper(hcloudRemediation, r.Client)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to create patch helper for HCloudRemediation: %w", err)
		}
		skippedMsg := fmt.Sprintf(
			"Remediation skipped: HCloudMachine has an irrecoverable server creation error. Delete the Machine to trigger a new creation attempt. Error: %s",
			irrecoverableMsg,
		)
		v1beta1conditions.MarkFalse(
			hcloudRemediation,
			infrav1.RemediationSkippedCondition,
			infrav1.IrrecoverableServerCreateFailureReason,
			clusterv1beta1.ConditionSeverityWarning,
			"%s",
			skippedMsg,
		)
		// Mirror the v1beta1 condition with a v1beta2 condition (negative polarity:
		// status=True means remediation IS skipped).
		v1beta2conditions.Set(hcloudRemediation, metav1.Condition{
			Type:    infrav1.HCloudRemediationSkippedV1Beta2Condition,
			Status:  metav1.ConditionTrue,
			Reason:  infrav1.HCloudRemediationIrrecoverableServerCreateFailureV1Beta2Reason,
			Message: skippedMsg,
		})

		// This is an early-exit path that bypasses the scope, so compute the v1beta2
		// Ready summary here using the shared SummaryOpts.
		if readyCondition, err := v1beta2conditions.NewSummaryCondition(
			hcloudRemediation,
			clusterv1beta1.ReadyV1Beta2Condition,
			infrav1.HCloudRemediationV1Beta2SummaryOpts()...,
		); err == nil {
			v1beta2conditions.Set(hcloudRemediation, *readyCondition)
		} else {
			log.Error(err, "Failed to set v1beta2 Ready condition")
			v1beta2conditions.Set(hcloudRemediation, metav1.Condition{
				Type:   clusterv1beta1.ReadyV1Beta2Condition,
				Status: metav1.ConditionUnknown,
				Reason: infrav1.InternalErrorV1Beta2Reason,
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

	hetznerCluster := &infrav1.HetznerCluster{}

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
	hcloudToken, _, err := getAndValidateHCloudTokenV1Beta1(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hcloudTokenErrorResultV1Beta1(ctx, err, hcloudRemediation, r, infrav1.HCloudRemediationV1Beta2SummaryOpts())
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

	v1beta1conditions.MarkTrue(hcloudRemediation, infrav1.HCloudTokenAvailableCondition)

	// Always close the scope when exiting this function so we can persist any HCloudRemediation changes.
	// Note: the deferred block below is responsible for setting the v1beta2 HCloudTokenAvailable condition
	// on both success and ErrUnauthorized paths, so no pre-defer Set is needed here.
	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			v1beta1conditions.MarkFalse(hcloudRemediation, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudCredentialsInvalidReason, clusterv1beta1.ConditionSeverityError, "wrong hcloud token")
			v1beta2conditions.Set(hcloudRemediation, metav1.Condition{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
				Message: "wrong hcloud token",
			})
		} else {
			v1beta1conditions.MarkTrue(hcloudRemediation, infrav1.HCloudTokenAvailableCondition)
			v1beta2conditions.Set(hcloudRemediation, metav1.Condition{
				Type:   infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: infrav1.HCloudTokenAvailableV1Beta2Reason,
			})
		}

		// Always attempt to Patch the Remediation object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []v1beta1patch.Option{v1beta1patch.WithStatusObservedGeneration{}}

		if err := remediationScope.Close(ctx, patchOpts...); err != nil {
			res = reconcile.Result{}
			reterr = errors.Join(reterr, err)
		}
	}()

	// Check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimitV1Beta1(hcloudRemediation, r.RateLimitWaitTime); wait {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// If we passed the rate limit check, delete any existing rate limit condition.
	v1beta2conditions.Delete(hcloudRemediation, infrav1.HCloudRateLimitExceededV1Beta2Condition)

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
		For(&infrav1.HCloudRemediation{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}
