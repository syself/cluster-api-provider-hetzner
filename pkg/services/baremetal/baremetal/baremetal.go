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

// Package baremetal implements functions to manage the lifecycle of baremetal machines as inventory
package baremetal

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
// hoursBeforeDeletion      time.Duration = 36 // TODO: Implement logic for removal of unpaid servers
// rateLimitTimeOut         time.Duration = 660 // TODO: Implement logic to handle rate limits
// rateLimitTimeOutDeletion time.Duration = 120
)

const (
	// ProviderIDPrefix is a prefix for ProviderID.
	ProviderIDPrefix = "hcloud://"
	// requeueAfter gives the duration of time until the next reconciliation should be performed.
	requeueAfter = time.Second * 30

	// FailureMessageMaintenanceMode indicates that host is in maintenance mode.
	FailureMessageMaintenanceMode = "host machine in maintenance mode"
)

// Service defines struct with machine scope to reconcile Hetzner bare metal machines.
type Service struct {
	scope *scope.BareMetalMachineScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.BareMetalMachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of Hetzner bare metal machines.
func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal machine", "name", s.scope.BareMetalMachine.Name)

	// if the machine is already provisioned, update and return
	if s.scope.IsProvisioned() {
		errType := capierrors.UpdateMachineError
		return s.checkMachineError(s.update(ctx, log), "Failed to update the HetznerBareMetalMachine", errType)
	}

	// Make sure bootstrap data is available and populated. If not, return, we
	// will get an event from the machine update when the flag is set to true.
	if !s.scope.IsBootstrapReady(ctx) {
		return &ctrl.Result{}, nil
	}

	errType := capierrors.CreateMachineError

	// Check if the bareMetalmachine was associated with a baremetalhost
	if !s.scope.HasAnnotation() {
		// Associate the baremetalhost hosting the machine
		err := s.associate(ctx, log)
		if err != nil {
			return s.checkMachineError(err, "failed to associate the HetznerBareMetalMachine to a BaremetalHost", errType)
		}
	}

	err = s.update(ctx, log)
	if err != nil {
		return s.checkMachineError(err, "failed to update BaremetalHost", errType)
	}

	providerID, bmhID := s.GetProviderIDAndBMHID()
	if bmhID == nil {
		bmhID, err = s.GetBaremetalHostID(ctx)
		if err != nil {
			return s.checkMachineError(err, "failed to get the providerID for the bareMetalMachine", errType)
		}
		if bmhID != nil {
			providerID = fmt.Sprintf("%s%s%s", ProviderIDPrefix, infrav1.BareMetalHostNamePrefix, *bmhID)
		}
	}
	if bmhID != nil {
		// Make sure Spec.ProviderID is set and mark the bareMetalMachine ready
		s.scope.BareMetalMachine.Spec.ProviderID = &providerID
		s.scope.BareMetalMachine.Status.Ready = true
		conditions.MarkTrue(s.scope.BareMetalMachine, infrav1.InstanceReadyCondition)
	}

	return &ctrl.Result{}, err
}

