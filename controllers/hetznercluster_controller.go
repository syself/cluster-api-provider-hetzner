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

// Package controllers implements controller types.
package controllers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/loadbalancer"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/network"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/placementgroup"
	certificatesv1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// HetznerClusterReconciler reconciles a HetznerCluster object.
type HetznerClusterReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	WatchFilterValue string

	targetClusterManagersCancelCtx map[types.NamespacedName]targetClusterManagerCancelCtx
	targetClusterManagersLock      sync.Mutex
}

type targetClusterManagerCancelCtx struct {
	Ctx      context.Context
	StopFunc context.CancelFunc
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	if r.targetClusterManagersCancelCtx == nil {
		r.targetClusterManagersCancelCtx = make(map[types.NamespacedName]targetClusterManagerCancelCtx)
	}

	controller, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HetznerCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(log)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	return controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			c, ok := o.(*clusterv1.Cluster)
			if !ok {
				panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
			}

			log := log.WithValues("objectMapper", "clusterToHetznerCluster", "namespace", c.Namespace, "cluster", c.Name)

			log.Info("Cluster spec", "spec", c.Spec)
			// Don't handle deleted clusters
			if !c.ObjectMeta.DeletionTimestamp.IsZero() {
				log.V(4).Info("Cluster has a deletion timestamp, skipping mapping.")
				return nil
			}

			// Make sure the ref is set
			if c.Spec.InfrastructureRef == nil {
				log.V(4).Info("Cluster does not have an InfrastructureRef, skipping mapping.")
				return nil
			}

			if c.Spec.InfrastructureRef.GroupVersionKind().Kind != "HetznerCluster" {
				log.V(4).Info("Cluster has an InfrastructureRef for a different type, skipping mapping.")
				return nil
			}

			hetznerCluster := &infrav1.HetznerCluster{}
			log.Info("Namespace", "ns", c.Spec.InfrastructureRef.Namespace)
			key := types.NamespacedName{Namespace: c.Spec.InfrastructureRef.Namespace, Name: c.Spec.InfrastructureRef.Name}

			if err := r.Get(ctx, key, hetznerCluster); err != nil {
				log.V(4).Error(err, "Failed to get HCloud cluster")
				return nil
			}

			if annotations.IsExternallyManaged(hetznerCluster) {
				log.V(4).Info("HetznerCluster is externally managed, skipping mapping.")
				return nil
			}

			log.V(4).Info("Adding request.", "hetznerCluster", c.Spec.InfrastructureRef.Name)
			return []ctrl.Request{
				{
					NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.InfrastructureRef.Name},
				},
			}
		}),
	)
}

//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters/finalizers,verbs=update

