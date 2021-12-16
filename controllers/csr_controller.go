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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/csr"
	certificatesv1 "k8s.io/api/certificates/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	Scheme           *runtime.Scheme
	WatchFilterValue string
	clientSet        *kubernetes.Clientset
	mCluster         ManagementCluster
}

//+kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages the lifecycle of a CSR object.
func (r *GuestCSRReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the CertificateSigningRequest instance.
	certificateSigningRequest := &certificatesv1.CertificateSigningRequest{}
	err := r.Get(ctx, req.NamespacedName, certificateSigningRequest)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
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

	// find matching HCloudMachine object
	var hcloudMachine infrav1.HCloudMachine
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      strings.TrimPrefix(certificateSigningRequest.Spec.Username, nodePrefix),
	}, &hcloudMachine); err != nil {
		return reconcile.Result{}, err
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
	if err := csr.ValidateKubeletCSR(csrRequest, &hcloudMachine); err != nil {
		condition.Type = certificatesv1.CertificateDenied
		condition.Reason = "CSRValidationFailed"
		condition.Message = fmt.Sprintf("Validation by cluster-api-provider-hetzner failed: %s", err)
	} else {
		condition.Type = certificatesv1.CertificateApproved
		condition.Reason = "CSRValidationSucceed"
		condition.Message = "Validation by cluster-api-provider-hetzner was successful"
	}

	certificateSigningRequest.Status.Conditions = append(
		certificateSigningRequest.Status.Conditions,
		condition,
	)

	if _, err := r.clientSet.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, hcloudMachine.Name, certificateSigningRequest, metav1.UpdateOptions{}); err != nil {
		log.Error(err, "updating approval of csr failed", "username", certificateSigningRequest.Spec.Username, "csr")
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GuestCSRReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&certificatesv1.CertificateSigningRequest{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}