// Delete implements delete method of bare metal machine.
func (s *Service) Delete(ctx context.Context) (_ *ctrl.Result, err error) {
	s.scope.Info("Deleting bareMetalMachine", "bareMetalMachine", s.scope.BareMetalMachine.Name)

	host, helper, err := s.getHost(ctx)
	if err != nil {
		return nil, err
	}

	if host != nil && host.Spec.ConsumerRef != nil {
		if s.scope.IsControlPlane() {
			if err := s.deleteServerOfLoadBalancer(ctx, host); err != nil {
				return nil, errors.Errorf("Error while deleting attached server of loadbalancer: %s", err)
			}
		}

		// don't remove the ConsumerRef if it references some other bareMetalMachine
		if !consumerRefMatches(host.Spec.ConsumerRef, s.scope.BareMetalMachine) {
			s.scope.Info("host already associated with another bareMetalMachine",
				"host", host.Name)
			// Remove the ownerreference to this machine, even if the consumer ref
			// references another machine.
			host.OwnerReferences, err = s.DeleteOwnerRef(host.OwnerReferences)
			if err != nil {
				return nil, err
			}
			return nil, err
		}

		if removeMachineSpecsFromHost(host) {
			// Update the BMH object, if the errors are NotFound, do not return the
			// errors.
			if err := patchIfFound(ctx, helper, host); err != nil {
				return nil, err
			}

			s.scope.Info("Patched BaremetalHost while deprovisioning, requeuing")
			return nil, &scope.RequeueAfterError{}
		}

		if host.Spec.Status.ProvisioningState != infrav1.StateNone {
			s.scope.Info("Deprovisioning BaremetalHost, requeuing", "host.Spec.Status.ProvisioningState", host.Spec.Status.ProvisioningState)
			return nil, &scope.RequeueAfterError{RequeueAfter: requeueAfter}
		}

		host.Spec.ConsumerRef = nil
		host.Spec.Status.HetznerClusterRef = ""
		host.SetDeletionTimestamp(nil)

		// Remove the ownerreference to this machine.
		host.OwnerReferences, err = s.DeleteOwnerRef(host.OwnerReferences)
		if err != nil {
			return nil, err
		}

		if host.Labels != nil && host.Labels[capi.ClusterLabelName] == s.scope.Machine.Spec.ClusterName {
			delete(host.Labels, capi.ClusterLabelName)
		}

		// Update the BMH object, if the errors are NotFound, do not return the
		// errors.
		if err := patchIfFound(ctx, helper, host); err != nil {
			return nil, err
		}
	}

	s.scope.Info("finished deleting bareMetalMachine")

	record.Eventf(
		s.scope.BareMetalMachine,
		"BareMetalMachineDeleted",
		"Bare metal machine with name %s deleted",
		s.scope.Name(),
	)
	return nil, nil
}

func removeMachineSpecsFromHost(host *infrav1.HetznerBareMetalHost) (updatedHost bool) {
	if host.Spec.Status.InstallImage != nil {
		host.Spec.Status.InstallImage = nil
		updatedHost = true
	}
	if host.Spec.Status.UserData != nil {
		host.Spec.Status.UserData = nil
		updatedHost = true
	}
	if host.Spec.Status.SSHSpec != nil {
		host.Spec.Status.SSHSpec = nil
		updatedHost = true
	}
	var emptySSHStatus = infrav1.SSHStatus{}
	if host.Spec.Status.SSHStatus != emptySSHStatus {
		host.Spec.Status.SSHStatus = emptySSHStatus
		updatedHost = true
	}
	return updatedHost
}

