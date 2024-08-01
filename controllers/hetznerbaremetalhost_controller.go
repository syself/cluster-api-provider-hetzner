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
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	bmclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/host"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// HetznerBareMetalHostReconciler reconciles a HetznerBareMetalHost object.
type HetznerBareMetalHostReconciler struct {
	client.Client
	RateLimitWaitTime  time.Duration
	APIReader          client.Reader
	RobotClientFactory robotclient.Factory
	SSHClientFactory   sshclient.Factory
	WatchFilterValue   string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalHost objects.
func (r *HetznerBareMetalHostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	// Fetch the Hetzner bare metal host instance.
	bmHost := &infrav1.HetznerBareMetalHost{}
	err := r.Get(ctx, req.NamespacedName, bmHost)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Add a finalizer to newly created objects.
	if bmHost.DeletionTimestamp.IsZero() && !hostHasFinalizer(bmHost) {
		bmHost.Finalizers = append(bmHost.Finalizers,
			infrav1.BareMetalHostFinalizer)
		err := r.Update(ctx, bmHost)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Certain cases need to be handled here and not later in the host state machine.
	// If res != nil, then we should return, otherwise not.
	res, err = r.reconcileSelectedStates(ctx, bmHost)
	if err != nil {
		return reconcile.Result{}, err
	}
	emptyResult := reconcile.Result{}
	if res != emptyResult {
		return res, nil
	}

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: bmHost.Namespace,
		Name:      bmHost.Spec.Status.HetznerClusterRef,
	}
	if bmHost.Spec.Status.HetznerClusterRef == "" {
		log.Info("bmHost.Spec.Status.HetznerClusterRef is empty. Looks like a stale cache read")
		return reconcile.Result{Requeue: true}, nil
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get HetznerCluster: %w", err)
	}

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, hetznerCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get Cluster: %w", err)
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	hetznerBareMetalMachine := &infrav1.HetznerBareMetalMachine{}

	if bmHost.Spec.ConsumerRef != nil {
		name := client.ObjectKey{
			Namespace: bmHost.Spec.ConsumerRef.Namespace,
			Name:      bmHost.Spec.ConsumerRef.Name,
		}

		if err := r.Client.Get(ctx, name, hetznerBareMetalMachine); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get HetznerBareMetalMachine: %w", err)
		}
	}

	log = log.WithValues("HetznerBareMetalMachine", klog.KObj(hetznerBareMetalMachine))

	ctx = ctrl.LoggerInto(ctx, log)

	// Get Hetzner robot api credentials
	secretManager := secretutil.NewSecretManager(log, r.Client, r.APIReader)
	robotCreds, err := getAndValidateRobotCredentials(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hetznerSecretErrorResult(ctx, err, bmHost, r.Client)
	}

	// Get secrets. Return when result != nil.
	osSSHSecret, rescueSSHSecret, res, err := r.getSecrets(ctx, *secretManager, bmHost, hetznerCluster)
	if err != nil {
		return reconcile.Result{}, err
	}
	if res != emptyResult {
		return res, nil
	}

	// Create the scope.
	hostScope, err := scope.NewBareMetalHostScope(scope.BareMetalHostScopeParams{
		Logger:                  log,
		Client:                  r.Client,
		HetznerCluster:          hetznerCluster,
		Cluster:                 cluster,
		HetznerBareMetalHost:    bmHost,
		HetznerBareMetalMachine: hetznerBareMetalMachine,
		RobotClient:             r.RobotClientFactory.NewClient(robotCreds),
		SSHClientFactory:        r.SSHClientFactory,
		OSSSHSecret:             osSSHSecret,
		RescueSSHSecret:         rescueSSHSecret,
		SecretManager:           secretManager,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(bmHost, r.RateLimitWaitTime); wait {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return r.reconcile(ctx, hostScope)
}

func (r *HetznerBareMetalHostReconciler) reconcile(
	ctx context.Context,
	hostScope *scope.BareMetalHostScope,
) (reconcile.Result, error) {
	result, err := host.NewService(hostScope).Reconcile(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to reconcile HetznerBareMetalHost %s/%s: %w",
			hostScope.HetznerBareMetalHost.Namespace, hostScope.HetznerBareMetalHost.Name, err)
	}
	return result, nil
}

func (r *HetznerBareMetalHostReconciler) reconcileSelectedStates(ctx context.Context, bmHost *infrav1.HetznerBareMetalHost) (res ctrl.Result, err error) {
	switch bmHost.Spec.Status.ProvisioningState {
	// Handle StateNone: check whether needs to be provisioned or deleted.
	case infrav1.StateNone:
		var needsUpdate bool
		if !bmHost.DeletionTimestamp.IsZero() && bmHost.Spec.ConsumerRef == nil {
			bmHost.Spec.Status.ProvisioningState = infrav1.StateDeleting
			needsUpdate = true
		} else if bmHost.NeedsProvisioning() {
			bmHost.Spec.Status.ProvisioningState = infrav1.StatePreparing
			needsUpdate = true
		}
		if needsUpdate {
			err := r.Update(ctx, bmHost)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
			}
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil

		// Handle StateDeleting
	case infrav1.StateDeleting:
		if !utils.StringInList(bmHost.Finalizers, infrav1.BareMetalHostFinalizer) {
			return reconcile.Result{Requeue: true}, nil
		}

		bmHost.Finalizers = utils.FilterStringFromList(bmHost.Finalizers, infrav1.BareMetalHostFinalizer)
		if err := r.Update(ctx, bmHost); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
		return reconcile.Result{Requeue: true}, nil
	}
	return res, nil
}

func (r *HetznerBareMetalHostReconciler) getSecrets(
	ctx context.Context,
	secretManager secretutil.SecretManager,
	bmHost *infrav1.HetznerBareMetalHost,
	hetznerCluster *infrav1.HetznerCluster,
) (
	osSSHSecret *corev1.Secret,
	rescueSSHSecret *corev1.Secret,
	res ctrl.Result,
	reterr error,
) {
	emptyResult := reconcile.Result{}
	if bmHost.Spec.Status.SSHSpec != nil {
		var err error
		osSSHSecretNamespacedName := types.NamespacedName{Namespace: bmHost.Namespace, Name: bmHost.Spec.Status.SSHSpec.SecretRef.Name}
		osSSHSecret, err = secretManager.ObtainSecret(ctx, osSSHSecretNamespacedName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("%s: %s", infrav1.ErrorMessageMissingOSSSHSecret, err.Error())
				conditions.MarkFalse(
					bmHost,
					infrav1.CredentialsAvailableCondition,
					infrav1.OSSSHSecretMissingReason,
					clusterv1.ConditionSeverityError,
					msg,
				)
				record.Warnf(bmHost, infrav1.OSSSHSecretMissingReason, msg)
				conditions.SetSummary(bmHost)
				result, err := host.SaveHostAndReturn(ctx, r.Client, bmHost)
				if result != emptyResult || err != nil {
					return nil, nil, result, err
				}

				return nil, nil, reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
			}
			return nil, nil, res, fmt.Errorf("failed to get secret: %w", err)
		}

		rescueSSHSecretNamespacedName := types.NamespacedName{Namespace: bmHost.Namespace, Name: hetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name}
		rescueSSHSecret, err = secretManager.AcquireSecret(ctx, rescueSSHSecretNamespacedName, hetznerCluster, false, hetznerCluster.DeletionTimestamp.IsZero())
		if err != nil {
			if apierrors.IsNotFound(err) {
				conditions.MarkFalse(
					bmHost,
					infrav1.CredentialsAvailableCondition,
					infrav1.RescueSSHSecretMissingReason,
					clusterv1.ConditionSeverityError,
					infrav1.ErrorMessageMissingRescueSSHSecret,
				)

				record.Warnf(bmHost, infrav1.RescueSSHSecretMissingReason, infrav1.ErrorMessageMissingRescueSSHSecret)
				conditions.SetSummary(bmHost)
				result, err := host.SaveHostAndReturn(ctx, r.Client, bmHost)
				if result != emptyResult || err != nil {
					return nil, nil, result, err
				}

				return nil, nil, reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
			}
			return nil, nil, res, fmt.Errorf("failed to acquire secret: %w", err)
		}
	}
	return osSSHSecret, rescueSSHSecret, res, nil
}

func getAndValidateRobotCredentials(
	ctx context.Context,
	namespace string,
	hetznerCluster *infrav1.HetznerCluster,
	secretManager *secretutil.SecretManager,
) (robotclient.Credentials, error) {
	secretNamspacedName := types.NamespacedName{Namespace: namespace, Name: hetznerCluster.Spec.HetznerSecret.Name}

	hetznerSecret, err := secretManager.AcquireSecret(
		ctx,
		secretNamspacedName,
		hetznerCluster,
		false,
		false,
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return robotclient.Credentials{},
				&secretutil.ResolveSecretRefError{Message: fmt.Sprintf("The Hetzner secret %s does not exist", secretNamspacedName)}
		}
		return robotclient.Credentials{}, err
	}

	creds := robotclient.Credentials{
		Username: string(hetznerSecret.Data[hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser]),
		Password: string(hetznerSecret.Data[hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword]),
	}

	// Validate token
	if creds.Username == "" {
		return robotclient.Credentials{}, &bmclient.CredentialsValidationError{
			Message: fmt.Sprintf("secret %s/%s: Missing Hetzner robot api connection detail '%s' in credentials",
				namespace, hetznerCluster.Spec.HetznerSecret.Name, hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser),
		}
	}
	if creds.Password == "" {
		return robotclient.Credentials{}, &bmclient.CredentialsValidationError{
			Message: fmt.Sprintf("secret %s/%s: Missing Hetzner robot api connection detail '%s' in credentials",
				namespace, hetznerCluster.Spec.HetznerSecret.Name, hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword),
		}
	}

	return creds, nil
}

