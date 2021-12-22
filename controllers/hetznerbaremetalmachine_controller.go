/*
Copyright 2021.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/baremetal"
)

// HetznerBareMetalMachineReconciler reconciles a HetznerBareMetalMachine object
type HetznerBareMetalMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalMachine objects
func (r *HetznerBareMetalMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the Hetzner bare metal instance.
	hbmMachine := &infrav1.HetznerBareMetalMachine{}
	err := r.Get(ctx, req.NamespacedName, hbmMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, hbmMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, hbmMachine) {
		log.Info("HetznerBareMetalMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for Hetzner cluster controller to create cluster infrastructure")
		return ctrl.Result{}, nil
	}

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: hbmMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		log.Info("HetznerCluster is not available yet")
		return reconcile.Result{}, nil
	}

	// Create the scope.
	machineScope, err := scope.NewBareMetalMachineScope(ctx, scope.BareMetalMachineScopeParams{
		ClusterScopeParams: scope.ClusterScopeParams{
			Client:         r.Client,
			Logger:         &log,
			Cluster:        cluster,
			HetznerCluster: hetznerCluster,
		},
		Machine:          machine,
		BareMetalMachine: hbmMachine,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any HetznerBareMetalMachine changes.
	defer func() {
		if err := machineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !hbmMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}

	return r.reconcileNormal(ctx, machineScope)
}

func (r *HetznerBareMetalMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.BareMetalMachineScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling HetznerBareMetalMachine delete")
	hbmMachine := machineScope.BareMetalMachine

	// delete servers
	if result, brk, err := breakReconcile(baremetal.NewService(machineScope).Delete(ctx)); brk {
		return result, errors.Wrapf(err, "failed to delete servers for HetznerBareMetalMachine %s/%s", hbmMachine.Namespace, hbmMachine.Name)
	}

	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(machineScope.BareMetalMachine, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *HetznerBareMetalMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.BareMetalMachineScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling HetznerBareMetalMachine")
	hbmMachine := machineScope.BareMetalMachine

	// If the HetznerBareMetalMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machineScope.BareMetalMachine, infrav1.MachineFinalizer)

	// Register the finalizer immediately to avoid orphaning HetznerBareMetal resources
	// on delete
	if err := machineScope.PatchObject(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// reconcile server
	if result, brk, err := breakReconcile(baremetal.NewService(machineScope).Reconcile(ctx)); brk {
		return result, errors.Wrapf(err, "failed to reconcile server for HetznerBareMetalMachine %s/%s", hbmMachine.Namespace, hbmMachine.Name)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.HetznerBareMetalMachine{}).
		Complete(r)
}