// update updates a machine and is invoked by the Machine Controller.
func (s *Service) update(ctx context.Context, log logr.Logger) error {
	log.V(1).Info("Updating machine")

	host, helper, err := s.getHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		return errors.Errorf("host not found for machine %s", s.scope.Machine.Name)
	}

	if host.Spec.MaintenanceMode && s.scope.BareMetalMachine.Status.FailureReason == nil {
		s.scope.BareMetalMachine.SetFailure(capierrors.UpdateMachineError, "host machine in maintenance mode")
		record.Eventf(
			s.scope.BareMetalMachine,
			"BareMetalMachineSetFailure",
			"set failure reason due to maintenance mode of underlying host",
		)
		return nil
	}

	if host.Spec.Status.ErrorType == infrav1.FatalError && s.scope.BareMetalMachine.Status.FailureReason == nil {
		s.scope.BareMetalMachine.SetFailure(capierrors.UpdateMachineError, host.Spec.Status.ErrorMessage)
		record.Eventf(
			s.scope.BareMetalMachine,
			"BareMetalMachineSetFailure",
			host.Spec.Status.ErrorMessage,
		)
		return nil
	}

	if host.Spec.Status.ErrorType == infrav1.ErrorType("") {
		s.scope.BareMetalMachine.Status.FailureMessage = nil
		s.scope.BareMetalMachine.Status.FailureReason = nil
	}

	// ensure that the host's consumer ref is correctly set
	err = s.setHostConsumerRef(host)
	if err != nil {
		if _, ok := err.(scope.HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to associate the BaremetalHost to the Hetzner bare metalMachine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// ensure that the host's specs are correctly set
	s.setHostSpec(host)

	err = helper.Patch(ctx, host)
	if err != nil {
		return err
	}

	if s.scope.IsControlPlane() {
		if err := s.reconcileLoadBalancerAttachment(ctx, host); err != nil {
			return errors.Wrap(err, "failed to reconcile load balancer attachement")
		}
	}

	err = s.ensureAnnotation(host)
	if err != nil {
		return err
	}

	s.updateMachineStatus(host)

	log.Info("Finished updating machine")
	return nil
}

func (s *Service) reconcileLoadBalancerAttachment(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	log := ctrl.LoggerFrom(ctx)

	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		return nil
	}

	// IPv4 and IPv6 might be empty
	var foundIPv4 bool
	if host.Spec.Status.IPv4 == "" {
		foundIPv4 = true
	}
	var foundIPv6 bool
	if host.Spec.Status.IPv6 == "" {
		foundIPv6 = true
	}

	for _, target := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
		if target.Type == infrav1.LoadBalancerTargetTypeIP {
			if target.IP == host.Spec.Status.IPv4 {
				foundIPv4 = true
			} else if target.IP == host.Spec.Status.IPv6 {
				foundIPv6 = true
			}
		}
	}

	// If already attached do nothing
	if foundIPv4 && foundIPv6 {
		return nil
	}

	log.V(1).Info("Reconciling load balancer attachement", "targets", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target)

	targetIPs := make([]string, 0, 2)
	if !foundIPv4 {
		targetIPs = append(targetIPs, host.Spec.Status.IPv4)
	}
	if !foundIPv6 {
		targetIPs = append(targetIPs, host.Spec.Status.IPv6)
	}

	for _, ip := range targetIPs {
		loadBalancerAddIPTargetOpts := hcloud.LoadBalancerAddIPTargetOpts{
			IP: net.ParseIP(ip),
		}

		if _, err := s.scope.HCloudClient.AddIPTargetToLoadBalancer(
			ctx,
			loadBalancerAddIPTargetOpts,
			&hcloud.LoadBalancer{
				ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
			}); err != nil {
			if hcloud.IsError(err, hcloud.ErrorCodeTargetAlreadyDefined) {
				return nil
			}
			s.scope.V(1).Info("Could not add ip as target to load balancer",
				"Server", host.Spec.ServerID, "ip", ip, "Load Balancer", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
			return err
		}

		record.Eventf(
			s.scope.HetznerCluster,
			"AddedAsTargetToLoadBalancer",
			"Added new target with server number %d and with ip %s to the loadbalancer %v",
			host.Spec.ServerID, ip, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
	}
	return nil
}

func (s *Service) deleteServerOfLoadBalancer(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	if host.Spec.Status.IPv4 != "" {
		if _, err := s.scope.HCloudClient.DeleteIPTargetOfLoadBalancer(
			ctx,
			&hcloud.LoadBalancer{
				ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
			},
			net.ParseIP(host.Spec.Status.IPv4)); err != nil {
			if !strings.Contains(err.Error(), "load_balancer_target_not_found") {
				s.scope.Info("Could not delete server IPv4 as target of load balancer",
					"Server", host.Spec.ServerID, "IP", host.Spec.Status.IPv4, "Load Balancer", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
				return err
			}
		}
		record.Eventf(
			s.scope.HetznerCluster,
			"DeletedTargetOfLoadBalancer",
			"Deleted new server with id %d and IPv4 %s of the loadbalancer %v",
			host.Spec.ServerID, host.Spec.Status.IPv4, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
	}

	if host.Spec.Status.IPv6 != "" {
		if _, err := s.scope.HCloudClient.DeleteIPTargetOfLoadBalancer(
			ctx,
			&hcloud.LoadBalancer{
				ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
			},
			net.ParseIP(host.Spec.Status.IPv6)); err != nil {
			if !strings.Contains(err.Error(), "load_balancer_target_not_found") {
				s.scope.Info("Could not delete server IPv6 as target of load balancer",
					"Server", host.Spec.ServerID, "IP", host.Spec.Status.IPv6, "Load Balancer", s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
				return err
			}
		}
		record.Eventf(
			s.scope.HetznerCluster,
			"DeletedTargetOfLoadBalancer",
			"Deleted new server with id %d and IPv6 %s of the loadbalancer %v",
			host.Spec.ServerID, host.Spec.Status.IPv6, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)
	}

	return nil
}

func (s *Service) associate(ctx context.Context, log logr.Logger) error {
	log.Info("Associating machine", "machine", s.scope.Machine.Name)

	// look for associated BMH
	host, helper, err := s.getHost(ctx)
	if err != nil {
		s.scope.SetError("Failed to get the BaremetalHost for the HetznerBareMetalMachine",
			capierrors.CreateMachineError,
		)
		return err
	}

	// no BMH found, trying to choose from available ones
	if host == nil {
		host, helper, err = s.chooseHost(ctx)
		if err != nil {
			if _, ok := err.(scope.HasRequeueAfterError); !ok {
				s.scope.SetError("Failed to pick a BaremetalHost for the HetznerBareMetalMachine",
					capierrors.CreateMachineError,
				)
			}
			return err
		}
		if host == nil {
			log.Info("No available host found. Requeuing.")
			return &scope.RequeueAfterError{RequeueAfter: requeueAfter}
		}
		log.Info("Associating machine with host", "host", host.Name)
	}

	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}
	host.Labels[capi.ClusterLabelName] = s.scope.Machine.Spec.ClusterName

	err = s.setHostConsumerRef(host)
	if err != nil {
		if _, ok := err.(scope.HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to associate the BaremetalHost to the HetznerBareMetalMachine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// ensure that the host's specs are correctly set
	s.setHostSpec(host)

	err = helper.Patch(ctx, host)
	if err != nil {
		if aggr, ok := err.(kerrors.Aggregate); ok {
			for _, kerr := range aggr.Errors() {
				if apierrors.IsConflict(kerr) {
					return &scope.RequeueAfterError{}
				}
			}
		}
		return err
	}

	err = s.ensureAnnotation(host)
	if err != nil {
		if _, ok := err.(scope.HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to annotate the HetznerBareMetalMachine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	log.Info("Finished associating machine")
	return nil
}

// getHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (s *Service) getHost(ctx context.Context) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	annotations := s.scope.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return nil, nil, nil
	}
	hostKey, ok := annotations[infrav1.HostAnnotation]
	if !ok {
		return nil, nil, nil
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		s.scope.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return nil, nil, err
	}

	host := infrav1.HetznerBareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = s.scope.Client.Get(ctx, key, &host)
	if apierrors.IsNotFound(err) {
		s.scope.Info("Annotated host not found", "host", hostKey)
		return nil, nil, nil
	} else if err != nil {
		return nil, nil, err
	}
	helper, err := patch.NewHelper(&host, s.scope.Client)
	return &host, helper, err
}

func (s *Service) chooseHost(ctx context.Context) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	// get list of BMH
	hosts := infrav1.HetznerBareMetalHostList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: s.scope.BareMetalMachine.Namespace,
	}

	if err := s.scope.Client.List(ctx, &hosts, opts); err != nil {
		return nil, nil, errors.Wrap(err, "failed to list hosts")
	}

	labelSelector, err := s.getLabelSelector()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get label selector")
	}

	availableHosts := []*infrav1.HetznerBareMetalHost{}

	for i, host := range hosts.Items {
		if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, s.scope.BareMetalMachine) {
			s.scope.Info("Found host with existing ConsumerRef", "host", host.Name)
			helper, err := patch.NewHelper(&hosts.Items[i], s.scope.Client)
			return &hosts.Items[i], helper, err
		}
		if host.Spec.ConsumerRef != nil {
			continue
		}
		if host.Spec.MaintenanceMode {
			continue
		}
		if host.GetDeletionTimestamp() != nil {
			continue
		}
		if host.Spec.Status.ErrorMessage != "" {
			continue
		}

		if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
			if host.Spec.Status.ProvisioningState == infrav1.StateNone {
				s.scope.Info(fmt.Sprintf("Host %v matched hostSelector for HetznerBareMetalMachine, adding it to availableHosts list", host.Name))
				availableHosts = append(availableHosts, &hosts.Items[i])
			}
		} else {
			s.scope.Info(fmt.Sprintf("Host %v did not match hostSelector for HetznerBareMetalMachine", host.Name))
		}
	}

	s.scope.Info(fmt.Sprintf("%d hosts available while choosing host for HetznerBareMetal machine", len(availableHosts)))
	if len(availableHosts) == 0 {
		return nil, nil, nil
	}

	// choose a host
	rand.Seed(time.Now().Unix())
	s.scope.Info(fmt.Sprintf("%d host(s) available, choosing a random host", len(availableHosts)))
	chosenHost := availableHosts[rand.Intn(len(availableHosts))] // #nosec

	helper, err := patch.NewHelper(chosenHost, s.scope.Client)
	return chosenHost, helper, err
}

