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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/remediation"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// HetznerBareMetalRemediationReconciler reconciles a HetznerBareMetalRemediation object.
type HetznerBareMetalRemediationReconciler struct {
	client.Client
	APIReader        client.Reader
	WatchFilterValue string

	// Reconcile only this namespace. Only needed for testing
	Namespace string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;update;patch

// Reconcile reconciles the hetznerBareMetalRemediation object.
func (r *HetznerBareMetalRemediationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (res reconcile.Result, reterr error) {
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

	// Fetch the Hetzner bare metal host instance.
	bareMetalRemediation := &infrav2.HetznerBareMetalRemediation{}
	err = r.Get(ctx, req.NamespacedName, bareMetalRemediation)
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
	initialBareMetalRemediation := bareMetalRemediation.DeepCopy()
	defer func() {
		// We can potentially optimize this further by ensuring that the cache is up to date only in
		// the cases where an outdated cache would lead to problems. Currently, we ensure that the
		// cache is up to date in all cases, i.e. for all possible changes to the
		// HetznerBareMetalRemediation object.
		if cmp.Equal(initialBareMetalRemediation, bareMetalRemediation) {
			// Nothing has changed. No need to wait.
			return
		}

		// The object changed. Wait until the new version is in the local cache

		// Get the latest version from the apiserver.
		apiserverBareMetalRemediation := &infrav2.HetznerBareMetalRemediation{}

		// Use uncached APIReader
		err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(bareMetalRemediation), apiserverBareMetalRemediation)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// resource was deleted. No need to reconcile again.
				reterr = nil
				res = reconcile.Result{}
				return
			}
			reterr = errors.Join(reterr,
				fmt.Errorf("failed get HetznerBareMetalRemediation via uncached APIReader: %w", err))
			return
		}

		apiserverRV := apiserverBareMetalRemediation.ResourceVersion

		err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 3*time.Second, true, func(ctx context.Context) (done bool, err error) {
			// new resource, read from local cache
			latestFromLocalCache := &infrav2.HetznerBareMetalRemediation{}
			getErr := r.Get(ctx, client.ObjectKeyFromObject(apiserverBareMetalRemediation), latestFromLocalCache)
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

	log = log.WithValues("HetznerBareMetalRemediation", klog.KObj(bareMetalRemediation))

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r, bareMetalRemediation.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Machine", klog.KObj(machine))

	// Fetch the BareMetalMachine instance.
	bareMetalMachine := &infrav1.HetznerBareMetalMachine{}

	key := client.ObjectKey{
		Name:      machine.Spec.InfrastructureRef.Name,
		Namespace: bareMetalRemediation.Namespace,
	}

	if err := r.Get(ctx, key, bareMetalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("HetznerBareMetalMachine", klog.KObj(bareMetalMachine))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	if annotations.IsPaused(cluster, bareMetalMachine) {
		log.Info("bareMetalMachine or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	hetznerCluster := &infrav2.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: bareMetalMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		return reconcile.Result{}, nil
	}

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Create the scope.
	remediationScope, err := scope.NewBareMetalRemediationScope(scope.BareMetalRemediationScopeParams{
		Client:               r,
		Logger:               &log,
		Machine:              machine,
		BareMetalMachine:     bareMetalMachine,
		HetznerCluster:       hetznerCluster,
		BareMetalRemediation: bareMetalRemediation,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Always close the scope when exiting this function so we can persist any BareMetalRemediation changes.
	defer func() {
		// Always attempt to Patch the Remediation object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{patch.WithStatusObservedGeneration{}}

		if err := remediationScope.Close(ctx, patchOpts...); err != nil {
			res = reconcile.Result{}
			reterr = errors.Join(reterr, err)
		}
	}()

	if !bareMetalRemediation.DeletionTimestamp.IsZero() {
		// Nothing to do
		return reconcile.Result{}, nil
	}
	return r.reconcileNormal(ctx, remediationScope)
}

func (r *HetznerBareMetalRemediationReconciler) reconcileNormal(ctx context.Context, remediationScope *scope.BareMetalRemediationScope) (reconcile.Result, error) {
	bareMetalRemediation := remediationScope.BareMetalRemediation

	// reconcile bare metal remediation
	result, err := remediation.NewService(remediationScope).Reconcile(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile server for BareMetalRemediation %s/%s: %w",
			bareMetalRemediation.Namespace, bareMetalRemediation.Name, err)
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalRemediationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav2.HetznerBareMetalRemediation{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}