func hetznerSecretErrorResult(
	ctx context.Context,
	err error,
	bmHost *infrav1.HetznerBareMetalHost,
	client client.Client,
) (res ctrl.Result, reterr error) {
	resolveErr := &secretutil.ResolveSecretRefError{}
	if errors.As(err, &resolveErr) {
		// In the event that the reference to the secret is defined, but we cannot find it
		// we requeue the host as we will not know if they create the secret
		// at some point in the future.
		conditions.MarkFalse(
			bmHost,
			infrav1.CredentialsAvailableCondition,
			infrav1.HetznerSecretUnreachableReason,
			clusterv1.ConditionSeverityError,
			infrav1.ErrorMessageMissingHetznerSecret,
		)

		record.Warnf(bmHost, infrav1.HetznerSecretUnreachableReason, fmt.Sprintf("%s: %s", infrav1.ErrorMessageMissingHetznerSecret, err.Error()))
		conditions.SetSummary(bmHost)
		result, err := host.SaveHostAndReturn(ctx, client, bmHost)
		if err != nil {
			return reconcile.Result{}, err
		}
		emptyResult := reconcile.Result{}
		if result != emptyResult {
			return result, nil
		}

		// No need to reconcile again, as it will be triggered as soon as the secret is updated.
		return res, nil
	}

	credValidationErr := &bmclient.CredentialsValidationError{}
	if errors.As(err, &credValidationErr) {
		conditions.MarkFalse(
			bmHost,
			infrav1.CredentialsAvailableCondition,
			infrav1.RobotCredentialsInvalidReason,
			clusterv1.ConditionSeverityError,
			infrav1.ErrorMessageMissingOrInvalidSecretData,
		)
		record.Warnf(bmHost, infrav1.SSHCredentialsInSecretInvalidReason, err.Error())
		conditions.SetSummary(bmHost)
		return host.SaveHostAndReturn(ctx, client, bmHost)
	}
	return reconcile.Result{}, fmt.Errorf("hetznerSecretErrorResult: an unhandled failure occurred: %T %w", err, err)
}