func (s *Service) getLabelSelector() (labels.Selector, error) {
	labelSelector := labels.NewSelector()
	var reqs labels.Requirements

	for labelKey, labelVal := range s.scope.BareMetalMachine.Spec.HostSelector.MatchLabels {
		r, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal})
		if err != nil {
			s.scope.Error(err, "Failed to create MatchLabel requirement, not choosing host")
			return nil, err
		}
		reqs = append(reqs, *r)
	}
	for _, req := range s.scope.BareMetalMachine.Spec.HostSelector.MatchExpressions {
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		r, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values)
		if err != nil {
			s.scope.Error(err, "Failed to create MatchExpression requirement, not choosing host")
			return nil, err
		}
		reqs = append(reqs, *r)
	}

	labelSelector = labelSelector.Add(reqs...)

	return labelSelector, nil
}

// GetProviderIDAndBMHID returns providerID and bmhID.
func (s *Service) GetProviderIDAndBMHID() (string, *string) {
	providerID := s.scope.BareMetalMachine.Spec.ProviderID
	if providerID == nil {
		return "", nil
	}
	return *providerID, pointer.StringPtr(parseProviderID(*providerID))
}

// GetBaremetalHostID return the provider identifier for this machine.
func (s *Service) GetBaremetalHostID(ctx context.Context) (*string, error) {
	// look for associated BMH
	host, _, err := s.getHost(ctx)
	if err != nil {
		s.scope.SetError("Failed to get a BaremetalHost for the BareMetalMachine",
			capierrors.CreateMachineError,
		)
		return nil, err
	}
	if host == nil {
		s.scope.Logger.Info("BaremetalHost not associated, requeuing")
		return nil, &scope.RequeueAfterError{RequeueAfter: requeueAfter}
	}
	if host.Spec.Status.ProvisioningState == infrav1.StateProvisioned {
		return pointer.StringPtr(strconv.Itoa(host.Spec.ServerID)), nil
	}
	s.scope.Logger.Info("Provisioning BaremetalHost, requeuing")
	// Do not requeue since BMH update will trigger a reconciliation
	return nil, nil
}

