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

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/packer"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/server"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// HCloudMachineReconciler reconciles a HCloudMachine object.
type HCloudMachineReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	WatchFilterValue string
	Packer           *packer.Packer
}

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines/finalizers,verbs=update

// Reconcile manages the lifecycle of an HCloud machine object.
func (r *HCloudMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the HCloudMachine instance.
	hcloudMachine := &infrav1.HCloudMachine{}
	err := r.Get(ctx, req.NamespacedName, hcloudMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize Packer
	if !hcloudMachine.Status.ImageInitialized {
		if err := r.Packer.Initialize(hcloudMachine); err != nil {
			log.Error(err, "unable to initialise packer manager")
			return reconcile.Result{}, err
		}
		hcloudMachine.Status.ImageInitialized = true
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, hcloudMachine.ObjectMeta)
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

	if annotations.IsPaused(cluster, hcloudMachine) {
		log.Info("HCloudMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: hcloudMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		log.Info("HetznerCluster is not available yet")
		return reconcile.Result{}, nil
	}

	// Create the scope.
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		ClusterScopeParams: scope.ClusterScopeParams{
			Ctx:            ctx,
			Client:         r.Client,
			Logger:         log,
			Cluster:        cluster,
			HetznerCluster: hetznerCluster,
			Packer:         r.Packer,
		},
		Machine:       machine,
		HCloudMachine: hcloudMachine,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any HCloudMachine changes.
	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !hcloudMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}

	return r.reconcileNormal(ctx, machineScope)
}

func breakReconcile(ctrl *reconcile.Result, err error) (reconcile.Result, bool, error) {
	c := reconcile.Result{}
	if ctrl != nil {
		c = *ctrl
	}
	return c, ctrl != nil || err != nil, err
}

func (r *HCloudMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling HCloudMachine delete")
	hcloudMachine := machineScope.HCloudMachine

	// delete servers
	if result, brk, err := breakReconcile(server.NewService(machineScope).Delete(ctx)); brk {
		return result, errors.Wrapf(err, "failed to delete servers for HCloudMachine %s/%s", hcloudMachine.Namespace, hcloudMachine.Name)
	}

	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(machineScope.HCloudMachine, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *HCloudMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.MachineScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling HCloudMachine")
	hcloudMachine := machineScope.HCloudMachine

	// If the HCloudMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machineScope.HCloudMachine, infrav1.MachineFinalizer)

	// Register the finalizer immediately to avoid orphaning HCloud resources
	// on delete
	if err := machineScope.PatchObject(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// reconcile server
	if result, brk, err := breakReconcile(server.NewService(machineScope).Reconcile(ctx)); brk {
		return result, errors.Wrapf(err, "failed to reconcile server for HCloudMachine %s/%s", hcloudMachine.Namespace, hcloudMachine.Name)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HCloudMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HCloudMachine{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("HCloudMachine"))),
		).
		Watches(
			&source.Kind{Type: &infrav1.HetznerCluster{}},
			handler.EnqueueRequestsFromMapFunc(r.HetznerClusterToHCloudMachines(ctx)),
		).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	clusterToObjectFunc, err := util.ClusterToObjectsMapper(r.Client, &infrav1.HCloudMachineList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrap(err, "failed to create mapper for Cluster to HCloudMachines")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// HetznerClusterToHCloudMachines is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of HCloudMachines.
func (r *HCloudMachineReconciler) HetznerClusterToHCloudMachines(ctx context.Context) handler.MapFunc {
	return func(o client.Object) []ctrl.Request {
		result := []ctrl.Request{}

		log := log.FromContext(ctx)

		c, ok := o.(*infrav1.HetznerCluster)
		if !ok {
			log.Error(errors.Errorf("expected a HetznerCluster but got a %T", o), "failed to get HCloudMachine for HetznerCluster")
			return nil
		}

		log = log.WithValues("objectMapper", "hetznerClusterToHCloudMachine", "namespace", c.Namespace, "hetznerCluster", c.Name)

		// Don't handle deleted HetznerCluster
		if !c.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("HetznerCluster has a deletion timestamp, skipping mapping.")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			log.V(4).Info("Cluster for HetznerCluster not found, skipping mapping.")
			return result
		case err != nil:
			log.Error(err, "failed to get owning cluster, skipping mapping.")
			return result
		}

		labels := map[string]string{clusterv1.ClusterLabelName: cluster.Name}
		machineList := &clusterv1.MachineList{}
		if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			log.Error(err, "failed to list Machines, skipping mapping.")
			return nil
		}
		for _, m := range machineList.Items {
			log.WithValues("machine", m.Name)
			if m.Spec.InfrastructureRef.GroupVersionKind().Kind != "HCloudMachine" {
				log.V(4).Info("Machine has an InfrastructureRef for a different type, will not add to reconciliation request.")
				continue
			}
			if m.Spec.InfrastructureRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
			log.WithValues("hcloudMachine", name.Name)
			log.V(4).Info("Adding HCloudMachine to reconciliation request.")
			result = append(result, ctrl.Request{NamespacedName: name})
		}

		return result
	}
}
