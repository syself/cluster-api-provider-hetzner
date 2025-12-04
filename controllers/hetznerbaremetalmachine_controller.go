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

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/conditions"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/baremetal"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
)

// HetznerBareMetalMachineReconciler reconciles a HetznerBareMetalMachine object.
type HetznerBareMetalMachineReconciler struct {
	client.Client
	APIReader           client.Reader
	RateLimitWaitTime   time.Duration
	HCloudClientFactory hcloudclient.Factory
	WatchFilterValue    string

	// Reconcile only this namespace. Only needed for testing
	Namespace string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalMachine objects.
func (r *HetznerBareMetalMachineReconciler) Reconcile(ctx context.Context, req reconcile.Request) (res reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	if r.Namespace != "" && req.Namespace != r.Namespace {
		// Just for testing, skip reconciling objects from finished tests.
		return ctrl.Result{}, nil
	}

	// Fetch the Hetzner bare metal instance.
	hbmMachine := &infrav1.HetznerBareMetalMachine{}
	err := r.Get(ctx, req.NamespacedName, hbmMachine)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("HetznerBareMetalMachine", klog.KObj(hbmMachine))

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, hbmMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get owner machine. BareMetalMachine.ObjectMeta.OwnerReferences %v: %w",
			hbmMachine.ObjectMeta.OwnerReferences, err)
	}
	if capiMachine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Machine", klog.KObj(capiMachine))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capiMachine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	if annotations.IsPaused(cluster, hbmMachine) {
		log.Info("HetznerBareMetalMachine or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: hbmMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		return reconcile.Result{}, nil
	}

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Create the scope.
	secretManager := secretutil.NewSecretManager(log, r.Client, r.APIReader)
	hcloudToken, _, err := getAndValidateHCloudToken(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hcloudTokenErrorResult(ctx, err, hbmMachine, infrav1.HCloudTokenAvailableCondition, r.Client)
	}

	hcc := r.HCloudClientFactory.NewClient(hcloudToken)

	// Create the scope.
	machineScope, err := scope.NewBareMetalMachineScope(scope.BareMetalMachineScopeParams{
		Client:           r.Client,
		Logger:           log,
		Machine:          capiMachine,
		BareMetalMachine: hbmMachine,
		HetznerCluster:   hetznerCluster,
		HCloudClient:     hcc,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Always close the scope when exiting this function so we can persist any HetznerBareMetalMachine changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			conditions.MarkFalse(hbmMachine, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudCredentialsInvalidReason, clusterv1.ConditionSeverityError, "wrong hcloud token")
		} else {
			conditions.MarkTrue(hbmMachine, infrav1.HCloudTokenAvailableCondition)
		}

		conditions.SetSummary(hbmMachine)

		if err := machineScope.Close(ctx); err != nil {
			res = reconcile.Result{}
			reterr = errors.Join(reterr, err)
		}
	}()

	// check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(hbmMachine, r.RateLimitWaitTime); wait {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !hbmMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}

	deleteConditionOfHbmm := conditions.Get(hbmMachine, infrav1.DeleteMachineSucceededCondition)
	if deleteConditionOfHbmm != nil && deleteConditionOfHbmm.Status == corev1.ConditionFalse {
		// The hbmm will be deleted. Do not reconcile it.
		log.Info("hbmm has DeleteMachineSucceededCondition=False. Waiting for deletion")
		return reconcile.Result{}, nil
	}

	return r.reconcileNormal(ctx, machineScope)
}

func (r *HetznerBareMetalMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.BareMetalMachineScope) (reconcile.Result, error) {
	// delete servers
	result, err := baremetal.NewService(machineScope).Delete(ctx)
	if err != nil {
		var requeueError *scope.RequeueAfterError
		if ok := errors.As(err, &requeueError); ok {
			return reconcile.Result{Requeue: true, RequeueAfter: requeueError.GetRequeueAfter()}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to delete servers for HetznerBareMetalMachine %s/%s: %w",
			machineScope.BareMetalMachine.Namespace, machineScope.BareMetalMachine.Name, err)
	}
	emptyResult := reconcile.Result{}
	if result != emptyResult {
		return result, nil
	}
	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(machineScope.BareMetalMachine, infrav1.HetznerBareMetalMachineFinalizer)
	controllerutil.RemoveFinalizer(machineScope.BareMetalMachine, infrav1.DeprecatedBareMetalMachineFinalizer)

	return result, nil
}

func (r *HetznerBareMetalMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.BareMetalMachineScope) (reconcile.Result, error) {
	// If the HetznerBareMetalMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machineScope.BareMetalMachine, infrav1.HetznerBareMetalMachineFinalizer)
	controllerutil.RemoveFinalizer(machineScope.BareMetalMachine, infrav1.DeprecatedBareMetalMachineFinalizer)

	// Register the finalizer immediately to avoid orphaning HetznerBareMetal resources on delete
	if err := machineScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// reconcile server
	result, err := baremetal.NewService(machineScope).Reconcile(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile server for HetznerBareMetalMachine %s/%s: %w",
			machineScope.BareMetalMachine.Namespace, machineScope.BareMetalMachine.Name, err)
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	clusterToObjectFunc, err := util.ClusterToTypedObjectsMapper(r.Client, &infrav1.HetznerBareMetalMachineList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for Cluster to BareMetalMachines: %w", err)
	}
	err = ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HetznerBareMetalMachine{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue)).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("HetznerBareMetalMachine"))),
		).
		Watches(
			&infrav1.HetznerCluster{},
			handler.EnqueueRequestsFromMapFunc(r.HetznerClusterToBareMetalMachines(ctx, log)),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToBareMetalMachines(ctx, log)),
		).
		Watches(
			&infrav1.HetznerBareMetalHost{},
			handler.EnqueueRequestsFromMapFunc(BareMetalHostToBareMetalMachines(r.Client, log)),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureProvisioned(mgr.GetScheme(), log)),
		).
		Complete(r)
	if err != nil {
		return fmt.Errorf("error creating controller: %w", err)
	}

	return nil
}