// setHostSpec will ensure the host's Spec is set according to the machine's
// details. It will then update the host via the kube API.
func (s *Service) setHostSpec(host *infrav1.HetznerBareMetalHost) {
	// We only want to update the image setting if the host does not
	// already have an image.
	//
	// A host with an existing image is already provisioned and
	// upgrades are not supported at this time. To re-provision a
	// host, we must fully deprovision it and then provision it again.
	// Not provisioning while we do not have the UserData.

	if host.Spec.Status.InstallImage == nil && s.scope.Machine.Spec.Bootstrap.DataSecretName != nil {
		host.Spec.Status.InstallImage = &s.scope.BareMetalMachine.Spec.InstallImage
		host.Spec.Status.UserData = &corev1.SecretReference{Namespace: s.scope.Namespace(), Name: *s.scope.Machine.Spec.Bootstrap.DataSecretName}
		host.Spec.Status.SSHSpec = &s.scope.BareMetalMachine.Spec.SSHSpec
		host.Spec.Status.HetznerClusterRef = s.scope.HetznerCluster.Name
	}
}

func patchIfFound(ctx context.Context, helper *patch.Helper, host client.Object) error {
	err := helper.Patch(ctx, host)
	if err != nil {
		notFound := true
		var aggr kerrors.Aggregate
		if ok := errors.As(err, &aggr); ok {
			for _, kerr := range aggr.Errors() {
				if !apierrors.IsNotFound(kerr) {
					notFound = false
				}
				if apierrors.IsConflict(kerr) {
					return &scope.RequeueAfterError{}
				}
			}
		} else {
			notFound = false
		}
		if notFound {
			return nil
		}
	}
	return err
}

