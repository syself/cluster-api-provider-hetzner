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
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
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
	if conditions.IsFalse(hcloudMachine, infrav1.ServerCreateSucceededCondition) &&
		conditions.GetReason(hcloudMachine, infrav1.ServerCreateSucceededCondition) == infrav1.ServerCreateFailedIrrecoverableErrorReason {
		log.Info("Skipping remediation for machine with irrecoverable creation failure",
			"reason", conditions.GetMessage(hcloudMachine, infrav1.ServerCreateSucceededCondition),
		)

		// signal remediation done.
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
	hcloudToken, _, err := getAndValidateHCloudToken(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hcloudTokenErrorResult(ctx, err, hcloudRemediation, r)
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

	conditions.MarkTrue(hcloudRemediation, infrav1.HCloudTokenAvailableCondition)

	// Always close the scope when exiting this function so we can persist any HCloudRemediation changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			conditions.MarkFalse(hcloudRemediation, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudCredentialsInvalidReason, clusterv1.ConditionSeverityError, "wrong hcloud token")
		} else {
			conditions.MarkTrue(hcloudRemediation, infrav1.HCloudTokenAvailableCondition)
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
		For(&infrav1.HCloudRemediation{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}
