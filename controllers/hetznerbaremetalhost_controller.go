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
	"fmt"
	"time"

	"github.com/pkg/errors"
	infrastructurev1beta1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/host"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioner"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	hostErrorRetryDelay = time.Second * 10
)

// HetznerBareMetalHostReconciler reconciles a HetznerBareMetalHost object
type HetznerBareMetalHostReconciler struct {
	client.Client
	APIReader          client.Reader
	ProvisionerFactory provisioner.Factory
	RobotClientFactory robotclient.Factory
	WatchFilterValue   string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalHost objects
func (r *HetznerBareMetalHostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {

	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal host", "name", req.Name)

	// Fetch the Hetzner bare metal host instance.
	host := &infrav1.HetznerBareMetalHost{}
	err := r.Get(ctx, req.NamespacedName, host)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// TODO: Do this only when I need those SSH keys and move it to the place where I use them
	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: host.Namespace,
		Name:      host.Spec.HetznerClusterRef,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		return ctrl.Result{}, errors.New("HetznerCluster not found")
	}

	// Add a finalizer to newly created objects.
	if host.DeletionTimestamp.IsZero() && !hostHasFinalizer(host) {
		log.Info("adding finalizer", "existingFinalizers", host.Finalizers, "newValue", infrav1.HetznerBareMetalHostFinalizer)
		host.Finalizers = append(host.Finalizers,
			infrav1.HetznerBareMetalHostFinalizer)
		err := r.Update(ctx, host)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get Hetzner robot api credentials
	secretManager := secretutil.NewSecretManager(log, r.Client, r.APIReader)
	robotCreds, _, err := getAndValidateRobotCredentials(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		// TODO (janis): Implement error handling with conditions similar to the one for HetznerCluster
		return ctrl.Result{}, errors.Wrap(err, "failed to get Hetzner robot credentials")
	}

	sshCreds, sshSecret, err := getAndValidateSSHCredentials(ctx, req.Namespace, hetznerCluster, host, secretManager)
	if err != nil {
		// TODO (janis): Implement error handling with conditions similar to the one for HetznerCluster
		return ctrl.Result{}, errors.Wrap(err, "failed to get Hetzner robot credentials")
	}

	robotClient := r.RobotClientFactory.NewClient(robotCreds)
	prov := r.ProvisionerFactory.NewProvisioner(provisioner.BuildHostData(robotCreds, sshCreds))
	// Create the scope.
	hostScope, err := scope.NewBareMetalHostScope(ctx, scope.BareMetalHostScopeParams{
		Client:               r.Client,
		HetznerCluster:       hetznerCluster,
		HetznerBareMetalHost: host,
		Provisioner:          prov,
		RobotClient:          robotClient,
		SSHSecret:            sshSecret,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	if !host.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, hostScope)
	}

	return r.reconcileNormal(ctx, hostScope)
}

func (r *HetznerBareMetalHostReconciler) reconcileDelete(
	ctx context.Context,
	hostScope *scope.BareMetalHostScope,
) (reconcile.Result, error) {
	bareMetalHost := hostScope.HetznerBareMetalHost

	// delete host
	if result, brk, err := breakReconcile(host.NewService(hostScope).Delete(ctx)); brk {
		return result, errors.Wrapf(err, "failed to delete HetznerBareMetalHost %s/%s", bareMetalHost.Namespace, bareMetalHost.Name)
	}

	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(hostScope.HetznerBareMetalHost, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *HetznerBareMetalHostReconciler) reconcileNormal(
	ctx context.Context,
	hostScope *scope.BareMetalHostScope,
) (reconcile.Result, error) {
	bareMetalHost := hostScope.HetznerBareMetalHost

	// If the HetznerBareMetalHost doesn't have our finalizer, add it.
	// TODO: Adapt finalizer
	controllerutil.AddFinalizer(hostScope.HetznerBareMetalHost, infrav1.MachineFinalizer)

	// Register the finalizer immediately to avoid orphaning HCloud resources on delete
	if err := hostScope.PatchObject(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// reconcile host
	if result, brk, err := breakReconcile(host.NewService(hostScope).Reconcile(ctx)); brk {
		return result, errors.Wrapf(err, "failed to reconcile HetznerBareMetalHost %s/%s", bareMetalHost.Namespace, bareMetalHost.Name)
	}

	return reconcile.Result{}, nil
}

func getAndValidateRobotCredentials(
	ctx context.Context,
	namespace string,
	hetznerCluster *infrav1.HetznerCluster,
	secretManager *secretutil.SecretManager,
) (robotclient.RobotCredentials, *corev1.Secret, error) {
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
			return robotclient.RobotCredentials{}, nil, &secretutil.ResolveSecretRefError{Message: fmt.Sprintf("The Hetzner secret %s does not exist", secretNamspacedName)}
		}
		return robotclient.RobotCredentials{}, nil, err
	}

	creds := robotclient.RobotCredentials{
		Username: string(hetznerSecret.Data[hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser]),
		Password: string(hetznerSecret.Data[hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword]),
	}

	// Validate token
	if err := creds.Validate(); err != nil {
		return robotclient.RobotCredentials{}, nil, err
	}

	return creds, hetznerSecret, nil
}

func getAndValidateSSHCredentials(
	ctx context.Context,
	namespace string,
	hetznerCluster *infrav1.HetznerCluster,
	host *infrav1.HetznerBareMetalHost,
	secretManager *secretutil.SecretManager,
) (robotclient.SSHCredentials, *corev1.Secret, error) {
	secretNamspacedName := types.NamespacedName{Namespace: namespace, Name: hetznerCluster.Spec.SSHKeys.Robot.Name}

	sshSecret, err := secretManager.AcquireSecret(
		ctx,
		secretNamspacedName,
		host,
		false,
		host.DeletionTimestamp.IsZero(),
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return robotclient.SSHCredentials{}, nil, &secretutil.ResolveSecretRefError{Message: fmt.Sprintf("The SSH secret %s does not exist", secretNamspacedName)}
		}
		return robotclient.SSHCredentials{}, nil, err
	}

	creds := robotclient.SSHCredentials{
		Name:       string(sshSecret.Data[hetznerCluster.Spec.SSHKeys.Robot.Key.Name]),
		PublicKey:  string(sshSecret.Data[hetznerCluster.Spec.SSHKeys.Robot.Key.PublicKey]),
		PrivateKey: string(sshSecret.Data[hetznerCluster.Spec.SSHKeys.Robot.Key.PrivateKey]),
	}

	// Validate token
	if err := creds.Validate(); err != nil {
		return robotclient.SSHCredentials{}, nil, err
	}

	return creds, sshSecret, nil
}

func hostHasFinalizer(host *infrav1.HetznerBareMetalHost) bool {
	return utils.StringInList(host.Finalizers, infrav1.HetznerBareMetalHostFinalizer)
}

func (r *HetznerBareMetalHostReconciler) saveHostStatus(host *infrav1.HetznerBareMetalHost) error {
	t := metav1.Now()
	host.Spec.Status.LastUpdated = &t

	return r.Status().Update(context.TODO(), host)
}

func (r *HetznerBareMetalHostReconciler) setErrorCondition(request ctrl.Request, bmHost *infrav1.HetznerBareMetalHost, errType infrav1.ErrorType, message string) error {
	host.SetErrorMessage(bmHost, errType, message)
	return r.saveHostStatus(bmHost)
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalHostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.HetznerBareMetalHost{}).
		Complete(r)
}