// setHostConsumerRef will ensure the host's Spec is set to link to this Hetzner bare metalMachine.
func (s *Service) setHostConsumerRef(host *infrav1.HetznerBareMetalHost) error {
	if host.Spec.ConsumerRef == nil || host.Spec.ConsumerRef.Name != s.scope.BareMetalMachine.Name {
		host.Spec.ConsumerRef = &corev1.ObjectReference{
			Kind:       "HetznerBareMetalMachine",
			Name:       s.scope.BareMetalMachine.Name,
			Namespace:  s.scope.BareMetalMachine.Namespace,
			APIVersion: s.scope.BareMetalMachine.APIVersion,
		}
	}
	// Set OwnerReferences
	hostOwnerReferences, err := s.SetOwnerRef(host.OwnerReferences, true)
	if err != nil {
		return err
	}
	host.OwnerReferences = hostOwnerReferences

	return nil
}

// updateMachineStatus updates a HetznerBareMetalMachine object's status.
func (s *Service) updateMachineStatus(host *infrav1.HetznerBareMetalHost) {
	addrs := nodeAddresses(host, s.scope.Name())

	bareMetalMachineOld := s.scope.BareMetalMachine.DeepCopy()

	s.scope.BareMetalMachine.Status.Addresses = addrs
	conditions.MarkTrue(s.scope.BareMetalMachine, infrav1.AssociateBMHCondition)

	// Update lastUpdated when status changed
	if !equality.Semantic.DeepEqual(s.scope.BareMetalMachine.Status, bareMetalMachineOld.Status) {
		now := metav1.Now()
		s.scope.BareMetalMachine.Status.LastUpdated = &now
	}
}

// NodeAddresses returns a slice of corev1.NodeAddress objects for a
// given HetznerBareMetal machine.
func nodeAddresses(host *infrav1.HetznerBareMetalHost, bareMetalMachineName string) []corev1.NodeAddress {
	addrs := []corev1.NodeAddress{}

	// If the host is nil or we have no hw details, return an empty address array.
	if host == nil || host.Spec.Status.HardwareDetails == nil {
		return addrs
	}

	for _, nic := range host.Spec.Status.HardwareDetails.NIC {
		address := corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: nic.IP,
		}
		addrs = append(addrs, address)
	}

	// Add hostname == bareMetalMachineName as well
	addrs = append(addrs, corev1.NodeAddress{
		Type:    corev1.NodeHostName,
		Address: bareMetalMachineName,
	})
	addrs = append(addrs, corev1.NodeAddress{
		Type:    corev1.NodeInternalDNS,
		Address: bareMetalMachineName,
	})

	return addrs
}

// consumerRefMatches returns a boolean based on whether the consumer
// reference and bare metal machine metadata match.
func consumerRefMatches(consumer *corev1.ObjectReference, bmMachine *infrav1.HetznerBareMetalMachine) bool {
	if consumer.Name != bmMachine.Name {
		return false
	}
	if consumer.Namespace != bmMachine.Namespace {
		return false
	}
	if consumer.Kind != bmMachine.Kind {
		return false
	}
	if consumer.GroupVersionKind().Group != bmMachine.GroupVersionKind().Group {
		return false
	}
	return true
}

// SetOwnerRef adds an ownerreference to this Hetzner bare metal machine.
func (s *Service) SetOwnerRef(refList []metav1.OwnerReference, controller bool) ([]metav1.OwnerReference, error) {
	return setOwnerRefInList(refList, controller, s.scope.BareMetalMachine.TypeMeta,
		s.scope.BareMetalMachine.ObjectMeta,
	)
}