func hostHasFinalizer(host *infrav1.HetznerBareMetalHost) bool {
	return utils.StringInList(host.Finalizers, infrav1.BareMetalHostFinalizer)
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalHostReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HetznerBareMetalHost{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					objectOld, oldOK := e.ObjectOld.(*infrav1.HetznerBareMetalHost)
					objectNew, newOK := e.ObjectNew.(*infrav1.HetznerBareMetalHost)

					if !(oldOK && newOK) {
						// The thing that changed wasn't a host, so we
						// need to assume that we must update. This
						// happens when, for example, an owned Secret
						// changes.
						return true
					}

					// If provisioning state changes, then we want to reconcile
					if objectOld.Spec.Status.ProvisioningState != objectNew.Spec.Status.ProvisioningState {
						return true
					}

					// If install image changes, then we want to reconcile, as this is important when working with bm machines
					if !reflect.DeepEqual(objectOld.Spec.Status.InstallImage, objectNew.Spec.Status.InstallImage) {
						return true
					}

					// Take updates of finalizers or annotations
					if !reflect.DeepEqual(objectNew.GetFinalizers(), objectOld.GetFinalizers()) ||
						!reflect.DeepEqual(objectNew.GetAnnotations(), objectOld.GetAnnotations()) {
						return true
					}

					objectO := objectOld.DeepCopy()
					objectN := objectNew.DeepCopy()
					objectO.Spec.Status = infrav1.ControllerGeneratedStatus{}
					objectN.Spec.Status = infrav1.ControllerGeneratedStatus{}

					// We can ignore changes only in status or spec.status. We can ignore this
					return !reflect.DeepEqual(objectO.Spec, objectN.Spec)
				},
			}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
