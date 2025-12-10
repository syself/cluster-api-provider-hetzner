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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/conditions"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/loadbalancer"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/network"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/placementgroup"
)

var secretErrorRetryDelay = time.Second * 10

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
	DisableCSRApproval             bool

	// Reconcile only this namespace. Only needed for testing
	Namespace string
}

//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters/finalizers,verbs=update

// Reconcile manages the lifecycle of a HetznerCluster object.
func (r *HetznerClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	if r.Namespace != "" && req.Namespace != r.Namespace {
		// Just for testing, skip reconciling objects from finished tests.
		return ctrl.Result{}, nil
	}

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

		if err := clusterScope.Close(ctx); err != nil {
			res = reconcile.Result{}
			reterr = errors.Join(reterr, err)
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
	controllerutil.AddFinalizer(hetznerCluster, infrav1.HetznerClusterFinalizer)
	controllerutil.RemoveFinalizer(hetznerCluster, infrav1.DeprecatedHetznerClusterFinalizer)

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
			"%s",
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
			defaultPort := int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port) //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
			if defaultPort == 0 {
				defaultPort = infrav1.DefaultAPIServerPort
			}

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
			hetznerCluster.Status.Initialization.Provisioned = ptr.To(true)
			conditions.MarkTrue(hetznerCluster, clusterv1.InfrastructureReadyCondition)
			hetznerCluster.Status.Ready = true
		} else {
			const msg = "enabled LoadBalancer but load balancer not ready yet"
			conditions.MarkFalse(hetznerCluster,
				infrav1.ControlPlaneEndpointSetCondition,
				infrav1.ControlPlaneEndpointNotSetReason,
				clusterv1.ConditionSeverityWarning,
				msg)
			hetznerCluster.Status.Initialization.Provisioned = ptr.To(false)
			conditions.MarkFalse(hetznerCluster,
				clusterv1.InfrastructureReadyCondition,
				infrav1.ControlPlaneEndpointNotSetReason,
				clusterv1.ConditionSeverityWarning,
				msg)
			hetznerCluster.Status.Ready = false
		}
	} else {
		if hetznerCluster.Spec.ControlPlaneEndpoint != nil && hetznerCluster.Spec.ControlPlaneEndpoint.Host != "" && hetznerCluster.Spec.ControlPlaneEndpoint.Port != 0 {
			conditions.MarkTrue(hetznerCluster, infrav1.ControlPlaneEndpointSetCondition)
			hetznerCluster.Status.Initialization.Provisioned = ptr.To(true)
			conditions.MarkTrue(hetznerCluster, clusterv1.InfrastructureReadyCondition)
			hetznerCluster.Status.Ready = true
		} else {
			const msg = "disabled LoadBalancer and not yet provided ControlPlane endpoint"
			conditions.MarkFalse(hetznerCluster,
				infrav1.ControlPlaneEndpointSetCondition,
				infrav1.ControlPlaneEndpointNotSetReason,
				clusterv1.ConditionSeverityWarning,
				msg)
			hetznerCluster.Status.Initialization.Provisioned = ptr.To(false)
			conditions.MarkFalse(hetznerCluster,
				clusterv1.InfrastructureReadyCondition,
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
	controllerutil.RemoveFinalizer(clusterScope.HetznerCluster, infrav1.HetznerClusterFinalizer)
	controllerutil.RemoveFinalizer(clusterScope.HetznerCluster, infrav1.DeprecatedHetznerClusterFinalizer)

	return reconcile.Result{}, nil
}

// reconcileRateLimit checks whether a rate limit has been reached and returns whether
// the controller should wait a bit more.
func reconcileRateLimit(setter capiconditions.Setter, rateLimitWaitTime time.Duration) bool {
	condition := conditions.Get(setter, infrav1.HetznerAPIReachableCondition)
	if condition != nil && condition.Status == metav1.ConditionFalse {
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
	inerr error,
	setter capiconditions.Setter,
	conditionType string,
	clientObj client.Client,
) (ctrl.Result, error) {
	res := ctrl.Result{}
	switch inerr.(type) {
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
		inerr = nil

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
			"%s",
			inerr.Error(),
		)
		return reconcile.Result{}, fmt.Errorf("an unhandled failure occurred with the Hetzner secret: %w", inerr)
	}
	conditions.SetSummary(setter)

	obj, ok := setter.(client.Object)
	if !ok {
		return reconcile.Result{}, fmt.Errorf("setter does not implement client.Object")
	}

	if err := clientObj.Status().Update(ctx, obj); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to update: %w", err)
	}
	if inerr != nil {
		return reconcile.Result{}, inerr
	}
	return res, nil
}

func reconcileTargetSecret(ctx context.Context, clusterScope *scope.ClusterScope) (res reconcile.Result, reterr error) {
	if clusterScope.HetznerCluster.Spec.SkipCreatingHetznerSecretInWorkloadCluster {
		// If the secret should not be created in the workload cluster, we just return.
		// This means the ccm is running outside of the workload cluster (or getting the secret differently).
		return reconcile.Result{}, nil
	}

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
				"%s",
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

	if !r.DisableCSRApproval {
		gr := &GuestCSRReconciler{
			Client: clusterMgr.GetClient(),
			mCluster: &managementCluster{
				Client:         r.Client,
				hetznerCluster: hetznerCluster,
			},
			WatchFilterValue: r.WatchFilterValue,
			clientSet:        clientSet,
			clusterName:      clusterScope.Cluster.Name,
		}

		if err := gr.SetupWithManager(ctx, clusterMgr, controller.Options{
			// SkipNameValidation. Avoid this error: failed to setup CSR controller: controller with
			// name certificatesigningrequest already exists. Controller names must be unique to
			// avoid multiple controllers reporting the same metric. This validation can be disabled
			// via the SkipNameValidation option
			//
			// By default, controller names must be unique (to prevent duplicate Prometheus
			// metrics). In our case the name is not unique, because it gets executed for every
			// workload cluster.
			SkipNameValidation: ptr.To(true),
		}); err != nil {
			return nil, fmt.Errorf("failed to setup CSR controller: %w", err)
		}
	}

	return clusterMgr, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	if r.targetClusterManagersStopCh == nil {
		r.targetClusterManagersStopCh = make(map[types.NamespacedName]chan struct{})
	}

	err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HetznerCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(mgr.GetScheme(), log)).
		WithEventFilter(IgnoreInsignificantHetznerClusterStatusUpdates(log)).
		Owns(&corev1.Secret{}).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.clusterToHetznerCluster),
			builder.WithPredicates(IgnoreInsignificantClusterStatusUpdates(log)),
		).
		Complete(r)
	if err != nil {
		return fmt.Errorf("error creating controller: %w", err)
	}

	return nil
}

