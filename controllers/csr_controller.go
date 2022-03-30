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

package controllers

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/csr"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ManagementCluster defines an interface.
type ManagementCluster interface {
	client.Client
	Namespace() string
}

// GuestCSRReconciler reconciles a CSR object.
type GuestCSRReconciler struct {
	client.Client
	WatchFilterValue string
	clientSet        *kubernetes.Clientset
	mCluster         ManagementCluster
}

//+kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch
//+kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=update
//+kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,verbs=approve,resourceNames=kubernetes.io/kubelet-serving
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages the lifecycle of a CSR object.
func (r *GuestCSRReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Starting reconcile CSR", "req", req)

	// Fetch the CertificateSigningRequest instance.
	certificateSigningRequest := &certificatesv1.CertificateSigningRequest{}
	err := r.Get(ctx, req.NamespacedName, certificateSigningRequest)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err, "found an error while getting CSR", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, err
	}

	// skip CSR that have already been decided
	if len(certificateSigningRequest.Status.Conditions) > 0 {
		return reconcile.Result{}, nil
	}

	nodePrefix := "system:node:"
	// skip CSR from non-nodes
	if !strings.HasPrefix(certificateSigningRequest.Spec.Username, nodePrefix) {
		return reconcile.Result{}, nil
	}

	var isHCloudMachine bool
	var machineName string
	var machineAddresses []corev1.NodeAddress
	// find matching HCloudMachine object
	var hcloudMachine infrav1.HCloudMachine
	err = r.mCluster.Get(ctx, types.NamespacedName{
		Namespace: r.mCluster.Namespace(),
		Name:      strings.TrimPrefix(certificateSigningRequest.Spec.Username, nodePrefix+infrav1.HCloudHostNamePrefix),
	}, &hcloudMachine)

	if err == nil {
		isHCloudMachine = true
		machineName = hcloudMachine.GetName()
		machineAddresses = hcloudMachine.Status.Addresses
	} else {
		// Check whether it is a bare metal machine
		var bmMachine infrav1.HetznerBareMetalMachine
		if err := r.mCluster.Get(ctx, types.NamespacedName{
			Namespace: r.mCluster.Namespace(),
			Name:      strings.TrimPrefix(certificateSigningRequest.Spec.Username, nodePrefix+infrav1.BareMetalHostNamePrefix),
		}, &bmMachine); err != nil {
			log.Error(err, "found an error while getting machine - bm machine or hcloud machine", "namespacedName", req.NamespacedName,
				"userName", certificateSigningRequest.Spec.Username,
				"nodePrefix", nodePrefix,
			)
			return reconcile.Result{}, err
		}
		machineName = bmMachine.GetName()
		machineAddresses = bmMachine.Status.Addresses
	}

	csrBlock, _ := pem.Decode(certificateSigningRequest.Spec.Request)

	csrRequest, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		record.Warnf(
			certificateSigningRequest,
			"CSRParsingError",
			"Error parsing CertificateSigningRequest %s: %s",
			req.Name,
			err,
		)
		return reconcile.Result{}, err
	}

	var condition = certificatesv1.CertificateSigningRequestCondition{
		LastUpdateTime: metav1.Time{Time: time.Now()},
	}

	if err := csr.ValidateKubeletCSR(csrRequest, machineName, isHCloudMachine, machineAddresses); err != nil {
		condition.Type = certificatesv1.CertificateDenied
		condition.Reason = "CSRValidationFailed"
		condition.Status = "True"
		condition.Message = fmt.Sprintf("Validation by cluster-api-provider-hetzner failed: %s", err)
		log.Error(err, "failed to validate kubelet csr")
	} else {
		condition.Type = certificatesv1.CertificateApproved
		condition.Reason = "CSRValidationSucceed"
		condition.Status = "True"
		condition.Message = "Validation by cluster-api-provider-hetzner was successful"
	}

	certificateSigningRequest.Status.Conditions = append(
		certificateSigningRequest.Status.Conditions,
		condition,
	)

	if _, err := r.clientSet.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, certificateSigningRequest.Name, certificateSigningRequest, metav1.UpdateOptions{}); err != nil {
		log.Error(err, "updating approval of csr failed", "username", certificateSigningRequest.Spec.Username)
	}
	log.Info("Finished reconcile CSR", "name", certificateSigningRequest.Name)

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GuestCSRReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&certificatesv1.CertificateSigningRequest{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				// We don't want to listen to delete events, as CSRs are deleted frequently without us having to do something
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				// We don't want to listen to generic events, as CSRs are genericd frequently without us having to do something
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				// We don't want to listen to Update events, as CSRs are Updated frequently without us having to do something
				return false
			},
		}).
		Complete(r)
}
