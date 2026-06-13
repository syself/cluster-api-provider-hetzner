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
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
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
	RateLimitWaitTime   time.Duration
	APIReader           client.Reader
	RobotClientFactory  robotclient.Factory
	SSHClientFactory    sshclient.Factory
	WatchFilterValue    string
	PreProvisionCommand string

	// Reconcile only this namespace. Only needed for testing
	Namespace string
	// WorkloadClusterClientFactory overrides the default real factory. Intended for tests only.
	WorkloadClusterClientFactory scope.WorkloadClusterClientFactory
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalHost objects.
func (r *HetznerBareMetalHostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	if r.Namespace != "" && req.Namespace != r.Namespace {
		// Just for testing, skip reconciling objects from finished tests.
		return ctrl.Result{}, nil
	}
	skipReconciliation, err := shouldSkipReconciliationForNamespace(ctx, r.Client, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if skipReconciliation {
		log.Info("Skipping reconciliation for namespace", "namespace", req.Namespace, "annotation", infrav2.SkipNamespaceAnnotation)
		return ctrl.Result{}, nil
	}

	start := time.Now()
	defer func() {
		// check duration of reconcile. Warn if it took too long.
		duration := time.Since(start)
		if duration > 15*time.Second {
			log.Info("Reconcile took too long", "duration", duration, "res", res, "reterr", reterr)
		}
	}()

	// Fetch the Hetzner bare metal host instance.
	bmHost := &infrav2.HetznerBareMetalHost{}
	err = r.Get(ctx, req.NamespacedName, bmHost)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// ----------------------------------------------------------------
	// Start: avoid conflict errors. Wait until local cache is up-to-date
	// Won't be needed once this was implemented:
	// https://github.com/kubernetes-sigs/controller-runtime/issues/3320
	initialHost := bmHost.DeepCopy()
	defer func() {
		// We can potentially optimize this further by ensuring that the cache is up to date only in
		// the cases where an outdated cache would lead to problems. Currently, we ensure that the
		// cache is up to date in all cases, i.e. for all possible changes to the
		// HetznerBareMetalHost object.
		if cmp.Equal(initialHost, bmHost) {
			// Nothing has changed. No need to wait.
			return
		}
		startReadOwnWrite := time.Now()

		// The object changed. Wait until the new version is in the local cache

		// Get the latest version from the apiserver.
		apiserverHost := &infrav2.HetznerBareMetalHost{}

		// Use uncached APIReader
		err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(bmHost), apiserverHost)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// resource was deleted. No need to reconcile again.
				reterr = nil
				res = reconcile.Result{}
				return
			}
			reterr = errors.Join(reterr,
				fmt.Errorf("failed get HetznerBareMetalHost via uncached APIReader: %w", err))
			return
		}

		apiserverRV := apiserverHost.ResourceVersion

		err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 3*time.Second, true, func(ctx context.Context) (done bool, err error) {
			// new resource, read from local cache
			latestFromLocalCache := &infrav2.HetznerBareMetalHost{}
			getErr := r.Get(ctx, client.ObjectKeyFromObject(apiserverHost), latestFromLocalCache)
			if apierrors.IsNotFound(getErr) {
				// the object was deleted. All is fine.
				return true, nil
			}
			if getErr != nil {
				return false, getErr
			}
			return utils.IsLocalCacheUpToDate(latestFromLocalCache.ResourceVersion, apiserverRV), nil
		})
		if err != nil {
			log.Error(err, "cache sync failed after BootState change")
		}
		if time.Since(startReadOwnWrite) > 50*time.Millisecond {
			log.Info("Wait for update being in local cache", "durationWaitForLocalCacheSync", time.Since(startReadOwnWrite).Round(time.Millisecond))
		}
	}()
	// End: avoid conflict errors. Wait until local cache is up-to-date
	// ----------------------------------------------------------------

	patchHelper, err := patch.NewHelper(bmHost, r)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	initialProvisioningState := bmHost.Status.ProvisioningState

	defer func() {
		if initialProvisioningState != bmHost.Status.ProvisioningState {
			log.Info("Provisioning state changed", "from", initialProvisioningState, "to", bmHost.Status.ProvisioningState)
		}

		deprecatedv1beta1conditions.SetSummary(bmHost)
		scope.SetHetznerBareMetalHostReadySummary(bmHost)

		if err := patchHelper.Patch(ctx, bmHost, scope.BareMetalHostPatchOpts()...); err != nil {
			res = reconcile.Result{}
			reterr = errors.Join(reterr, err)
			return
		}
	}()

	// Add a finalizer to newly created objects.
	if bmHost.DeletionTimestamp.IsZero() &&
		(controllerutil.AddFinalizer(bmHost, infrav2.HetznerBareMetalHostFinalizer) ||
			controllerutil.RemoveFinalizer(bmHost, infrav2.DeprecatedBareMetalHostFinalizer)) {
		return ctrl.Result{Requeue: true}, nil
	}

	// Remove permanent error, if the corresponding annotation was removed by the user.
	removed := removePermanentErrorIfAnnotationIsGone(bmHost)
	if removed {
		// The permanent error was removed from the status.
		// Save the changes, and then reconcile again.
		return reconcile.Result{Requeue: true}, nil
	}

	// Fetch the consuming HetznerBareMetalMachine and its CAPI Machine. The host both starts
	// provisioning and deprovisions based on the machine, and reads its provisioning inputs
	// (installImage, sshSpec, bootstrap data) from it.
	var hetznerBareMetalMachine *infrav1.HetznerBareMetalMachine
	var machine *clusterv1.Machine

	if bmHost.Spec.ConsumerRef != nil {
		// The consuming machine always lives in the namespace of the host.
		hbmm := &infrav1.HetznerBareMetalMachine{}
		name := client.ObjectKey{
			Namespace: bmHost.Namespace,
			Name:      bmHost.Spec.ConsumerRef.Name,
		}
		if err := r.Get(ctx, name, hbmm); err != nil {
			if !apierrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
			// The machine was force deleted. The host scope gets a nil machine and the host
			// deprovisions with the robot-side cleanup only.
		} else {
			hetznerBareMetalMachine = hbmm

			// If the owner Machine was force deleted while the hbmm lingers, keep reconciling with a
			// nil machine so the host can still deprovision. A nil machine is handled downstream
			// (needsProvisioning is false, provisioningCancelled treats it as cancellation).
			machine, err = util.GetOwnerMachine(ctx, r, hbmm.ObjectMeta)
			if apierrors.IsNotFound(err) {
				machine = nil
			} else if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	log = log.WithValues("HetznerBareMetalMachine", klog.KObj(hetznerBareMetalMachine))

	// Certain cases need to be handled here and not later in the host state machine.
	// If res != nil, then we should return, otherwise not.
	res = r.reconcileSelectedStates(bmHost, hetznerBareMetalMachine, machine)
	emptyResult := reconcile.Result{}
	if res != emptyResult {
		return res, nil
	}

	// Case "Delete" was handled in reconcileSelectedStates. From now we know that the host has no
	// DeletionTimestamp set. But the hbmm could be in Deprovisioning.

	if bmHost.Labels[clusterv1.ClusterNameLabel] == "" {
		log.Info("HetznerBareMetalHost has no cluster-name label. Looks like a stale cache read")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Fetch the Cluster through the cluster-name label. The machine side sets the label when a
	// machine takes the host.
	cluster, err := util.GetClusterFromMetadata(ctx, r, bmHost.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	// Fetch the HetznerCluster through the Cluster infrastructure ref.
	infraRef := cluster.Spec.InfrastructureRef
	if !infraRef.IsDefined() || infraRef.Kind != "HetznerCluster" || infraRef.APIGroup != infrav1.GroupVersion.Group {
		log.Info("Cluster has no HetznerCluster infrastructure ref. Won't reconcile", "infrastructureRef", infraRef)
		return reconcile.Result{}, nil
	}

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: bmHost.Namespace,
		Name:      infraRef.Name,
	}
	if err := r.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))

	if annotations.IsPaused(cluster, bmHost) {
		log.Info("HetznerBareMetalHost or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	ctx = ctrl.LoggerInto(ctx, log)

	// check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRobotRateLimit(bmHost, r.RateLimitWaitTime); wait {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get Hetzner robot api credentials
	secretManager := secretutil.NewSecretManager(log, r, r.APIReader)
	robotCreds, err := getAndValidateRobotCredentials(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hetznerSecretErrorResult(err, bmHost)
	}

	// Get secrets. Return when result != nil.
	osSSHSecret, rescueSSHSecret, res, err := r.getSecrets(ctx, *secretManager, bmHost, hetznerBareMetalMachine, hetznerCluster)
	if err != nil {
		return reconcile.Result{}, err
	}
	if res != emptyResult {
		return res, nil
	}

	// Create the scope.
	hostScope, err := scope.NewBareMetalHostScope(scope.BareMetalHostScopeParams{
		Logger:                       log,
		Client:                       r,
		HetznerCluster:               hetznerCluster,
		Cluster:                      cluster,
		HetznerBareMetalHost:         bmHost,
		HetznerBareMetalMachine:      hetznerBareMetalMachine,
		Machine:                      machine,
		RobotClient:                  r.RobotClientFactory.NewClient(robotCreds),
		SSHClientFactory:             r.SSHClientFactory,
		OSSSHSecret:                  osSSHSecret,
		RescueSSHSecret:              rescueSSHSecret,
		SecretManager:                secretManager,
		PreProvisionCommand:          r.PreProvisionCommand,
		WorkloadClusterClientFactory: r.WorkloadClusterClientFactory,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
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

func (r *HetznerBareMetalHostReconciler) reconcileSelectedStates(
	bmHost *infrav2.HetznerBareMetalHost,
	hbmm *infrav1.HetznerBareMetalMachine,
	machine *clusterv1.Machine,
) ctrl.Result {
	switch bmHost.Status.ProvisioningState {
	// Handle StateNone: check whether needs to be provisioned or deleted.
	case infrav2.StateNone:
		if !bmHost.DeletionTimestamp.IsZero() && bmHost.Spec.ConsumerRef == nil {
			bmHost.Status.ProvisioningState = infrav2.StateDeleting
			conditions.Set(bmHost, metav1.Condition{
				Type:   infrav2.HetznerBareMetalHostDeletingCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HetznerBareMetalHostDeletingReason,
			})
		} else if needsProvisioning(hbmm, machine) {
			bmHost.Status.ProvisioningState = infrav2.StatePreparing
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}

	// Handle StateDeleting
	case infrav2.StateDeleting:
		conditions.Set(bmHost, metav1.Condition{
			Type:   infrav2.HetznerBareMetalHostDeletingCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HetznerBareMetalHostDeletingReason,
		})
		// remove finalizers.
		controllerutil.RemoveFinalizer(bmHost, infrav2.HetznerBareMetalHostFinalizer)
		controllerutil.RemoveFinalizer(bmHost, infrav2.DeprecatedBareMetalHostFinalizer)
		return reconcile.Result{Requeue: true}
	}
	return ctrl.Result{}
}

// needsProvisioning returns true when the host has a consuming machine that is not being deleted,
// whose owner CAPI Machine exists and is not being deleted, and whose bootstrap data is available.
// The host used to decide this based on the installImage that the machine copied onto it; since
// v1beta2 the provisioning inputs stay on the machine.
func needsProvisioning(hbmm *infrav1.HetznerBareMetalMachine, machine *clusterv1.Machine) bool {
	if hbmm == nil || !hbmm.DeletionTimestamp.IsZero() {
		return false
	}
	if machine == nil || !machine.DeletionTimestamp.IsZero() {
		return false
	}
	return machine.Spec.Bootstrap.DataSecretName != nil
}

func (r *HetznerBareMetalHostReconciler) getSecrets(
	ctx context.Context,
	secretManager secretutil.SecretManager,
	bmHost *infrav2.HetznerBareMetalHost,
	hbmm *infrav1.HetznerBareMetalMachine,
	hetznerCluster *infrav1.HetznerCluster,
) (
	osSSHSecret *corev1.Secret,
	rescueSSHSecret *corev1.Secret,
	res ctrl.Result,
	reterr error,
) {
	if hbmm != nil {
		var err error
		osSSHSecretNamespacedName := types.NamespacedName{Namespace: bmHost.Namespace, Name: hbmm.Spec.SSHSpec.SecretRef.Name}
		osSSHSecret, err = secretManager.ObtainSecret(ctx, osSSHSecretNamespacedName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("%s: %s", infrav2.ErrorMessageMissingOSSSHSecret, err.Error())
				deprecatedv1beta1conditions.MarkFalse(
					bmHost,
					infrav2.CredentialsAvailableV1Beta1Condition,
					infrav2.OSSSHSecretMissingV1Beta1Reason,
					clusterv1.ConditionSeverityError,
					"%s",
					msg,
				)
				conditions.Set(bmHost, metav1.Condition{
					Type:    infrav2.HetznerBareMetalHostSSHKeysAvailableCondition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav2.HetznerBareMetalHostOSSSHSecretMissingReason,
					Message: msg,
				})
				record.Warnf(bmHost, infrav2.OSSSHSecretMissingV1Beta1Reason, msg)
				deprecatedv1beta1conditions.SetSummary(bmHost)
				scope.SetHetznerBareMetalHostReadySummary(bmHost)
				return nil, nil, reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
			}
			return nil, nil, res, fmt.Errorf("failed to get secret: %w", err)
		}

		rescueSSHSecretNamespacedName := types.NamespacedName{Namespace: bmHost.Namespace, Name: hetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name}
		rescueSSHSecret, err = secretManager.AcquireSecret(ctx, rescueSSHSecretNamespacedName, hetznerCluster, false, hetznerCluster.DeletionTimestamp.IsZero())
		if err != nil {
			if apierrors.IsNotFound(err) {
				deprecatedv1beta1conditions.MarkFalse(
					bmHost,
					infrav2.CredentialsAvailableV1Beta1Condition,
					infrav2.RescueSSHSecretMissingV1Beta1Reason,
					clusterv1.ConditionSeverityError,
					infrav2.ErrorMessageMissingRescueSSHSecret,
				)
				conditions.Set(bmHost, metav1.Condition{
					Type:    infrav2.HetznerBareMetalHostSSHKeysAvailableCondition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav2.HetznerBareMetalHostRescueSSHSecretMissingReason,
					Message: infrav2.ErrorMessageMissingRescueSSHSecret,
				})

				record.Warnf(bmHost, infrav2.RescueSSHSecretMissingV1Beta1Reason, infrav2.ErrorMessageMissingRescueSSHSecret)
				deprecatedv1beta1conditions.SetSummary(bmHost)
				scope.SetHetznerBareMetalHostReadySummary(bmHost)
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
	err error,
	bmHost *infrav2.HetznerBareMetalHost,
) (res ctrl.Result, reterr error) {
	resolveErr := &secretutil.ResolveSecretRefError{}
	if errors.As(err, &resolveErr) {
		// In the event that the reference to the secret is defined, but we cannot find it
		// we requeue the host as we will not know if they create the secret
		// at some point in the future.
		deprecatedv1beta1conditions.MarkFalse(
			bmHost,
			infrav2.RobotCredentialsAvailableV1Beta1Condition,
			infrav2.HetznerSecretUnreachableV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			infrav2.ErrorMessageMissingHetznerSecret,
		)
		conditions.Set(bmHost, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostRobotCredentialsAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostSecretUnreachableReason,
			Message: infrav2.ErrorMessageMissingHetznerSecret,
		})

		record.Warnf(bmHost, infrav2.HetznerSecretUnreachableV1Beta1Reason, fmt.Sprintf("%s: %s", infrav2.ErrorMessageMissingHetznerSecret, err.Error()))
		deprecatedv1beta1conditions.SetSummary(bmHost)
		scope.SetHetznerBareMetalHostReadySummary(bmHost)

		// No need to reconcile again, as it will be triggered as soon as the secret is updated.
		return res, nil
	}

	credValidationErr := &bmclient.CredentialsValidationError{}
	if errors.As(err, &credValidationErr) {
		deprecatedv1beta1conditions.MarkFalse(
			bmHost,
			infrav2.RobotCredentialsAvailableV1Beta1Condition,
			infrav2.RobotCredentialsInvalidV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			infrav2.ErrorMessageMissingOrInvalidSecretData,
		)
		conditions.Set(bmHost, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostRobotCredentialsAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostRobotCredentialsInvalidReason,
			Message: infrav2.ErrorMessageMissingOrInvalidSecretData,
		})
		record.Warnf(bmHost, infrav2.RobotCredentialsInvalidV1Beta1Reason, err.Error())
		return res, nil
	}
	return reconcile.Result{}, fmt.Errorf("hetznerSecretErrorResult: an unhandled failure occurred: %T %w", err, err)
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalHostReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	clusterToObjectFunc, err := util.ClusterToTypedObjectsMapper(r, &infrav2.HetznerBareMetalHostList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for Cluster to HetznerBareMetalHosts: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav2.HetznerBareMetalHost{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue)).
		WithEventFilter(
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					objectOld, oldOK := e.ObjectOld.(*infrav2.HetznerBareMetalHost)
					objectNew, newOK := e.ObjectNew.(*infrav2.HetznerBareMetalHost)

					if !oldOK || !newOK {
						// The thing that changed wasn't a host, so we
						// need to assume that we must update. This
						// happens when, for example, an owned Secret
						// changes.
						return true
					}

					// If provisioning state changes, then we want to reconcile
					if objectOld.Status.ProvisioningState != objectNew.Status.ProvisioningState {
						return true
					}

					// Take updates of finalizers or annotations
					if !reflect.DeepEqual(objectNew.GetFinalizers(), objectOld.GetFinalizers()) ||
						!reflect.DeepEqual(objectNew.GetAnnotations(), objectOld.GetAnnotations()) {
						return true
					}

					// We can ignore changes that only touch the rest of the status.
					return !reflect.DeepEqual(objectOld.Spec, objectNew.Spec)
				},
			}).
		Owns(&corev1.Secret{}).
		Watches(
			&infrav1.HetznerBareMetalMachine{},
			handler.EnqueueRequestsFromMapFunc(hetznerBareMetalMachineToHetznerBareMetalHost),
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

// hetznerBareMetalMachineToHetznerBareMetalHost enqueues the host that is bound to the changed
// machine. The host starts provisioning and deprovisions based on its machine, so it must see
// machine events.
func hetznerBareMetalMachineToHetznerBareMetalHost(_ context.Context, obj client.Object) []reconcile.Request {
	hbmm, ok := obj.(*infrav1.HetznerBareMetalMachine)
	if !ok {
		return nil
	}

	hostKey, ok := hbmm.GetAnnotations()[infrav2.HostAnnotation]
	if !ok {
		return nil
	}

	// The annotation has the format "namespace/hbmh-name". The namespace gets ignored, as
	// cross-namespace references are not allowed.
	parts := strings.Split(hostKey, "/")
	if len(parts) != 2 {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: hbmm.Namespace,
				Name:      parts[1],
			},
		},
	}
}

func removePermanentErrorIfAnnotationIsGone(bmHost *infrav2.HetznerBareMetalHost,
) (removed bool) {
	if bmHost.Status.OperationalState != infrav2.OperationalStatePermanentError {
		// OperationalStatePermanentError not set. Do nothing.
		return false
	}
	for k := range bmHost.GetAnnotations() {
		if k == infrav2.PermanentErrorAnnotation {
			// Annotation was not removed by user. Do nothing.
			return false
		}
	}
	bmHost.ClearOperationalState()
	record.Eventf(bmHost, "PermanentErrorWasRemoved", "The permanent error was removed, because the annotation %q was removed",
		infrav2.PermanentErrorAnnotation)
	return true
}