// HetznerClusterToBareMetalMachines is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of BareMetalMachines.
func (r *HetznerBareMetalMachineReconciler) HetznerClusterToBareMetalMachines(ctx context.Context, log logr.Logger) handler.MapFunc {
	return func(_ context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		c, ok := o.(*infrav1.HetznerCluster)
		if !ok {
			log.Error(fmt.Errorf("expected a HetznerCluster but got a %T", o),
				"failed to get BareMetalMachine for HetznerCluster")
			return nil
		}

		log := log.WithValues("objectMapper", "hetznerClusterToBareMetalMachine", "namespace", c.Namespace, "hetznerCluster", c.Name)

		// Don't handle deleted HetznerCluster
		if !c.ObjectMeta.DeletionTimestamp.IsZero() {
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			log.V(1).Info("Cluster for HetznerCluster not found, skipping mapping")
			return result
		case err != nil:
			log.Error(err, "failed to get owning cluster, skipping mapping")
			return result
		}

		labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
		machineList := &clusterv1.MachineList{}
		if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			log.Error(err, "failed to list Machines, skipping mapping")
			return nil
		}
		for _, m := range machineList.Items {
			infraRef := m.Spec.InfrastructureRef
			if !infraRef.IsDefined() {
				continue
			}
			if infraRef.Kind != "HetznerBareMetalMachine" || infraRef.APIGroup != infrav1.GroupVersion.Group {
				continue
			}
			if infraRef.Name == "" {
				continue
			}

			name := client.ObjectKey{Namespace: m.Namespace, Name: infraRef.Name}

			result = append(result, reconcile.Request{NamespacedName: name})
		}

		return result
	}
}

// ClusterToBareMetalMachines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of BareMetalMachines.
func (r *HetznerBareMetalMachineReconciler) ClusterToBareMetalMachines(ctx context.Context, log logr.Logger) handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		c, ok := obj.(*clusterv1.Cluster)

		if !ok {
			log.Error(fmt.Errorf("expected a Cluster but got a %T", obj),
				"failed to get BareMetalMachine for Cluster")
			return nil
		}

		labels := map[string]string{clusterv1.ClusterNameLabel: c.Name}
		capiMachineList := &clusterv1.MachineList{}
		if err := r.Client.List(ctx, capiMachineList, client.InNamespace(c.Namespace),
			client.MatchingLabels(labels),
		); err != nil {
			log.Error(err, "failed to list BareMetalMachines")
			return nil
		}
		for _, m := range capiMachineList.Items {
			infraRef := m.Spec.InfrastructureRef
			if !infraRef.IsDefined() || infraRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: infraRef.Name}
			result = append(result, reconcile.Request{NamespacedName: name})
		}

		return result
	}
}

// BareMetalHostToBareMetalMachines will return a reconcile request for a BareMetalMachine if the event is for a
// BareMetalHost and that BareMetalHost references a BareMetalMachine.
func BareMetalHostToBareMetalMachines(c client.Client, log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		host, ok := obj.(*infrav1.HetznerBareMetalHost)
		if !ok {
			log.Error(fmt.Errorf("expected a BareMetalHost but got a %T", obj),
				"failed to get BareMetalMachine for BareMetalHost")
			return nil
		}

		// If this host has a consumerRef (hbmm), then reconcile the corresponding hbmm.
		if host.Spec.ConsumerRef != nil {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      host.Spec.ConsumerRef.Name,
						Namespace: host.Spec.ConsumerRef.Namespace,
					},
				},
			}
		}

		if host.Spec.Status.ErrorType != "" {
			return []reconcile.Request{}
		}

		// We have a free host. Trigger a matching HetznerBareMetalMachine to be reconciled.
		hbmmList := infrav1.HetznerBareMetalMachineList{}
		err := c.List(ctx, &hbmmList, client.InNamespace(host.Namespace))
		if err != nil {
			log.Error(err, "failed to list HetznerBareMetalMachines")
			return []reconcile.Request{}
		}

		// Search for a machines which would like to use this host.
		var found []reconcile.Request
		for i := range hbmmList.Items {
			hbmm := &hbmmList.Items[i]

			// Skip if the hbmm is already in use.
			if hbmm.HasHostAnnotation() {
				continue
			}

			hosts := []infrav1.HetznerBareMetalHost{*host}
			chosenHost, _, err := baremetal.ChooseHost(hbmm, hosts)
			if err != nil {
				log.Error(err, "failed to choose host for HetznerBareMetalMachine")
				continue
			}

			// this hbmm does not match the host
			if chosenHost == nil {
				continue
			}

			// We found a matching hbmm for the free host. Trigger Reconcile for the hbmm.
			found = append(found, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      hbmm.Name,
					Namespace: hbmm.Namespace,
				},
			})
		}
		return found
	}
}
