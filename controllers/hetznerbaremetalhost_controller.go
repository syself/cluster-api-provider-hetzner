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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pkg/errors"
	infrastructurev1beta1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/inventory"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioner"
)

// HetznerBareMetalHostReconciler reconciles a HetznerBareMetalHost object
type HetznerBareMetalHostReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	ProvisionerFactory provisioner.Factory
	WatchFilterValue   string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalhosts/finalizers,verbs=update

// Reconcile implements the reconcilement of HetznerBareMetalHost objects
func (r *HetznerBareMetalHostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	//	log := ctrl.LoggerFrom(ctx)

	// Fetch the Hetzner bare metal host instance.
	host := &infrav1.HetznerBareMetalHost{}
	err := r.Get(ctx, req.NamespacedName, host)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	initialState := host.Status.Provisioning.State

	info := inventory.NewReconcileInfo(host)

	prov, err := r.ProvisionerFactory.NewProvisioner(provisioner.BuildHostData(*host))
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create provisioner")
	}

	// ready, err := prov.IsReady()
	// if err != nil {
	// 	return ctrl.Result{}, errors.Wrap(err, "failed to check services availability")
	// }
	// if !ready {
	// 	reqLogger.Info("provisioner is not ready", "RequeueAfter:", provisionerNotReadyRetryDelay)
	// 	return ctrl.Result{Requeue: true, RequeueAfter: provisionerNotReadyRetryDelay}, nil
	// }

	stateMachine := inventory.NewHostStateMachine(host, prov, true)
	actResult := stateMachine.ReconcileState(info)
	_, err = actResult.Result() // result, err :=
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("action %q failed", initialState))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HetznerBareMetalHostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.HetznerBareMetalHost{}).
		Complete(r)
}