func (r *HetznerClusterReconciler) clusterToHetznerCluster(ctx context.Context, o client.Object) []reconcile.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	// Don't handle deleted clusters
	if !c.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	// Make sure the ref is set
	if !c.Spec.InfrastructureRef.IsDefined() {
		return nil
	}

	if c.Spec.InfrastructureRef.Kind != "HetznerCluster" || c.Spec.InfrastructureRef.APIGroup != infrav1.GroupVersion.Group {
		return nil
	}

	hetznerCluster := &infrav1.HetznerCluster{}
	key := types.NamespacedName{Namespace: c.Namespace, Name: c.Spec.InfrastructureRef.Name}

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
}

// IgnoreInsignificantClusterStatusUpdates is a predicate used for ignoring insignificant HetznerCluster.Status updates.
func IgnoreInsignificantClusterStatusUpdates(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := logger.WithValues(
				"predicate", "IgnoreInsignificantClusterStatusUpdates",
				"type", "update",
				"namespace", e.ObjectNew.GetNamespace(),
				"kind", strings.ToLower(e.ObjectNew.GetObjectKind().GroupVersionKind().Kind),
				"name", e.ObjectNew.GetName(),
			)

			var oldCluster, newCluster *clusterv1.Cluster
			var ok bool
			// This predicate only looks at Cluster objects
			if oldCluster, ok = e.ObjectOld.(*clusterv1.Cluster); !ok {
				return true
			}
			if newCluster, ok = e.ObjectNew.(*clusterv1.Cluster); !ok {
				// Something weird happened, and we received two different kinds of objects
				return true
			}

			// We should not modify the original objects, this causes issues with code that relies on the original object.
			oldCluster = oldCluster.DeepCopy()
			newCluster = newCluster.DeepCopy()

			// Set fields we do not care about to nil

			oldCluster.ManagedFields = nil
			newCluster.ManagedFields = nil

			oldCluster.ResourceVersion = ""
			newCluster.ResourceVersion = ""

			oldCluster.Status = clusterv1.ClusterStatus{}
			newCluster.Status = clusterv1.ClusterStatus{}

			if reflect.DeepEqual(oldCluster, newCluster) {
				// Only insignificant fields changed, no need to reconcile
				return false
			}
			// There is a noteworthy diff, so we should reconcile
			log.V(1).Info("Cluster -> HetznerCluster")
			return true
		},
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return true },
	}
}

// IgnoreInsignificantHetznerClusterStatusUpdates is a predicate used for ignoring insignificant HetznerHetznerCluster.Status updates.
func IgnoreInsignificantHetznerClusterStatusUpdates(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := logger.WithValues(
				"predicate", "IgnoreInsignificantHetznerClusterStatusUpdates",
				"namespace", e.ObjectNew.GetNamespace(),
				"kind", strings.ToLower(e.ObjectNew.GetObjectKind().GroupVersionKind().Kind),
				"name", e.ObjectNew.GetName(),
			)

			var oldHetznerCluster, newHetznerCluster *infrav1.HetznerCluster
			var ok bool
			// This predicate only looks at HetznerCluster objects
			if oldHetznerCluster, ok = e.ObjectOld.(*infrav1.HetznerCluster); !ok {
				return true
			}
			if newHetznerCluster, ok = e.ObjectNew.(*infrav1.HetznerCluster); !ok {
				// Something weird happened, and we received two different kinds of objects
				return true
			}

			// We should not modify the original objects, this causes issues with code that relies on the original object.
			oldHetznerCluster = oldHetznerCluster.DeepCopy()
			newHetznerCluster = newHetznerCluster.DeepCopy()

			// check if status is empty - if so, it should be restored
			emptyStatus := infrav1.HetznerClusterStatus{}
			if reflect.DeepEqual(newHetznerCluster.Status, emptyStatus) {
				return true
			}

			// Set fields we do not care about to nil

			oldHetznerCluster.ManagedFields = nil
			newHetznerCluster.ManagedFields = nil

			oldHetznerCluster.ResourceVersion = ""
			newHetznerCluster.ResourceVersion = ""

			oldHetznerCluster.Status = infrav1.HetznerClusterStatus{}
			newHetznerCluster.Status = infrav1.HetznerClusterStatus{}

			if reflect.DeepEqual(oldHetznerCluster, newHetznerCluster) {
				// Only insignificant fields changed, no need to reconcile
				return false
			}
			// There is a noteworthy diff, so we should reconcile
			log.V(1).Info("HetznerCluster Update")
			return true
		},
		// We only care about Update events, anything else should be reconciled
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return true },
	}
}
