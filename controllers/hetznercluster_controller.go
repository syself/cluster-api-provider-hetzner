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

// Package controllers implements controller types.
package controllers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/loadbalancer"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/network"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/placementgroup"
)

const (
	secretErrorRetryDelay = time.Second * 10
)

// HetznerClusterReconciler reconciles a HetznerCluster object.
type HetznerClusterReconciler struct {
	client.Client
	RateLimitWaitTime              time.Duration
	APIReader                      client.Reader
	HCloudClientFactory            hcloudclient.Factory
	targetClusterManagersStopCh    map[types.NamespacedName]chan struct{}
	targetClusterManagersLock      sync.Mutex
	TargetClusterManagersWaitGroup *sync.WaitGroup
	WatchFilterValue               string
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

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, hetznerCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get owner cluster: %w", err)
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

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

	// Create the scope.
	secretManager := secretutil.NewSecretManager(log, r.Client, r.APIReader)
	hcloudToken, hetznerSecret, err := getAndValidateHCloudToken(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hcloudTokenErrorResult(ctx, err, hetznerCluster, infrav1.HCloudTokenAvailableCondition, r.Client)
	}
	hcloudClient := r.HCloudClientFactory.NewClient(hcloudToken)

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:         r.Client,
		APIReader:      r.APIReader,
		Logger:         log,
		Cluster:        cluster,
		HetznerCluster: hetznerCluster,
		HCloudClient:   hcloudClient,
		HetznerSecret:  hetznerSecret,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Always close the scope when exiting this function so we can persist any HetznerCluster changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			conditions.MarkFalse(hetznerCluster, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudCredentialsInvalidReason, clusterv1.ConditionSeverityError, "wrong hcloud token")
		} else {
			conditions.MarkTrue(hetznerCluster, infrav1.HCloudTokenAvailableCondition)
		}

		if err := clusterScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// delete the deprecated condition from existing cluster objects
	conditions.Delete(hetznerCluster, infrav1.DeprecatedRateLimitExceededCondition)

	// check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(hetznerCluster, r.RateLimitWaitTime); wait {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Handle deleted clusters
	if !hetznerCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, clusterScope)
}

