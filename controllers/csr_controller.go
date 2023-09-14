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

	certificatesv1 "k8s.io/api/certificates/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/csr"
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

const nodePrefix = "system:node:"

//+kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch
//+kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=update
//+kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,verbs=approve,resourceNames=kubernetes.io/kubelet-serving
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hcloudmachines,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages the lifecycle of a CSR object.
func (r *GuestCSRReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the CertificateSigningRequest instance.
	certificateSigningRequest := &certificatesv1.CertificateSigningRequest{}
	err := r.Get(ctx, req.NamespacedName, certificateSigningRequest)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get CSR: %w", err)
	}

	// skip CSR that have already been decided
	if len(certificateSigningRequest.Status.Conditions) > 0 {
		return reconcile.Result{}, nil
	}

	log = log.WithValues("CertificateSigningRequest", klog.KObj(certificateSigningRequest))

	// skip CSR from non-nodes
	if !isCSRFromNode(certificateSigningRequest) {
		return reconcile.Result{}, nil
	}

	condition := certificatesv1.CertificateSigningRequestCondition{
		LastUpdateTime: metav1.Time{Time: time.Now()},
	}

	isTooOld := certificateSigningRequest.CreationTimestamp.Before(&metav1.Time{Time: time.Now().Add(-1 * time.Hour)})
	if isTooOld {
		condition.Type = certificatesv1.CertificateDenied
		condition.Reason = "CSRTooOld"
		condition.Status = "True"
		condition.Message = "csr ist too old"
	} else {
		// get machine addresses from corresponding machine
		machineAddresses, isHCloudMachine, err := r.getMachineAddresses(ctx, certificateSigningRequest)
		if err != nil {
			log.Error(err, "could not find an associated bm machine or hcloud machine",
				"userName", certificateSigningRequest.Spec.Username)
			return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
		}

		machineName := machineNameFromCSR(certificateSigningRequest, isHCloudMachine)
		machineRef := klog.KRef(r.mCluster.Namespace(), machineName)

		if isHCloudMachine {
			log = log.WithValues("HCloudMachine", machineRef)
		} else {
			log = log.WithValues("HetznerBareMetalMachine", machineRef)
		}

		ctx = ctrl.LoggerInto(ctx, log)

		csrRequest, err := getx509CSR(certificateSigningRequest)
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

		nameWithPrefix := machineNameWithPrefix(machineName, isHCloudMachine)

		if err := csr.ValidateKubeletCSR(csrRequest, nameWithPrefix, machineAddresses); err != nil {
			condition.Type = certificatesv1.CertificateDenied
			condition.Reason = "CSRValidationFailed"
			condition.Status = "True"
			condition.Message = fmt.Sprintf("Validation by cluster-api-provider-hetzner failed: %s", err)
			record.Warnf(certificateSigningRequest, condition.Reason, "failed to validate kubelet csr: %s", err.Error())
		} else {
			condition.Type = certificatesv1.CertificateApproved
			condition.Reason = "CSRValidationSucceed"
			condition.Status = "True"
			condition.Message = "Validation by cluster-api-provider-hetzner was successful"
		}
	}

	certificateSigningRequest.Status.Conditions = append(
		certificateSigningRequest.Status.Conditions,
		condition,
	)

	if _, err := r.clientSet.CertificatesV1().CertificateSigningRequests().UpdateApproval(
		ctx,
		certificateSigningRequest.Name,
		certificateSigningRequest,
		metav1.UpdateOptions{},
	); err != nil {
		record.Warnf(certificateSigningRequest, "ApprovalFailed", "approval of csr failed: %s", err.Error())
		return reconcile.Result{}, fmt.Errorf("updating approval of csr failed. userName %q: %w",
			certificateSigningRequest.Spec.Username, err)
	}

	record.Event(certificateSigningRequest, "CSRApproved", "approved csr")
	return reconcile.Result{}, nil
}

func isCSRFromNode(certificateSigningRequest *certificatesv1.CertificateSigningRequest) bool {
	return strings.HasPrefix(certificateSigningRequest.Spec.Username, nodePrefix)
}

func hcloudMachineNameFromCSR(certificateSigningRequest *certificatesv1.CertificateSigningRequest) string {
	return strings.TrimPrefix(certificateSigningRequest.Spec.Username, nodePrefix)
}

func bmMachineNameFromCSR(certificateSigningRequest *certificatesv1.CertificateSigningRequest) string {
	return strings.TrimPrefix(certificateSigningRequest.Spec.Username, nodePrefix+infrav1.BareMetalHostNamePrefix)
}

func machineNameFromCSR(certificateSigningRequest *certificatesv1.CertificateSigningRequest, isHCloudMachine bool) string {
	if isHCloudMachine {
		return hcloudMachineNameFromCSR(certificateSigningRequest)
	}
	return bmMachineNameFromCSR(certificateSigningRequest)
}

func machineNameWithPrefix(machineName string, isHCloudMachine bool) string {
	var hostNamePrefix string
	if !isHCloudMachine {
		hostNamePrefix = infrav1.BareMetalHostNamePrefix
	}
	return hostNamePrefix + machineName
}

func (r *GuestCSRReconciler) getMachineAddresses(
	ctx context.Context,
	certificateSigningRequest *certificatesv1.CertificateSigningRequest,
) (machineAddresses []clusterv1.MachineAddress, isHCloudMachine bool, err error) {
	// try to find matching HCloudMachine object
	var hcloudMachine infrav1.HCloudMachine

	hcloudMachineName := types.NamespacedName{
		Namespace: r.mCluster.Namespace(),
		Name:      hcloudMachineNameFromCSR(certificateSigningRequest),
	}

	err = r.mCluster.Get(ctx, hcloudMachineName, &hcloudMachine)
	if err != nil {
		// Could not find HCloud machine. Try to find bare metal machine.
		var bmMachine infrav1.HetznerBareMetalMachine
		bmMachineName := types.NamespacedName{
			Namespace: r.mCluster.Namespace(),
			Name:      bmMachineNameFromCSR(certificateSigningRequest),
		}

		if err := r.mCluster.Get(ctx, bmMachineName, &bmMachine); err != nil {
			return nil, false, fmt.Errorf("failed to get hcloud and bare metal machine")
		}

		return bmMachine.Status.Addresses, false, nil
	}

	return hcloudMachine.Status.Addresses, true, nil
}

func getx509CSR(certificateSigningRequest *certificatesv1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	csrBlock, _ := pem.Decode(certificateSigningRequest.Spec.Request)
	if csrBlock == nil {
		return nil, fmt.Errorf("failed to decode csr request")
	}
	csrRequest, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSR to x509: %w", err)
	}
	return csrRequest, nil
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
