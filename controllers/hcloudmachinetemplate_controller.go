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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
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
		log.Info("Skipping reconciliation for namespace", "namespace", req.Namespace, "annotation", infrav2.SkipNamespaceAnnotation)
		return ctrl.Result{}, nil
	}

	hcloudMachineTemplate := &infrav2.HCloudMachineTemplate{}
	if err := r.Get(ctx, req.NamespacedName, hcloudMachineTemplate); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("HCloudMachineTemplate", klog.KObj(hcloudMachineTemplate))

	patchHelper, err := patch.NewHelper(hcloudMachineTemplate, r)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get patch helper: %w", err)
	}

	defer func() {
		// set summary for deprecated v1beta1 conditions.
		deprecatedv1beta1conditions.SetSummary(hcloudMachineTemplate)

		// set summary for conditions.
		if err := scope.SetHCloudMachineTemplateSummaryCondition(hcloudMachineTemplate); err != nil {
			log.Error(err, "Failed to set Ready condition")
			conditions.Set(hcloudMachineTemplate, metav1.Condition{
				Type:   clusterv1.ReadyCondition,
				Status: metav1.ConditionUnknown,
				Reason: clusterv1.InternalErrorReason,
			})
		}

		if err := patchHelper.Patch(ctx, hcloudMachineTemplate, scope.MachineTemplatePatchOpts()...); err != nil {
			log.Error(err, "failed to patch HCloudMachineTemplate")
		}
	}()

	// Check whether owner is a ClusterClass. In that case there is nothing to do.
	if hasOwnerClusterClass(hcloudMachineTemplate.ObjectMeta) {
		hcloudMachineTemplate.Status.OwnerType = "ClusterClass"
		conditions.Set(hcloudMachineTemplate, metav1.Condition{
			Type:   infrav2.HCloudMachineTemplateAvailableCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HCloudMachineTemplateOwnedByClusterClassReason,
		})
		return reconcile.Result{}, nil
	}

	var cluster *clusterv1.Cluster
	cluster, err = util.GetOwnerCluster(ctx, r, hcloudMachineTemplate.ObjectMeta)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(hcloudMachineTemplate, metav1.Condition{
				Type:   infrav2.HCloudMachineTemplateAvailableCondition,
				Status: metav1.ConditionUnknown,
				Reason: infrav2.HCloudMachineTemplateWaitingForOwnerClusterReason,
			})
		} else {
			conditions.Set(hcloudMachineTemplate, metav1.Condition{
				Type:    infrav2.HCloudMachineTemplateAvailableCondition,
				Status:  metav1.ConditionUnknown,
				Reason:  clusterv1.InternalErrorReason,
				Message: err.Error(),
			})
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("%s is missing ownerRef to cluster %s/%s",
			hcloudMachineTemplate.Kind, hcloudMachineTemplate.Namespace, hcloudMachineTemplate.Name))
		conditions.Set(hcloudMachineTemplate, metav1.Condition{
			Type:   infrav2.HCloudMachineTemplateAvailableCondition,
			Status: metav1.ConditionUnknown,
			Reason: infrav2.HCloudMachineTemplateWaitingForOwnerClusterReason,
		})
		return reconcile.Result{}, nil
	}
	hcloudMachineTemplate.Status.OwnerType = cluster.Kind

	log = log.WithValues("Cluster", klog.KObj(cluster))

	// Requeue if cluster has no infrastructure yet.
	if !cluster.Spec.InfrastructureRef.IsDefined() {
		conditions.Set(hcloudMachineTemplate, metav1.Condition{
			Type:   infrav2.HCloudMachineTemplateAvailableCondition,
			Status: metav1.ConditionFalse,
			Reason: infrav2.HCloudMachineTemplateMissingInfrastructureRefReason,
		})
		return reconcile.Result{Requeue: true}, nil
	}

	hetznerCluster := &infrav2.HetznerCluster{}

	hetznerClusterName := client.ObjectKey{
		Namespace: hcloudMachineTemplate.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		reason := clusterv1.InternalErrorReason
		if apierrors.IsNotFound(err) {
			reason = clusterv1.WaitingForClusterInfrastructureReadyReason
		}
		conditions.Set(hcloudMachineTemplate, metav1.Condition{
			Type:    infrav2.HCloudMachineTemplateAvailableCondition,
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
		return hcloudTokenErrorResult(ctx, err, hcloudMachineTemplate, r, infrav2.HCloudMachineTemplateSummaryOpts())
	}

	hcc := r.HCloudClientFactory.NewClient(hcloudToken)

	machineTemplateScope, err := scope.NewHCloudMachineTemplateScope(scope.HCloudMachineTemplateScopeParams{
		Logger:                &log,
		HCloudMachineTemplate: hcloudMachineTemplate,
		HCloudClient:          hcc,
	})
	if err != nil {
		err := fmt.Errorf("failed to create scope: %w", err)
		conditions.Set(hcloudMachineTemplate, metav1.Condition{
			Type:    infrav2.HCloudMachineTemplateAvailableCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  clusterv1.InternalErrorReason,
			Message: err.Error(),
		})
		return reconcile.Result{}, err
	}

	defer func() {
		if reterr != nil && errors.Is(reterr, hcloudclient.ErrUnauthorized) {
			deprecatedv1beta1conditions.MarkFalse(hcloudMachineTemplate, infrav2.HCloudTokenAvailableV1Beta1Condition, infrav2.HCloudCredentialsInvalidV1Beta1Reason, clusterv1.ConditionSeverityError, "wrong hcloud token")
			conditions.Set(hcloudMachineTemplate, metav1.Condition{
				Type:    infrav2.HCloudTokenAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudTokenInvalidReason,
				Message: "wrong hcloud token",
			})
		} else {
			deprecatedv1beta1conditions.MarkTrue(hcloudMachineTemplate, infrav2.HCloudTokenAvailableV1Beta1Condition)
			conditions.Set(hcloudMachineTemplate, metav1.Condition{
				Type:   infrav2.HCloudTokenAvailableCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HCloudTokenAvailableReason,
			})
		}
	}()

	// check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(hcloudMachineTemplate, r.RateLimitWaitTime); wait {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return r.reconcile(ctx, machineTemplateScope)
}

func (r *HCloudMachineTemplateReconciler) reconcile(ctx context.Context, machineTemplateScope *scope.HCloudMachineTemplateScope) (reconcile.Result, error) {
	hcloudMachineTemplate := machineTemplateScope.HCloudMachineTemplate

	result, err := machinetemplate.NewService(machineTemplateScope).Reconcile(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to reconcile machine template for HCloudMachineTemplate %s/%s: %w",
			hcloudMachineTemplate.Namespace, hcloudMachineTemplate.Name, err)
	}

	return result, nil
}

func (r *HCloudMachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav2.HCloudMachineTemplate{}).
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
