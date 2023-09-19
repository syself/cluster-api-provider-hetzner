/*
Copyright 2021 The Kubernetes Authors.

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
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/server"
)

// HCloudMachineReconciler reconciles a HCloudMachine object.
type HCloudMachineReconciler struct {
	client.Client
	RateLimitWaitTime   time.Duration
	APIReader           client.Reader
	HCloudClientFactory hcloudclient.Factory
	WatchFilterValue    string
}

//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines/finalizers,verbs=update

// Reconcile manages the lifecycle of an HCloud machine object.
func (r *HCloudMachineReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the HCloudMachine instance.
	hcloudMachine := &infrav1.HCloudMachine{}
	err := r.Get(ctx, req.NamespacedName, hcloudMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("HCloudMachine", klog.KObj(hcloudMachine))

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, hcloudMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Machine", klog.KObj(machine))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
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
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		log.Info("HetznerCluster is not available yet")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Create the scope.
	secretManager := secretutil.NewSecretManager(log, r.Client, r.APIReader)
	hcloudToken, hetznerSecret, err := getAndValidateHCloudToken(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hcloudTokenErrorResult(ctx, err, hcloudMachine, infrav1.HCloudTokenAvailableCondition, r.Client)
	}

	hcc := r.HCloudClientFactory.NewClient(hcloudToken)

	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		ClusterScopeParams: scope.ClusterScopeParams{
			Client:         r.Client,
			Logger:         log,
			Cluster:        cluster,
			HetznerCluster: hetznerCluster,
			HCloudClient:   hcc,
			HetznerSecret:  hetznerSecret,
			APIReader:      r.APIReader,
		},
		Machine:       machine,
		HCloudMachine: hcloudMachine,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any HCloudMachine changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			conditions.MarkFalse(hcloudMachine, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudCredentialsInvalidReason, clusterv1.ConditionSeverityError, "wrong hcloud token")
		} else {
			conditions.MarkTrue(hcloudMachine, infrav1.HCloudTokenAvailableCondition)
		}

		if err := machineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(hcloudMachine, r.RateLimitWaitTime); wait {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !hcloudMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}

	return r.reconcileNormal(ctx, machineScope)
}

func (r *HCloudMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope) (reconcile.Result, error) {
	hcloudMachine := machineScope.HCloudMachine

	// Delete servers.
	result, err := server.NewService(machineScope).Delete(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to delete servers for HCloudMachine %s/%s: %w", hcloudMachine.Namespace, hcloudMachine.Name, err)
	}
	emptyResult := reconcile.Result{}
	if result != emptyResult {
		return result, nil
	}
	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(machineScope.HCloudMachine, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *HCloudMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.MachineScope) (reconcile.Result, error) {
	hcloudMachine := machineScope.HCloudMachine

	// If the HCloudMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machineScope.HCloudMachine, infrav1.MachineFinalizer)

	// Register the finalizer immediately to avoid orphaning HCloud resources on delete.
	if err := machineScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// reconcile server
	result, err := server.NewService(machineScope).Reconcile(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to reconcile server for HCloudMachine %s/%s: %w",
			hcloudMachine.Namespace, hcloudMachine.Name, err)
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HCloudMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HCloudMachine{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("HCloudMachine"))),
		).
		Watches(
			&infrav1.HetznerCluster{},
			handler.EnqueueRequestsFromMapFunc(r.HetznerClusterToHCloudMachines(ctx)),
		).
		Build(r)
	if err != nil {
		return fmt.Errorf("error creating controller: %w", err)
	}

	clusterToObjectFunc, err := util.ClusterToObjectsMapper(r.Client, &infrav1.HCloudMachineList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for Cluster to HCloudMachines: %w", err)
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return fmt.Errorf("failed adding a watch for ready clusters: %w", err)
	}

	return nil
}

// HetznerClusterToHCloudMachines is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of HCloudMachines.
func (r *HCloudMachineReconciler) HetznerClusterToHCloudMachines(_ context.Context) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		log := log.FromContext(ctx)

		c, ok := o.(*infrav1.HetznerCluster)
		if !ok {
			log.Error(fmt.Errorf("expected a HetznerCluster but got a %T", o), "failed to get HCloudMachine for HetznerCluster")
			return nil
		}

		log = log.WithValues("objectMapper", "hetznerClusterToHCloudMachine", "namespace", c.Namespace, "hetznerCluster", c.Name)

		// Don't handle deleted HetznerCluster
		if !c.ObjectMeta.DeletionTimestamp.IsZero() {
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			return result
		case err != nil:
			return result
		}

		labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
		machineList := &clusterv1.MachineList{}
		if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			log.Error(err, "failed to list Machines, skipping mapping")
			return nil
		}
		for _, m := range machineList.Items {
			log = log.WithValues("machine", m.Name)
			if m.Spec.InfrastructureRef.GroupVersionKind().Kind != "HCloudMachine" {
				continue
			}
			if m.Spec.InfrastructureRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}

			result = append(result, reconcile.Request{NamespacedName: name})
		}

		return result
	}
}
