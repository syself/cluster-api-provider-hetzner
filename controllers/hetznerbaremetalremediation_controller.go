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

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/remediation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// HetznerBareMetalRemediationReconciler reconciles a HetznerBareMetalRemediation object.
type HetznerBareMetalRemediationReconciler struct {
	client.Client
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediations/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;update;patch

// Reconcile reconciles the hetznerBareMetalRemediation object.
func (r *HetznerBareMetalRemediationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the Hetzner bare metal host instance.
	bareMetalRemediation := &infrav1.HetznerBareMetalRemediation{}
	err := r.Get(ctx, req.NamespacedName, bareMetalRemediation)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, bareMetalRemediation.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the BareMetalMachine instance.
	bareMetalMachine := &infrav1.HetznerBareMetalMachine{}

	key := client.ObjectKey{
		Name:      machine.Spec.InfrastructureRef.Name,
		Namespace: machine.Spec.InfrastructureRef.Namespace,
	}

	if err := r.Get(ctx, key, bareMetalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, bareMetalMachine) {
		log.Info("HCloudMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: bareMetalMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		log.Info("HetznerCluster is not available yet")
		return reconcile.Result{}, nil
	}

	// Create the scope.
	remediationScope, err := scope.NewBareMetalRemediationScope(ctx, scope.BareMetalRemediationScopeParams{
		Client:               r.Client,
		Logger:               &log,
		Machine:              machine,
		BareMetalMachine:     bareMetalMachine,
		HetznerCluster:       hetznerCluster,
		BareMetalRemediation: bareMetalRemediation,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any BareMetalRemediation changes.
	defer func() {
		// Always attempt to Patch the Remediation object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})

		if err := remediationScope.Close(ctx, patchOpts...); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !bareMetalRemediation.ObjectMeta.DeletionTimestamp.IsZero() {
		// Nothing to do
		return reconcile.Result{}, nil
	}

	return r.reconcileNormal(ctx, remediationScope)
}

func (r *HetznerBareMetalRemediationReconciler) reconcileNormal(ctx context.Context, remediationScope *scope.BareMetalRemediationScope) (reconcile.Result, error) {
	remediationScope.Info("Reconciling BareMetalRemediation")
	bareMetalRemediation := remediationScope.BareMetalRemediation

	// reconcile bare metal remediation
	if result, brk, err := breakReconcile(remediation.NewService(remediationScope).Reconcile(ctx)); brk {
		return result, errors.Wrapf(err, "failed to reconcile server for BareMetalRemediation %s/%s", bareMetalRemediation.Namespace, bareMetalRemediation.Name)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalRemediationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.HetznerBareMetalRemediation{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}
