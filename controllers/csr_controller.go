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
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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
	clusterName      string
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
		if errors.Is(err, errNoHetznerBareMetalMachineByProviderIDFound) {
			log.Info(fmt.Sprintf("ProviderID not set yet. The hbmm seems to be in 'ensure-provision'. Retrying. %s",
				err.Error()))
			return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
		}
		if err != nil {
			log.Error(err, "could not find an associated bm machine or hcloud machine",
				"userName", certificateSigningRequest.Spec.Username)
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
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

var (
	constantBareMetalHostnameRegex                = regexp.MustCompile(`^bm-(\S*)-(\d+)$`)
	errNoHetznerBareMetalMachineByProviderIDFound = fmt.Errorf("no HetznerBaremetalMachine by ProviderID found")
)

func (r *GuestCSRReconciler) getMachineAddresses(
	ctx context.Context,
	certificateSigningRequest *certificatesv1.CertificateSigningRequest,
) (machineAddresses []clusterv1.MachineAddress, isHCloudMachine bool, err error) {
	_, serverID := getServerIDFromConstantHostname(ctx, certificateSigningRequest.Spec.Username, r.clusterName)

	if serverID != "" {
		// According the the regex this is BM server with ConstantHostname. Handle that fist,
		// no need to check for a HCloud server.

		hbmm, err := getHbmmWithConstantHostname(ctx, certificateSigningRequest.Spec.Username, r.clusterName, r.mCluster)
		if errors.Is(err, errNoHetznerBareMetalMachineByProviderIDFound) {
			// No machine found yet. Likely: Cloud-init has run, the kubelet has started. But machine is still in ensure-provisioned.
			// The providerID will be set soon.
			return nil, false, err
		}
		if err != nil {
			return nil, false, fmt.Errorf("getHbmmWithConstantHostname(%q) failed: %w", certificateSigningRequest.Spec.Username, err)
		}
		if hbmm != nil {
			return hbmm.Status.Addresses, false, nil
		}
		return nil, false, fmt.Errorf("getHbmmWithConstantHostname(%q) failed to get hbmm (should not happen)", certificateSigningRequest.Spec.Username)
	}

	// It could be both: A hcloud machine or a bm-machine without ConstantHostname.

	// Try to find matching HCloudMachine object
	var hcloudMachine infrav1.HCloudMachine

	hcloudMachineName := types.NamespacedName{
		Namespace: r.mCluster.Namespace(),
		Name:      hcloudMachineNameFromCSR(certificateSigningRequest),
	}
	err = r.mCluster.Get(ctx, hcloudMachineName, &hcloudMachine)
	if err != nil {
		// Could not find HCloud machine. Try to find bare metal machine without ConstantHostname.

		var bmMachine infrav1.HetznerBareMetalMachine
		bmMachineName := types.NamespacedName{
			Namespace: r.mCluster.Namespace(),
			Name:      bmMachineNameFromCSR(certificateSigningRequest),
		}

		if err := r.mCluster.Get(ctx, bmMachineName, &bmMachine); err != nil {
			return nil, false, fmt.Errorf("failed to get hcloud (%s) or bare metal machine (%s): %w",
				hcloudMachineName.Name,
				bmMachineName.Name,
				err)
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
			DeleteFunc: func(_ event.DeleteEvent) bool {
				// We don't want to listen to delete events, as CSRs are deleted frequently without us having to do something
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				// We don't want to listen to generic events, as CSRs are genericd frequently without us having to do something
				return false
			},
			UpdateFunc: func(_ event.UpdateEvent) bool {
				// We don't want to listen to Update events, as CSRs are Updated frequently without us having to do something
				return false
			},
		}).
		Complete(r)
}

func getServerIDFromConstantHostname(ctx context.Context, csrUsername string, clusterName string) (clusterFromCSR string, serverID string) {
	log := ctrl.LoggerFrom(ctx)
	// example csrUsername: system:node:bm-my-cluster-1234567
	matches := constantBareMetalHostnameRegex.FindStringSubmatch(strings.TrimPrefix(csrUsername, nodePrefix))
	if len(matches) != 3 {
		log.V(1).Info("No constant baremetal hostname - regex does not match CSR username",
			"regex", constantBareMetalHostnameRegex.String(), "csrUserName", csrUsername)
		return "", ""
	}

	clusterFromCSR = matches[1]
	if clusterFromCSR != clusterName {
		log.V(1).Info("No constant baremetal hostname - mismatch of cluster found in csrUserName", "got", clusterFromCSR, "want", clusterName)
		return "", ""
	}
	return clusterFromCSR, matches[2]
}

func getHbmmWithConstantHostname(ctx context.Context, csrUsername string, clusterName string, mCluster ManagementCluster) (*infrav1.HetznerBareMetalMachine, error) {
	log := ctrl.LoggerFrom(ctx)

	clusterFromCSR, serverID := getServerIDFromConstantHostname(ctx, csrUsername, clusterName)
	providerID := "hcloud://bm-" + serverID
	hList := &infrav1.HetznerBareMetalMachineList{}
	selector := labels.NewSelector()
	req, err := labels.NewRequirement(clusterv1.ClusterNameLabel, selection.Equals, []string{clusterFromCSR})
	if err != nil {
		return nil, fmt.Errorf("failed to create selector %s=%s. %w",
			clusterv1.ClusterNameLabel, clusterFromCSR, err)
	}

	selector.Add(*req)

	if err := mCluster.List(ctx, hList, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     mCluster.Namespace(),
	}); err != nil {
		return nil, fmt.Errorf("failed to get HetznerBareMetalMachineList: %w", err)
	}

	var hbmm *infrav1.HetznerBareMetalMachine
	for i := range hList.Items {
		if hList.Items[i].Spec.ProviderID == nil {
			continue
		}
		if *hList.Items[i].Spec.ProviderID == providerID {
			hbmm = &hList.Items[i]
			break
		}
	}
	if hbmm == nil {
		return nil, fmt.Errorf("ProviderID: %q %w", providerID, errNoHetznerBareMetalMachineByProviderIDFound)
	}

	log.Info("Found HetznerBareMetalMachine with constant hostname", "csr-username", csrUsername, "hetznerBareMetalMachine", hbmm.Name)
	return hbmm, nil
}
