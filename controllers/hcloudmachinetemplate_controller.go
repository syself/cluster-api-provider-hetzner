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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/machinetemplate"
)

// HCloudMachineTemplateReconciler reconciles a HCloudMachineTemplate object.
type HCloudMachineTemplateReconciler struct {
	client.Client
	RateLimitWaitTime   time.Duration
	APIReader           client.Reader
	HCloudClientFactory hcloudclient.Factory
	WatchFilterValue    string

	// Reconcile only this namespace. Only needed for testing
	Namespace string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachinetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachinetemplates/status,verbs=get;update;patch

// Reconcile manages the lifecycle of an HCloudMachineTemplate object.
func (r *HCloudMachineTemplateReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
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
		log.Info("Skipping reconciliation for namespace", "namespace", req.Namespace, "annotation", infrav1.SkipNamespaceAnnotation)
		return ctrl.Result{}, nil
	}

	machineTemplate := &infrav1.HCloudMachineTemplate{}
	if err := r.Get(ctx, req.NamespacedName, machineTemplate); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("HCloudMachineTemplate", klog.KObj(machineTemplate))

	patchHelper, err := v1beta1patch.NewHelper(machineTemplate, r)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get patch helper: %w", err)
	}

	defer func() {
		// Compute v1beta1 summary.
		v1beta1conditions.SetSummary(machineTemplate)

		// Compute v1beta2 Ready summary from the owned conditions.
		if err := scope.SetHCloudMachineTemplateV1Beta2SummaryCondition(machineTemplate); err != nil {
			log.Error(err, "Failed to set v1beta2 Ready condition")
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:   clusterv1beta1.ReadyV1Beta2Condition,
				Status: metav1.ConditionUnknown,
				Reason: infrav1.InternalErrorV1Beta2Reason,
			})
		}

		if err := patchHelper.Patch(ctx, machineTemplate, scope.MachineTemplatePatchOpts()...); err != nil {
			log.Error(err, "failed to patch HCloudMachineTemplate")
		}
	}()

	// removing finalizer that was set in previous versions but is not needed
	// We can remove that code in 2025.
	controllerutil.RemoveFinalizer(machineTemplate, infrav1.DeprecatedHCloudMachineFinalizer)

	// Check whether owner is a ClusterClass. In that case there is nothing to do.
	if hasOwnerClusterClass(machineTemplate.ObjectMeta) {
		machineTemplate.Status.OwnerType = "ClusterClass"
		v1beta2conditions.Set(machineTemplate, metav1.Condition{
			Type:   infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.HCloudMachineTemplateOwnedByClusterClassV1Beta2Reason,
		})
		return reconcile.Result{}, nil
	}

	var cluster *clusterv1.Cluster
	cluster, err = util.GetOwnerCluster(ctx, r, machineTemplate.ObjectMeta)
	if err != nil {
		if apierrors.IsNotFound(err) {
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:   infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
				Status: metav1.ConditionUnknown,
				Reason: infrav1.HCloudMachineTemplateWaitingForOwnerClusterV1Beta2Reason,
			})
		} else {
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:    infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
				Status:  metav1.ConditionUnknown,
				Reason:  infrav1.InternalErrorV1Beta2Reason,
				Message: err.Error(),
			})
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("%s is missing ownerRef to cluster %s/%s",
			machineTemplate.Kind, machineTemplate.Namespace, machineTemplate.Name))
		v1beta2conditions.Set(machineTemplate, metav1.Condition{
			Type:   infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
			Status: metav1.ConditionUnknown,
			Reason: infrav1.HCloudMachineTemplateWaitingForOwnerClusterV1Beta2Reason,
		})
		return reconcile.Result{}, nil
	}
	machineTemplate.Status.OwnerType = cluster.Kind

	log = log.WithValues("Cluster", klog.KObj(cluster))

	// Requeue if cluster has no infrastructure yet.
	if !cluster.Spec.InfrastructureRef.IsDefined() {
		v1beta2conditions.Set(machineTemplate, metav1.Condition{
			Type:   infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
			Status: metav1.ConditionFalse,
			Reason: infrav1.HCloudMachineTemplateMissingInfrastructureRefV1Beta2Reason,
		})
		return reconcile.Result{Requeue: true}, nil
	}

	hetznerCluster := &infrav1.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: machineTemplate.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		reason := infrav1.InternalErrorV1Beta2Reason
		if apierrors.IsNotFound(err) {
			reason = clusterv1beta1.WaitingForClusterInfrastructureReadyV1Beta2Reason
		}
		v1beta2conditions.Set(machineTemplate, metav1.Condition{
			Type:    infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  reason,
			Message: err.Error(),
		})
		return reconcile.Result{}, nil
	}

	log = log.WithValues("HetznerCluster", klog.KObj(hetznerCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Create the scope.
	secretManager := secretutil.NewSecretManager(log, r, r.APIReader)
	hcloudToken, _, err := getAndValidateHCloudToken(ctx, req.Namespace, hetznerCluster, secretManager)
	if err != nil {
		return hcloudTokenErrorResult(ctx, err, machineTemplate, r, infrav1.HCloudMachineTemplateV1Beta2SummaryOpts())
	}

	hcc := r.HCloudClientFactory.NewClient(hcloudToken)

	machineTemplateScope, err := scope.NewHCloudMachineTemplateScope(scope.HCloudMachineTemplateScopeParams{
		Logger:                &log,
		HCloudMachineTemplate: machineTemplate,
		HCloudClient:          hcc,
	})
	if err != nil {
		err := fmt.Errorf("failed to create scope: %w", err)
		v1beta2conditions.Set(machineTemplate, metav1.Condition{
			Type:    infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  infrav1.InternalErrorV1Beta2Reason,
			Message: err.Error(),
		})
		return reconcile.Result{}, err
	}

	// Always close the scope when exiting this function so we can persist any HCloudMachine changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			v1beta1conditions.MarkFalse(machineTemplate, infrav1.HCloudTokenAvailableCondition, infrav1.HCloudCredentialsInvalidReason, clusterv1beta1.ConditionSeverityError, "wrong hcloud token")
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
				Message: "wrong hcloud token",
			})
		} else {
			v1beta1conditions.MarkTrue(machineTemplate, infrav1.HCloudTokenAvailableCondition)
			v1beta2conditions.Set(machineTemplate, metav1.Condition{
				Type:   infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: infrav1.HCloudTokenAvailableV1Beta2Reason,
			})
		}
	}()

	// check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(machineTemplate, r.RateLimitWaitTime); wait {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := r.reconcile(ctx, machineTemplateScope); err != nil {
		v1beta2conditions.Set(machineTemplate, metav1.Condition{
			Type:    infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.InternalErrorV1Beta2Reason,
			Message: err.Error(),
		})
		return reconcile.Result{}, err
	}
	v1beta2conditions.Set(machineTemplate, metav1.Condition{
		Type:   infrav1.HCloudMachineTemplateAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudMachineTemplateAvailableV1Beta2Reason,
	})
	return reconcile.Result{}, nil
}

func (r *HCloudMachineTemplateReconciler) reconcile(ctx context.Context, machineTemplateScope *scope.HCloudMachineTemplateScope) error {
	hcloudMachineTemplate := machineTemplateScope.HCloudMachineTemplate

	// reconcile machine template
	if err := machinetemplate.NewService(machineTemplateScope).Reconcile(ctx); err != nil {
		return fmt.Errorf("failed to reconcile machine template for HCloudMachineTemplate %s/%s: %w",
			hcloudMachineTemplate.Namespace, hcloudMachineTemplate.Name, err)
	}

	return nil
}

func (r *HCloudMachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.HCloudMachineTemplate{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// hasOwnerClusterClass returns whether the object has a ClusterClass as owner.
func hasOwnerClusterClass(obj metav1.ObjectMeta) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == "ClusterClass" {
			return true
		}
	}
	return false
}