func (r *HetznerClusterReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	hetznerCluster := clusterScope.HetznerCluster

	// If the HetznerCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(hetznerCluster, infrav1.ClusterFinalizer)
	if err := clusterScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// set failure domains in status using information in spec
	clusterScope.SetStatusFailureDomain(clusterScope.GetSpecRegion())

	// reconcile the network
	if err := network.NewService(clusterScope).Reconcile(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile network for HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	emptyResult := reconcile.Result{}

	// reconcile the load balancers
	res, err := loadbalancer.NewService(clusterScope).Reconcile(ctx)
	if res != emptyResult {
		return res, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile load balancers for HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	// reconcile the placement groups
	if err := placementgroup.NewService(clusterScope).Reconcile(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile placement groups for HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	processControlPlaneEndpoint(hetznerCluster)

	// delete deprecated conditions of old clusters
	conditions.Delete(clusterScope.HetznerCluster, infrav1.DeprecatedHetznerClusterTargetClusterReadyCondition)

	result, err := r.reconcileTargetClusterManager(ctx, clusterScope)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile target cluster manager: %w", err)
	}
	if result != emptyResult {
		return result, nil
	}

	// target cluster is ready
	conditions.MarkTrue(hetznerCluster, infrav1.TargetClusterReadyCondition)

	result, err = reconcileTargetSecret(ctx, clusterScope)
	if err != nil {
		reterr := fmt.Errorf("failed to reconcile target secret: %w", err)
		conditions.MarkFalse(
			clusterScope.HetznerCluster,
			infrav1.TargetClusterSecretReadyCondition,
			infrav1.TargetSecretSyncFailedReason,
			clusterv1.ConditionSeverityError,
			reterr.Error(),
		)
		return reconcile.Result{}, reterr
	}
	if result != emptyResult {
		return result, nil
	}

	// target cluster secret is ready
	conditions.MarkTrue(hetznerCluster, infrav1.TargetClusterSecretReadyCondition)

	return reconcile.Result{}, nil
}

func processControlPlaneEndpoint(hetznerCluster *infrav1.HetznerCluster) {
	if hetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled {
		if hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4 != "<nil>" {
			defaultHost := hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4
			defaultPort := int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port)

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
			conditions.MarkTrue(hetznerCluster, infrav1.ControlPlaneEndpointSetCondition)
			hetznerCluster.Status.Ready = true
		} else {
			msg := "enabled LoadBalancer but load balancer not ready yet"
			conditions.MarkFalse(hetznerCluster,
				infrav1.ControlPlaneEndpointSetCondition,
				infrav1.ControlPlaneEndpointNotSetReason,
				clusterv1.ConditionSeverityWarning,
				msg)
			hetznerCluster.Status.Ready = false
		}
	} else {
		if hetznerCluster.Spec.ControlPlaneEndpoint != nil && hetznerCluster.Spec.ControlPlaneEndpoint.Host != "" && hetznerCluster.Spec.ControlPlaneEndpoint.Port != 0 {
			conditions.MarkTrue(hetznerCluster, infrav1.ControlPlaneEndpointSetCondition)
			hetznerCluster.Status.Ready = true
		} else {
			msg := "disabled LoadBalancer and not yet provided ControlPlane endpoint"
			conditions.MarkFalse(hetznerCluster,
				infrav1.ControlPlaneEndpointSetCondition,
				infrav1.ControlPlaneEndpointNotSetReason,
				clusterv1.ConditionSeverityWarning,
				msg)
			hetznerCluster.Status.Ready = false
		}
	}
}

func (r *HetznerClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	hetznerCluster := clusterScope.HetznerCluster

	// wait for all hcloudMachines to be deleted
	machines, _, err := clusterScope.ListMachines(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list machines for HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}
	if len(machines) > 0 {
		names := make([]string, len(machines))
		for i, m := range machines {
			names[i] = fmt.Sprintf("machine/%s", m.Name)
		}
		record.Eventf(
			hetznerCluster,
			"WaitingForMachineDeletion",
			"Machines %s still running, waiting with deletion of HetznerCluster",
			strings.Join(names, ", "),
		)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	secretManager := secretutil.NewSecretManager(clusterScope.Logger, r.Client, r.APIReader)
	// Remove finalizer of secret
	if err := secretManager.ReleaseSecret(ctx, clusterScope.HetznerSecret(), clusterScope.HetznerCluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to release Hetzner secret: %w", err)
	}

	// Check if rescue ssh secret exists and release it if yes
	if hetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name != "" {
		rescueSSHSecretObjectKey := client.ObjectKey{Name: hetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name, Namespace: hetznerCluster.Namespace}
		rescueSSHSecret, err := secretManager.ObtainSecret(ctx, rescueSSHSecretObjectKey)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("failed to get Rescue SSH secret: %w", err)
			}
		}
		if rescueSSHSecret != nil {
			if err := secretManager.ReleaseSecret(ctx, rescueSSHSecret, clusterScope.HetznerCluster); err != nil {
				if !apierrors.IsNotFound(err) {
					return reconcile.Result{}, fmt.Errorf("failed to release Rescue SSH secret: %w", err)
				}
			}
		}
	}

	// delete load balancers
	if err := loadbalancer.NewService(clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete load balancers for HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	// delete the network
	if err := network.NewService(clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete network for HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	// delete the placement groups
	if err := placementgroup.NewService(clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete placement groups for HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	// Stop CSR manager
	r.targetClusterManagersLock.Lock()
	defer r.targetClusterManagersLock.Unlock()

	key := types.NamespacedName{
		Namespace: clusterScope.HetznerCluster.Namespace,
		Name:      clusterScope.HetznerCluster.Name,
	}
	if stopCh, ok := r.targetClusterManagersStopCh[key]; ok {
		close(stopCh)
		delete(r.targetClusterManagersStopCh, key)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(clusterScope.HetznerCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

// reconcileRateLimit checks whether a rate limit has been reached and returns whether
// the controller should wait a bit more.
func reconcileRateLimit(setter conditions.Setter, rateLimitWaitTime time.Duration) bool {
	condition := conditions.Get(setter, infrav1.HetznerAPIReachableCondition)
	if condition != nil && condition.Status == corev1.ConditionFalse {
		if time.Now().Before(condition.LastTransitionTime.Time.Add(rateLimitWaitTime)) {
			// Not yet timed out, reconcile again after timeout
			// Don't give a more precise requeueAfter value to not reconcile too many
			// objects at the same time
			return true
		}
		// Wait time is over, we continue
		conditions.MarkTrue(setter, infrav1.HetznerAPIReachableCondition)
	}
	return false
}

func getAndValidateHCloudToken(ctx context.Context, namespace string, hetznerCluster *infrav1.HetznerCluster, secretManager *secretutil.SecretManager) (string, *corev1.Secret, error) {
	// retrieve Hetzner secret
	secretNamspacedName := types.NamespacedName{Namespace: namespace, Name: hetznerCluster.Spec.HetznerSecret.Name}

	hetznerSecret, err := secretManager.AcquireSecret(
		ctx,
		secretNamspacedName,
		hetznerCluster,
		false,
		hetznerCluster.DeletionTimestamp.IsZero(),
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil, &secretutil.ResolveSecretRefError{Message: fmt.Sprintf("The Hetzner secret %s does not exist", secretNamspacedName)}
		}
		return "", nil, err
	}

	hcloudToken := string(hetznerSecret.Data[hetznerCluster.Spec.HetznerSecret.Key.HCloudToken])

	// Validate token
	if hcloudToken == "" {
		return "", nil, &secretutil.HCloudTokenValidationError{}
	}

	return hcloudToken, hetznerSecret, nil
}

func hcloudTokenErrorResult(
	ctx context.Context,
	err error,
	setter conditions.Setter,
	conditionType clusterv1.ConditionType,
	client client.Client,
) (res ctrl.Result, reterr error) {
	switch err.(type) {
	// In the event that the reference to the secret is defined, but we cannot find it
	// we requeue the host as we will not know if they create the secret
	// at some point in the future.
	case *secretutil.ResolveSecretRefError:
		conditions.MarkFalse(setter,
			conditionType,
			infrav1.HetznerSecretUnreachableReason,
			clusterv1.ConditionSeverityError,
			"could not find HetznerSecret",
		)
		res = ctrl.Result{RequeueAfter: secretErrorRetryDelay}

	// No need to reconcile again, as it will be triggered as soon as the secret is updated.
	case *secretutil.HCloudTokenValidationError:
		conditions.MarkFalse(setter,
			conditionType,
			infrav1.HCloudCredentialsInvalidReason,
			clusterv1.ConditionSeverityError,
			"invalid or not specified hcloud token in Hetzner secret",
		)

	default:
		conditions.MarkFalse(setter,
			conditionType,
			infrav1.HCloudCredentialsInvalidReason,
			clusterv1.ConditionSeverityError,
			err.Error(),
		)
		return reconcile.Result{}, fmt.Errorf("an unhandled failure occurred with the Hetzner secret: %w", err)
	}
	conditions.SetSummary(setter)
	if err := client.Status().Update(ctx, setter); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to update: %w", err)
	}

	return res, err
}

func reconcileTargetSecret(ctx context.Context, clusterScope *scope.ClusterScope) (res reconcile.Result, reterr error) {
	// Checking if control plane is ready
	clientConfig, err := clusterScope.ClientConfig(ctx)
	if err != nil {
		clusterScope.V(1).Info("failed to get clientconfig with api endpoint")
		return reconcile.Result{}, err
	}

	if err := scope.IsControlPlaneReady(ctx, clientConfig); err != nil {
		conditions.MarkFalse(
			clusterScope.HetznerCluster,
			infrav1.TargetClusterSecretReadyCondition,
			infrav1.TargetClusterControlPlaneNotReadyReason,
			clusterv1.ConditionSeverityInfo,
			"target cluster not ready",
		)
		return reconcile.Result{Requeue: true}, nil //nolint:nilerr
	}

	// Control plane ready, so we can check if the secret exists already

	// getting client
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get rest config: %w", err)
	}

	client, err := client.New(restConfig, client.Options{})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get client: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterScope.HetznerCluster.Spec.HetznerSecret.Name,
			Namespace: metav1.NamespaceSystem,
		},
	}

	// Make sure secret exists and has the expected values
	_, err = controllerutil.CreateOrUpdate(ctx, client, secret, func() error {
		tokenSecretName := types.NamespacedName{
			Namespace: clusterScope.HetznerCluster.Namespace,
			Name:      clusterScope.HetznerCluster.Spec.HetznerSecret.Name,
		}
		secretManager := secretutil.NewSecretManager(clusterScope.Logger, clusterScope.Client, clusterScope.APIReader)
		tokenSecret, err := secretManager.AcquireSecret(ctx, tokenSecretName, clusterScope.HetznerCluster, false, clusterScope.HetznerCluster.DeletionTimestamp.IsZero())
		if err != nil {
			return fmt.Errorf("failed to acquire secret: %w", err)
		}

		hetznerToken, keyExists := tokenSecret.Data[clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HCloudToken]
		if !keyExists {
			return fmt.Errorf("error key %s does not exist in secret/%s: %w",
				clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HCloudToken,
				tokenSecretName,
				err,
			)
		}

		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		secret.Data[clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HCloudToken] = hetznerToken

		// Save robot credentials if available
		if clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser != "" {
			robotUserName := tokenSecret.Data[clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser]
			secret.Data[clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser] = robotUserName
			robotPassword := tokenSecret.Data[clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword]
			secret.Data[clusterScope.HetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword] = robotPassword
		}

		// Save network ID in secret
		if clusterScope.HetznerCluster.Spec.HCloudNetwork.Enabled {
			secret.Data["network"] = []byte(strconv.FormatInt(clusterScope.HetznerCluster.Status.Network.ID, 10))
		}
		// Save api server information
		secret.Data["apiserver-host"] = []byte(clusterScope.HetznerCluster.Spec.ControlPlaneEndpoint.Host)
		secret.Data["apiserver-port"] = []byte(strconv.Itoa(int(clusterScope.HetznerCluster.Spec.ControlPlaneEndpoint.Port)))

		return nil
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create or update secret: %w", err)
	}

	return res, nil
}

func (r *HetznerClusterReconciler) reconcileTargetClusterManager(ctx context.Context, clusterScope *scope.ClusterScope) (res reconcile.Result, err error) {
	r.targetClusterManagersLock.Lock()
	defer r.targetClusterManagersLock.Unlock()

	key := types.NamespacedName{
		Namespace: clusterScope.HetznerCluster.Namespace,
		Name:      clusterScope.HetznerCluster.Name,
	}

	if _, ok := r.targetClusterManagersStopCh[key]; !ok {
		// create a new cluster manager
		m, err := r.newTargetClusterManager(ctx, clusterScope)
		if err != nil {
			conditions.MarkFalse(
				clusterScope.HetznerCluster,
				infrav1.TargetClusterReadyCondition,
				infrav1.TargetClusterCreateFailedReason,
				clusterv1.ConditionSeverityError,
				err.Error(),
			)

			return reconcile.Result{}, fmt.Errorf("failed to create a clusterManager for HetznerCluster %s/%s: %w",
				clusterScope.HetznerCluster.Namespace,
				clusterScope.HetznerCluster.Name,
				err,
			)
		}

		// manager could not be created yet - reconcile again
		if m == nil {
			return reconcile.Result{Requeue: true}, nil
		}

		r.targetClusterManagersStopCh[key] = make(chan struct{})

		ctx, cancel := context.WithCancel(ctx)

		r.TargetClusterManagersWaitGroup.Add(1)

		// Start manager
		go func() {
			defer r.TargetClusterManagersWaitGroup.Done()

			if err := m.Start(ctx); err != nil {
				clusterScope.Error(err, "failed to start a targetClusterManager")
				conditions.MarkFalse(
					clusterScope.HetznerCluster,
					infrav1.TargetClusterReadyCondition,
					infrav1.TargetClusterCreateFailedReason,
					clusterv1.ConditionSeverityError,
					"failed to start a targetClusterManager: %s", err.Error(),
				)
			} else {
				clusterScope.Info("stop targetClusterManager")
			}
			r.targetClusterManagersLock.Lock()
			defer r.targetClusterManagersLock.Unlock()
			delete(r.targetClusterManagersStopCh, key)
		}()

		// Cancel when stop channel received input
		go func() {
			<-r.targetClusterManagersStopCh[key]
			cancel()
		}()
	}
	return res, nil
}

var _ ManagementCluster = &managementCluster{}

type managementCluster struct {
	client.Client
	hetznerCluster *infrav1.HetznerCluster
}

func (c *managementCluster) Namespace() string {
	return c.hetznerCluster.Namespace
}

func (r *HetznerClusterReconciler) newTargetClusterManager(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Manager, error) {
	hetznerCluster := clusterScope.HetznerCluster

	clientConfig, err := clusterScope.ClientConfig(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditions.MarkFalse(
				hetznerCluster,
				infrav1.TargetClusterReadyCondition,
				infrav1.KubeConfigNotFoundReason,
				clusterv1.ConditionSeverityInfo,
				"kubeconfig not found (yet)",
			)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get a clientConfig for the API of HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	if err := scope.IsControlPlaneReady(ctx, clientConfig); err != nil {
		conditions.MarkFalse(
			clusterScope.HetznerCluster,
			infrav1.TargetClusterReadyCondition,
			infrav1.TargetClusterControlPlaneNotReadyReason,
			clusterv1.ConditionSeverityInfo,
			"target cluster not ready",
		)
		return nil, nil //nolint:nilerr
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get a restConfig for the API of HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get a clientSet for the API of HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	scheme := runtime.NewScheme()
	_ = certificatesv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get an HTTP client for the API of HetznerCluster %s/%s: %w", hetznerCluster.Namespace, hetznerCluster.Name, err)
	}

	// Check whether kubeapi server responds
	if _, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient); err != nil {
		conditions.MarkFalse(
			hetznerCluster,
			infrav1.TargetClusterReadyCondition,
			infrav1.KubeAPIServerNotRespondingReason,
			clusterv1.ConditionSeverityInfo,
			"kubeapi server not responding (yet)",
		)
		return nil, nil //nolint:nilerr
	}

	clusterMgr, err := ctrl.NewManager(
		restConfig,
		ctrl.Options{
			Scheme:         scheme,
			LeaderElection: false,
			Metrics:        metricsserver.Options{BindAddress: "0"},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to setup guest cluster manager: %w", err)
	}

	hasConstantBareMetalHostname := clusterScope.Cluster.Annotations[infrav1.ConstantBareMetalHostnameAnnotation] == "true"

	gr := &GuestCSRReconciler{
		Client: clusterMgr.GetClient(),
		mCluster: &managementCluster{
			Client:         r.Client,
			hetznerCluster: hetznerCluster,
		},
		WatchFilterValue:             r.WatchFilterValue,
		clientSet:                    clientSet,
		clusterName:                  clusterScope.Cluster.Name,
		hasConstantBareMetalHostname: hasConstantBareMetalHostname,
	}

	if err := gr.SetupWithManager(ctx, clusterMgr, controller.Options{}); err != nil {
		return nil, fmt.Errorf("failed to setup CSR controller: %w", err)
	}

	return clusterMgr, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	if r.targetClusterManagersStopCh == nil {
		r.targetClusterManagersStopCh = make(map[types.NamespacedName]chan struct{})
	}

	controller, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HetznerCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(log)).
		Owns(&corev1.Secret{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("error creating controller: %w", err)
	}

	return controller.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			c, ok := o.(*clusterv1.Cluster)
			if !ok {
				panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
			}

			// Don't handle deleted clusters
			if !c.ObjectMeta.DeletionTimestamp.IsZero() {
				return nil
			}

			// Make sure the ref is set
			if c.Spec.InfrastructureRef == nil {
				return nil
			}

			if c.Spec.InfrastructureRef.GroupVersionKind().Kind != "HetznerCluster" {
				return nil
			}

			hetznerCluster := &infrav1.HetznerCluster{}
			key := types.NamespacedName{Namespace: c.Spec.InfrastructureRef.Namespace, Name: c.Spec.InfrastructureRef.Name}

			if err := r.Get(ctx, key, hetznerCluster); err != nil {
				return nil
			}

			if annotations.IsExternallyManaged(hetznerCluster) {
				return nil
			}

			return []ctrl.Request{
				{
					NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.InfrastructureRef.Name},
				},
			}
		}),
	)
}