// Reconcile manages the lifecycle of a HetznerCluster object.
func (r *HetznerClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the HetznerCluster instance
	hetznerCluster := &infrav1.HetznerCluster{}
	err := r.Get(ctx, req.NamespacedName, hetznerCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("Starting reconciling cluster")

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, hetznerCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}

	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	if annotations.IsPaused(cluster, hetznerCluster) {
		log.Info("HetznerCluster or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	log.V(1).Info("Creating cluster scope")
	// Create the scope.
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Ctx:            ctx,
		Client:         r.Client,
		Logger:         log,
		Cluster:        cluster,
		HetznerCluster: hetznerCluster,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any HetznerCluster changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !hetznerCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	log.V(1).Info("Reconciling normal")
	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, clusterScope)
}

func (r *HetznerClusterReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("Reconciling HetznerCluster")

	hetznerCluster := clusterScope.HetznerCluster

	// If the HetznerCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(hetznerCluster, infrav1.ClusterFinalizer)
	if err := clusterScope.PatchObject(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// set failure domains in status using information in spec
	clusterScope.SetStatusFailureDomain(clusterScope.GetSpecRegion())

	// reconcile the network
	if err := network.NewService(clusterScope).Reconcile(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile network for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	// reconcile the load balancers
	if err := loadbalancer.NewService(clusterScope).Reconcile(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile load balancers for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	// reconcile the placement groups
	if err := placementgroup.NewService(clusterScope).Reconcile(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile placement groups for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	// In the case when the controlPlaneLoadBalancer has an IPv4 we use it as default for
	// the host of the controlPlaneEndpoint.
	// The first service of the loadBalancer is the kubeAPIService, from which we take the
	// destinationPort as default
	if hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4 != "<nil>" {
		var defaultHost = hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4
		var defaultPort = int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Services[0].DestinationPort)

		if hetznerCluster.Spec.ControlPlaneEndpoint == nil {
			hetznerCluster.Spec.ControlPlaneEndpoint = &clusterv1.APIEndpoint{
				Host: defaultHost,
				Port: defaultPort,
			}
		} else {
			if hetznerCluster.Spec.ControlPlaneEndpoint.Host == "" {
				hetznerCluster.Spec.ControlPlaneEndpoint.Host = defaultHost
			}
			if hetznerCluster.Spec.ControlPlaneEndpoint.Port == 0 {
				hetznerCluster.Spec.ControlPlaneEndpoint.Port = defaultPort
			}
		}

		// set cluster infrastructure as ready
		hetznerCluster.Status.Ready = true
	}

	// start targetClusterManager
	if err := r.reconcileTargetClusterManager(ctx, clusterScope); err != nil {
		return reconcile.Result{}, err
	}

	if err := reconcileTargetSecret(ctx, clusterScope); err != nil {
		log.Error(err, "failed to reconcile target secret")
	}

	log.V(1).Info("Reconciling finished")
	return reconcile.Result{}, nil
}

func (r *HetznerClusterReconciler) reconcileTargetClusterManager(inputctx context.Context, clusterScope *scope.ClusterScope) error {
	if len(clusterScope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target) == 0 {
		return nil
	}

	hetznerCluster := clusterScope.HetznerCluster
	deleted := !hetznerCluster.DeletionTimestamp.IsZero()

	r.targetClusterManagersLock.Lock()
	defer r.targetClusterManagersLock.Unlock()
	key := types.NamespacedName{
		Namespace: hetznerCluster.Namespace,
		Name:      hetznerCluster.Name,
	}
	if stopCtx, ok := r.targetClusterManagersCancelCtx[key]; !ok && !deleted {
		// create a new cluster manager
		m, err := r.newTargetClusterManager(inputctx, clusterScope)
		if err != nil {
			return errors.Wrapf(err, "failed to create a clusterManager for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
		}

		ctx, cancel := context.WithCancel(context.Background())

		r.targetClusterManagersCancelCtx[key] = targetClusterManagerCancelCtx{
			Ctx:      ctx,
			StopFunc: cancel,
		}

		go func() {
			if err := m.Start(ctx); err != nil {
				clusterScope.Error(err, "failed to start a targetClusterManager")
			} else {
				clusterScope.Info("stoppend targetClusterManager")
			}
			r.targetClusterManagersLock.Lock()
			defer r.targetClusterManagersLock.Unlock()
			delete(r.targetClusterManagersCancelCtx, key)
		}()
	} else if ok && deleted {
		stopCtx.StopFunc()
		delete(r.targetClusterManagersCancelCtx, key)
	}

	return nil
}

func (r *HetznerClusterReconciler) newTargetClusterManager(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Manager, error) {
	hetznerCluster := clusterScope.HetznerCluster

	clientConfig, err := clusterScope.ClientConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get a clientConfig for the API of HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get a restConfig for the API of HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get a clientSet for the API of HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	scheme := runtime.NewScheme()
	_ = certificatesv1.AddToScheme(scheme)

	clusterMgr, err := ctrl.NewManager(
		restConfig,
		ctrl.Options{
			Scheme:             scheme,
			MetricsBindAddress: "0",
			LeaderElection:     false,
		},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to setup guest cluster manager")
	}

	gr := &GuestCSRReconciler{
		Client: clusterMgr.GetClient(),
		mCluster: &managementCluster{
			Client:         r.Client,
			hetznerCluster: hetznerCluster,
		},
		WatchFilterValue: r.WatchFilterValue,
		clientSet:        clientSet,
	}

	if err := gr.SetupWithManager(ctx, clusterMgr, controller.Options{}); err != nil {
		return nil, errors.Wrapf(err, "failed to setup CSR controller")
	}

	return clusterMgr, nil
}

var _ ManagementCluster = &managementCluster{}

type managementCluster struct {
	client.Client
	hetznerCluster *infrav1.HetznerCluster
}

func (c *managementCluster) Namespace() string {
	return c.hetznerCluster.Namespace
}

func (r *HetznerClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling HetznerCluster delete")

	hetznerCluster := clusterScope.HetznerCluster

	// wait for all hcloudMachines to be deleted
	if machines, _, err := clusterScope.ListMachines(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to list machines for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	} else if len(machines) > 0 {
		var names []string
		for _, m := range machines {
			names = append(names, fmt.Sprintf("machine/%s", m.Name))
		}
		record.Eventf(
			hetznerCluster,
			"WaitingForMachineDeletion",
			"Machines %s still running, waiting with deletion of HetznerCluster",
			strings.Join(names, ", "),
		)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// delete load balancers
	if err := loadbalancer.NewService(clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to delete load balancers for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	// delete the network
	if err := network.NewService(clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to delete network for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	// delete the placement groups
	if err := placementgroup.NewService(clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to delete placement groups for HetznerCluster %s/%s", hetznerCluster.Namespace, hetznerCluster.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(clusterScope.HetznerCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func reconcileTargetSecret(ctx context.Context, clusterScope *scope.ClusterScope) error {
	if len(clusterScope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target) == 0 {
		return nil
	}

	log := ctrl.LoggerFrom(ctx)

	// Checking if control plane is ready
	clientConfig, err := clusterScope.ClientConfig()
	if err != nil {
		log.V(1).Info("failed to get clientconfig with api endpoint")
		return err
	}

	if err := scope.IsControlPlaneReady(ctx, clientConfig); err != nil {
		log.V(1).Info("Control plane not ready - reconcile target secret again")
		return err
	}

	// Control plane ready, so we can check if the secret exists already

	// getting client set
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	if _, err := clientSet.CoreV1().Secrets("kube-system").Get(
		ctx,
		clusterScope.HetznerCluster.Spec.HCloudTokenRef.Name,
		metav1.GetOptions{}); err != nil {
		// Set new secret as no secret was found
		if strings.HasSuffix(err.Error(), "not found") {
			var tokenSecret corev1.Secret
			tokenSecretName := types.NamespacedName{Namespace: clusterScope.HetznerCluster.Namespace, Name: clusterScope.HetznerCluster.Spec.HCloudTokenRef.Name}
			if err := clusterScope.Client.Get(ctx, tokenSecretName, &tokenSecret); err != nil {
				return errors.Errorf("error getting referenced token secret/%s: %s", tokenSecretName, err)
			}

			tokenBytes, keyExists := tokenSecret.Data[clusterScope.HetznerCluster.Spec.HCloudTokenRef.Key]
			if !keyExists {
				return errors.Errorf("error key %s does not exist in secret/%s", clusterScope.HetznerCluster.Spec.HCloudTokenRef.Key, tokenSecretName)
			}
			hetznerToken := string(tokenBytes)

			immutable := false
			data := make(map[string][]byte)
			data[clusterScope.HetznerCluster.Spec.HCloudTokenRef.Key] = []byte(hetznerToken)
			if clusterScope.HetznerCluster.Spec.NetworkSpec.NetworkEnabled {
				data["network"] = []byte(strconv.Itoa(clusterScope.HetznerCluster.Status.Network.ID))
			}
			data["apiserver-host"] = []byte(clusterScope.HetznerCluster.Spec.ControlPlaneEndpoint.Host)
			data["apiserver-port"] = []byte(strconv.Itoa(int(clusterScope.HetznerCluster.Spec.ControlPlaneEndpoint.Port)))

			newSecret := corev1.Secret{
				Immutable: &immutable,
				Data:      data,
				TypeMeta:  metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterScope.HetznerCluster.Spec.HCloudTokenRef.Name,
					Namespace: "kube-system",
				},
			}

			if _, err := clientSet.CoreV1().Secrets("kube-system").Create(ctx, &newSecret, metav1.CreateOptions{}); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// func (r *HetznerClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
// 	r.targetClusterManagersLock.Lock()
// 	defer r.targetClusterManagersLock.Unlock()

// 	var (
// 		controlledType     = &infrav1.HetznerCluster{}
// 		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
// 		controlledTypeGVK  = infrav1.GroupVersion.WithKind(controlledTypeName)
// 	)
// 	return ctrl.NewControllerManagedBy(mgr).
// 		WithOptions(options).
// 		For(controlledType).
// 		// Watch the CAPI resource that owns this infrastructure resource.
// 		Watches(
// 			&source.Kind{Type: &clusterv1.Cluster{}},
// 			handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(controlledTypeGVK)),
// 		).
// 		Complete(r)
// }
