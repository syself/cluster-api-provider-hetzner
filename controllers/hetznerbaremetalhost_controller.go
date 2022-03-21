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
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	bmclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/host"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// HetznerBareMetalHostReconciler reconciles a HetznerBareMetalHost object.
type HetznerBareMetalHostReconciler struct {
	client.Client
	APIReader          client.Reader
	RobotClientFactory robotclient.Factory
	SSHClientFactory   sshclient.Factory
	WatchFilterValue   string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalHost objects.
func (r *HetznerBareMetalHostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal host", "name", req.Name)

	// Fetch the Hetzner bare metal host instance.
	bmHost := &infrav1.HetznerBareMetalHost{}
	err := r.Get(ctx, req.NamespacedName, bmHost)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: bmHost.Namespace,
		Name:      bmHost.Spec.HetznerClusterRef,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		return ctrl.Result{}, errors.New("HetznerCluster not found")
	}

	// Add a finalizer to newly created objects.
	if bmHost.DeletionTimestamp.IsZero() && !hostHasFinalizer(bmHost) {
		log.Info("adding finalizer", "existingFinalizers", bmHost.Finalizers, "newValue", infrav1.BareMetalHostFinalizer)
		bmHost.Finalizers = append(bmHost.Finalizers,
			infrav1.BareMetalHostFinalizer)
		err := r.Update(ctx, bmHost)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get Hetzner robot api credentials
	secretManager := secretutil.NewSecretManager(log, r.Client, r.APIReader)
	robotCreds, err := getAndValidateRobotCredentials(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hetznerSecretErrorResult(ctx, err, bmHost, r.Client)
	}

	// If bare metal machine has already set ssh spec, then acquire os ssh secret
	var osSSHSecret *corev1.Secret
	if bmHost.Spec.Status.SSHSpec != nil {
		osSSHSecretNamespacedName := types.NamespacedName{Namespace: req.Namespace, Name: bmHost.Spec.Status.SSHSpec.SecretRef.Name}
		osSSHSecret, err = secretManager.ObtainSecret(ctx, osSSHSecretNamespacedName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				if err := host.SetErrorCondition(
					ctx,
					bmHost,
					r.Client,
					infrav1.PreparationError,
					infrav1.ErrorMessageMissingOSSSHSecret,
				); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: host.CalculateBackoff(bmHost.Spec.Status.ErrorCount)}, nil
			}
			return reconcile.Result{}, errors.Wrap(err, "failed to get secret")
		}
	}

	rescueSSHSecretNamespacedName := types.NamespacedName{Namespace: req.Namespace, Name: hetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name}
	rescueSSHSecret, err := secretManager.AcquireSecret(ctx, rescueSSHSecretNamespacedName, hetznerCluster, false, hetznerCluster.DeletionTimestamp.IsZero())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := host.SetErrorCondition(
				ctx,
				bmHost,
				r.Client,
				infrav1.PreparationError,
				infrav1.ErrorMessageMissingRescueSSHSecret,
			); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: host.CalculateBackoff(bmHost.Spec.Status.ErrorCount)}, nil
		}
		return reconcile.Result{}, errors.Wrap(err, "failed to acquire secret")
	}

	// Create the scope.
	hostScope, err := scope.NewBareMetalHostScope(ctx, scope.BareMetalHostScopeParams{
		Logger:               &log,
		Client:               r.Client,
		HetznerCluster:       hetznerCluster,
		HetznerBareMetalHost: bmHost,
		RobotClient:          r.RobotClientFactory.NewClient(robotCreds),
		SSHClientFactory:     r.SSHClientFactory,
		OSSSHSecret:          osSSHSecret,
		RescueSSHSecret:      rescueSSHSecret,
		SecretManager:        secretManager,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	if !bmHost.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, hostScope)
	}

	return r.reconcileNormal(ctx, hostScope)
}

func (r *HetznerBareMetalHostReconciler) reconcileDelete(
	ctx context.Context,
	hostScope *scope.BareMetalHostScope,
) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling HetznerBareMetalHost delete")

	if result, brk, err := breakReconcile(host.NewService(hostScope).Delete(ctx)); brk {
		return result, errors.Wrapf(
			err,
			"failed to delete HetznerBareMetalHost %s/%s",
			hostScope.HetznerBareMetalHost.Namespace,
			hostScope.HetznerBareMetalHost.Name,
		)
	}
	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(hostScope.HetznerBareMetalHost, infrav1.BareMetalHostFinalizer)
	if err := r.Update(ctx, hostScope.HetznerBareMetalHost); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to remove finalizer")
	}
	return reconcile.Result{}, nil
}

func (r *HetznerBareMetalHostReconciler) reconcileNormal(
	ctx context.Context,
	hostScope *scope.BareMetalHostScope,
) (reconcile.Result, error) {
	if result, brk, err := breakReconcile(host.NewService(hostScope).Reconcile(ctx)); brk {
		return result, errors.Wrapf(
			err,
			"failed to reconcile HetznerBareMetalHost %s/%s",
			hostScope.HetznerBareMetalHost.Namespace,
			hostScope.HetznerBareMetalHost.Name,
		)
	}
	return reconcile.Result{}, nil
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
	if err := creds.Validate(); err != nil {
		return robotclient.Credentials{}, err
	}

	return creds, nil
}

func hetznerSecretErrorResult(
	ctx context.Context,
	err error,
	bmHost *infrav1.HetznerBareMetalHost,
	client client.Client,
) (res ctrl.Result, reterr error) {
	switch err.(type) {
	// In the event that the reference to the secret is defined, but we cannot find it
	// we requeue the host as we will not know if they create the secret
	// at some point in the future.
	case *secretutil.ResolveSecretRefError:
		if err := host.SetErrorCondition(
			ctx,
			bmHost,
			client,
			infrav1.PreparationError,
			infrav1.ErrorMessageMissingHetznerSecret,
		); err != nil {
			return ctrl.Result{}, err
		}
		backoff := host.CalculateBackoff(bmHost.Spec.Status.ErrorCount)
		res = ctrl.Result{RequeueAfter: backoff}
		// No need to reconcile again, as it will be triggered as soon as the secret is updated.
	case *bmclient.CredentialsValidationError:
		if err := host.SetErrorCondition(
			ctx,
			bmHost,
			client,
			infrav1.PreparationError,
			infrav1.ErrorMessageMissingOrInvalidSecretData,
		); err != nil {
			return ctrl.Result{}, err
		}
	default:
		return ctrl.Result{}, errors.Wrap(err, "An unhandled failure occurred")
	}

	return res, err
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
					if objectOld.Spec.Status.InstallImage != objectNew.Spec.Status.InstallImage {
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
