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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/baremetal"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

// HetznerBareMetalMachineReconciler reconciles a HetznerBareMetalMachine object.
type HetznerBareMetalMachineReconciler struct {
	client.Client
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalMachine objects.
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

	if hbmMachine.Status.FailureMessage != nil {
		return ctrl.Result{}, nil
	}
	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, hbmMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get owner machine. BareMetalMachine.ObjectMeta.OwnerReferences: %v", hbmMachine.ObjectMeta.OwnerReferences)
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
		Client:           r.Client,
		Logger:           &log,
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
	// delete servers
	if result, brk, err := breakReconcile(baremetal.NewService(machineScope).Delete(ctx)); brk {
		if requeueErr, ok := errors.Cause(err).(scope.HasRequeueAfterError); ok {
			return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
		}
		return result, errors.Wrapf(
			err,
			"failed to delete servers for HetznerBareMetalMachine %s/%s",
			machineScope.BareMetalMachine.Namespace,
			machineScope.BareMetalMachine.Name,
		)
	}

	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(machineScope.BareMetalMachine, infrav1.BareMetalMachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *HetznerBareMetalMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.BareMetalMachineScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling HetznerBareMetalMachine")

	// If the HetznerBareMetalMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machineScope.BareMetalMachine, infrav1.BareMetalMachineFinalizer)

	// Register the finalizer immediately to avoid orphaning HetznerBareMetal resources on delete
	if err := machineScope.PatchObject(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// reconcile server
	if result, brk, err := breakReconcile(baremetal.NewService(machineScope).Reconcile(ctx)); brk {
		return result, errors.Wrapf(
			err,
			"failed to reconcile server for HetznerBareMetalMachine %s/%s",
			machineScope.BareMetalMachine.Namespace,
			machineScope.BareMetalMachine.Name,
		)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HetznerBareMetalMachine{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("HetznerBareMetalMachine"))),
		).
		Watches(
			&source.Kind{Type: &infrav1.HetznerCluster{}},
			handler.EnqueueRequestsFromMapFunc(r.HetznerClusterToBareMetalMachines(ctx)),
		).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToBareMetalMachines(ctx, log)),
		).
		Watches(
			&source.Kind{Type: &infrav1.HetznerBareMetalHost{}},
			handler.EnqueueRequestsFromMapFunc(r.BareMetalHostToBareMetalMachines(log)),
		).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	clusterToObjectFunc, err := util.ClusterToObjectsMapper(r.Client, &infrav1.HetznerBareMetalMachineList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrap(err, "failed to create mapper for Cluster to BareMetalMachines")
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

// HetznerClusterToBareMetalMachines is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of BareMetalMachines.
func (r *HetznerBareMetalMachineReconciler) HetznerClusterToBareMetalMachines(ctx context.Context) handler.MapFunc {
	return func(o client.Object) []ctrl.Request {
		result := []ctrl.Request{}

		log := log.FromContext(ctx)

		c, ok := o.(*infrav1.HetznerCluster)
		if !ok {
			log.Error(errors.Errorf("expected a HetznerCluster but got a %T", o), "failed to get BareMetalMachine for HetznerCluster")
			return nil
		}

		log = log.WithValues("objectMapper", "hetznerClusterToBareMetalMachine", "namespace", c.Namespace, "hetznerCluster", c.Name)

		// Don't handle deleted HetznerCluster
		if !c.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(1).Info("HetznerCluster has a deletion timestamp, skipping mapping.")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			log.V(1).Info("Cluster for HetznerCluster not found, skipping mapping.")
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
			if m.Spec.InfrastructureRef.GroupVersionKind().Kind != "HetznerBareMetalMachine" {
				log.V(1).Info("Machine has an InfrastructureRef for a different type, will not add to reconciliation request.")
				continue
			}
			if m.Spec.InfrastructureRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
			log.WithValues("bareMetalMachine", name.Name)
			log.V(1).Info("Adding BareMetalMachine to reconciliation request.")
			result = append(result, ctrl.Request{NamespacedName: name})
		}

		return result
	}
}

// ClusterToBareMetalMachines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of BareMetalMachines.
func (r *HetznerBareMetalMachineReconciler) ClusterToBareMetalMachines(ctx context.Context, log logr.Logger) handler.MapFunc {
	return func(obj client.Object) []ctrl.Request {
		result := []ctrl.Request{}
		c, ok := obj.(*clusterv1.Cluster)

		if !ok {
			log.Error(errors.Errorf("expected a Cluster but got a %T", obj),
				"failed to get BareMetalMachine for Cluster",
			)
			return nil
		}

		labels := map[string]string{clusterv1.ClusterLabelName: c.Name}
		capiMachineList := &clusterv1.MachineList{}
		if err := r.Client.List(ctx, capiMachineList, client.InNamespace(c.Namespace),
			client.MatchingLabels(labels),
		); err != nil {
			log.Error(err, "failed to list BareMetalMachines")
			return nil
		}
		for _, m := range capiMachineList.Items {
			if m.Spec.InfrastructureRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
			if m.Spec.InfrastructureRef.Namespace != "" {
				name = client.ObjectKey{Namespace: m.Spec.InfrastructureRef.Namespace, Name: m.Spec.InfrastructureRef.Name}
			}
			result = append(result, ctrl.Request{NamespacedName: name})
		}

		return result
	}
}

// BareMetalHostToBareMetalMachines will return a reconcile request for a BareMetalMachine if the event is for a
// BareMetalHost and that BareMetalHost references a BareMetalMachine.
func (r *HetznerBareMetalMachineReconciler) BareMetalHostToBareMetalMachines(log logr.Logger) handler.MapFunc {
	return func(obj client.Object) []ctrl.Request {
		if host, ok := obj.(*infrav1.HetznerBareMetalHost); ok {
			if host.Spec.ConsumerRef != nil &&
				host.Spec.ConsumerRef.Kind == "HetznerBareMetalMachine" &&
				host.Spec.ConsumerRef.GroupVersionKind().Group == infrav1.GroupVersion.Group {
				return []ctrl.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      host.Spec.ConsumerRef.Name,
							Namespace: host.Spec.ConsumerRef.Namespace,
						},
					},
				}
			}
		} else {
			log.Error(errors.Errorf("expected a BareMetalHost but got a %T", obj),
				"failed to get BareMetalMachine for BareMetalHost",
			)
		}
		return []ctrl.Request{}
	}
}