// SetOwnerRef adds an ownerreference to this Hetzner bare metal machine.
func setOwnerRefInList(refList []metav1.OwnerReference, controller bool,
	objType metav1.TypeMeta, objMeta metav1.ObjectMeta,
) ([]metav1.OwnerReference, error) {
	index, err := findOwnerRefFromList(refList, objType, objMeta)
	if err != nil {
		if _, ok := err.(*NotFoundError); !ok {
			return nil, err
		}
		refList = append(refList, metav1.OwnerReference{
			APIVersion: objType.APIVersion,
			Kind:       objType.Kind,
			Name:       objMeta.Name,
			UID:        objMeta.UID,
			Controller: pointer.BoolPtr(controller),
		})
	} else {
		// The UID and the APIVersion might change due to move or version upgrade
		refList[index].APIVersion = objType.APIVersion
		refList[index].UID = objMeta.UID
		refList[index].Controller = pointer.BoolPtr(controller)
	}
	return refList, nil
}

// DeleteOwnerRef removes the ownerreference to this BareMetalMachine.
func (s *Service) DeleteOwnerRef(refList []metav1.OwnerReference) ([]metav1.OwnerReference, error) {
	return deleteOwnerRefFromList(refList, s.scope.BareMetalMachine.TypeMeta,
		s.scope.BareMetalMachine.ObjectMeta,
	)
}

// DeleteOwnerRefFromList removes the ownerreference to this BareMetalMachine.
func deleteOwnerRefFromList(refList []metav1.OwnerReference,
	objType metav1.TypeMeta, objMeta metav1.ObjectMeta,
) ([]metav1.OwnerReference, error) {
	if len(refList) == 0 {
		return refList, nil
	}
	index, err := findOwnerRefFromList(refList, objType, objMeta)
	if err != nil {
		if _, ok := err.(*NotFoundError); !ok {
			return nil, err
		}
		return refList, nil
	}
	if len(refList) == 1 {
		return []metav1.OwnerReference{}, nil
	}
	refListLen := len(refList) - 1
	refList[index] = refList[refListLen]
	refList, err = deleteOwnerRefFromList(refList[:refListLen], objType, objMeta)
	if err != nil {
		return nil, err
	}
	return refList, nil
}

// findOwnerRefFromList finds OwnerRef to this Hetzner bare metal machine.
func findOwnerRefFromList(refList []metav1.OwnerReference, objType metav1.TypeMeta,
	objMeta metav1.ObjectMeta,
) (int, error) {
	for i, curOwnerRef := range refList {
		aGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return 0, err
		}

		bGV, err := schema.ParseGroupVersion(objType.APIVersion)
		if err != nil {
			return 0, err
		}
		// not matching on UID since when pivoting it might change
		// Not matching on API version as this might change
		if curOwnerRef.Name == objMeta.Name &&
			curOwnerRef.Kind == objType.Kind &&
			aGV.Group == bGV.Group {
			return i, nil
		}
	}
	return 0, &NotFoundError{}
}

// ensureAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (s *Service) ensureAnnotation(host *infrav1.HetznerBareMetalHost) error {
	annotations := s.scope.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	hostKey, err := cache.MetaNamespaceKeyFunc(host)
	if err != nil {
		s.scope.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return err
	}
	existing, ok := annotations[infrav1.HostAnnotation]
	if ok {
		if existing == hostKey {
			return nil
		}
		s.scope.Info("Warning: found stray annotation for host on machine. Overwriting.", "host", existing)
	}
	annotations[infrav1.HostAnnotation] = hostKey
	s.scope.BareMetalMachine.ObjectMeta.SetAnnotations(annotations)

	return nil
}

func (s *Service) checkMachineError(err error, errMessage string, errType capierrors.MachineStatusError) (*ctrl.Result, error) {
	if err == nil {
		return &ctrl.Result{}, nil
	}
	if requeueErr, ok := errors.Cause(err).(scope.HasRequeueAfterError); ok {
		return &ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
	}
	s.scope.SetError(errMessage, errType)
	return &ctrl.Result{}, errors.Wrap(err, errMessage)
}

// NotFoundError represents that an object was not found.
type NotFoundError struct {
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	return "Object not found"
}

func parseProviderID(providerID string) string {
	return strings.TrimPrefix(providerID, ProviderIDPrefix)
}
