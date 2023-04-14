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

// Package baremetal implements functions to manage the lifecycle of baremetal machines as inventory.
package baremetal

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TODO: Implement logic for removal of unpaid servers.

const (
	// providerIDPrefix is a prefix for ProviderID.
	providerIDPrefix = "hcloud://"

	// requeueAfter gives the duration of time until the next reconciliation should be performed.
	requeueAfter = time.Second * 30

	// FailureMessageMaintenanceMode indicates that host is in maintenance mode.
	FailureMessageMaintenanceMode = "host machine in maintenance mode"
)

// Service defines struct with machine scope to reconcile HetznerBareMetalMachines.
type Service struct {
	scope *scope.BareMetalMachineScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.BareMetalMachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HetznerBareMetalMachines.
func (s *Service) Reconcile(ctx context.Context) (res reconcile.Result, err error) {
	// Make sure bootstrap data is available and populated. If not, return, we
	// will get an event from the machine update when the flag is set to true.
	if !s.scope.IsBootstrapReady(ctx) {
		conditions.MarkFalse(
			s.scope.BareMetalMachine,
			infrav1.InstanceBootstrapReadyCondition,
			infrav1.InstanceBootstrapNotReadyReason,
			clusterv1.ConditionSeverityInfo,
			"bootstrap not ready yet",
		)
		return res, nil
	}

	conditions.MarkTrue(
		s.scope.BareMetalMachine,
		infrav1.InstanceBootstrapReadyCondition,
	)

	// Check if the bareMetalmachine is associated with a host already. If not, associate a new host.
	if !s.scope.BareMetalMachine.HasHostAnnotation() {
		err := s.associate(ctx)
		if err != nil {
			return checkForRequeueError(err, "failed to associate machine to a host")
		}
	}

	// update the machine
	if err := s.update(ctx); err != nil {
		return checkForRequeueError(err, "failed to update machine")
	}

	// set providerID if necessary
	if err := s.setProviderID(ctx); err != nil {
		return res, fmt.Errorf("failed to set providerID: %w", err)
	}

	// set machine ready
	s.scope.BareMetalMachine.Status.Ready = true
	conditions.MarkTrue(s.scope.BareMetalMachine, infrav1.InstanceReadyCondition)

	return res, nil
}

// Delete implements delete method of bare metal machine.
func (s *Service) Delete(ctx context.Context) (res reconcile.Result, err error) {
	// get host - ignore if not found
	host, helper, err := s.getAssociatedHost(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return res, fmt.Errorf("failed to get associated host: %w", err)
	}

	if host != nil && host.Spec.ConsumerRef != nil {
		// remove control plane as load balancer target
		if s.scope.IsControlPlane() && s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled {
			if err := s.removeAttachedServerOfLoadBalancer(ctx, host); err != nil {
				return res, fmt.Errorf("failed to delete attached server of load balancer: %w", err)
			}
		}

		// don't remove the consumerRef if it references some other HetznerBareMetalMachine
		if !consumerRefMatches(host.Spec.ConsumerRef, s.scope.BareMetalMachine) {
			// remove the ownerRef to this host, even if the consumerRef references another machine
			host.OwnerReferences = s.removeOwnerRef(host.OwnerReferences)

			return res, nil
		}

		if removeMachineSpecsFromHost(host) {
			// Patch the host object. If the error is NotFound, do not return the error.
			if err := analyzePatchError(helper.Patch(ctx, host), true); err != nil {
				return checkForRequeueError(err, "failed to patch host")
			}

			return reconcile.Result{Requeue: true}, nil
		}

		// check if deprovisioning is done
		if host.Spec.Status.ProvisioningState != infrav1.StateNone {
			return reconcile.Result{RequeueAfter: requeueAfter}, nil
		}

		// deprovisiong is done - remove all references of host
		host.Spec.ConsumerRef = nil
		host.Spec.Status.HetznerClusterRef = ""
		host.SetDeletionTimestamp(nil)
		host.OwnerReferences = s.removeOwnerRef(host.OwnerReferences)

		// remove labels
		if host.Labels != nil && host.Labels[clusterv1.ClusterNameLabel] == s.scope.Machine.Spec.ClusterName {
			delete(host.Labels, clusterv1.ClusterNameLabel)
		}

		// patch host object
		if err := analyzePatchError(helper.Patch(ctx, host), true); err != nil {
			return checkForRequeueError(err, "failed to patch host")
		}
	}

	record.Eventf(
		s.scope.BareMetalMachine,
		"BareMetalMachineDeleted",
		"HetznerBareMetalMachine with name %s deleted",
		s.scope.Name(),
	)
	return res, nil
}

// update updates a machine and is invoked by the Machine Controller.
func (s *Service) update(ctx context.Context) error {
	host, helper, err := s.getAssociatedHost(ctx)
	if err != nil {
		return fmt.Errorf("failed to get host: %w", err)
	}
	if host == nil {
		s.scope.BareMetalMachine.SetFailure(capierrors.UpdateMachineError, "host not found")
		return fmt.Errorf("host not found for machine %s: %w", s.scope.Machine.Name, err)
	}

	// maintenance mode on the host is a fatal error for the machine object
	if host.Spec.MaintenanceMode != nil && *host.Spec.MaintenanceMode && s.scope.BareMetalMachine.Status.FailureReason == nil {
		s.scope.BareMetalMachine.SetFailure(capierrors.UpdateMachineError, FailureMessageMaintenanceMode)
		record.Eventf(
			s.scope.BareMetalMachine,
			"BareMetalMachineSetFailure",
			"set failure reason due to maintenance mode of underlying host",
		)
		return nil
	}

	// if host has a fatal error, then it should be set on the machine object as well
	if host.Spec.Status.ErrorType == infrav1.FatalError && s.scope.BareMetalMachine.Status.FailureReason == nil {
		s.scope.BareMetalMachine.SetFailure(capierrors.UpdateMachineError, host.Spec.Status.ErrorMessage)
		record.Eventf(s.scope.BareMetalMachine, "BareMetalMachineSetFailure", host.Spec.Status.ErrorMessage)
		return nil
	}

	// if host is healthy, the machine is healthy as well
	if host.Spec.Status.ErrorType == infrav1.ErrorType("") {
		s.scope.BareMetalMachine.Status.FailureMessage = nil
		s.scope.BareMetalMachine.Status.FailureReason = nil
	}

	// ensure that the references are correctly set on host
	s.setReferencesOnHost(host)

	// ensure that the specs of the host are correctly set
	s.setHostSpec(host)

	// ensure cluster label on host
	ensureClusterLabel(host, s.scope.Machine.Spec.ClusterName)

	if err := analyzePatchError(helper.Patch(ctx, host), false); err != nil {
		return fmt.Errorf("failed to patch host: %w", err)
	}

	// if machine is a control plane, the host should be set as target of load balancer
	if s.scope.IsControlPlane() {
		if err := s.reconcileLoadBalancerAttachment(ctx, host); err != nil {
			return fmt.Errorf("failed to reconcile load balancer attachement: %w", err)
		}
	}

	// ensure annotations are correctly set
	s.ensureMachineAnnotation(host)

	// update status of HetznerBareMetalMachine with infos from host
	s.updateMachineAddresses(host)
	return nil
}

func (s *Service) associate(ctx context.Context) error {
	// look for associated host
	associatedHost, _, err := s.getAssociatedHost(ctx)
	if err != nil {
		return fmt.Errorf("failed to get associated host: %w", err)
	}

	// if host is found, there is nothing to do
	if associatedHost != nil {
		return nil
	}

	// choose new host
	host, helper, err := s.chooseHost(ctx)
	if err != nil {
		return fmt.Errorf("failed to choose host: %w", err)
	}
	if host == nil {
		s.scope.V(1).Info("No available host found. Requeuing.")
		return &scope.RequeueAfterError{RequeueAfter: requeueAfter}
	}

	// ensure cluster label on host
	ensureClusterLabel(host, s.scope.Machine.Spec.ClusterName)

	// ensure references are set
	s.setReferencesOnHost(host)

	// ensure that the specs are correctly updated
	s.setHostSpec(host)

	if err := analyzePatchError(helper.Patch(ctx, host), false); err != nil {
		return fmt.Errorf("failed to patch host: %w", err)
	}

	s.ensureMachineAnnotation(host)
	return nil
}

// getAssociatedHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (s *Service) getAssociatedHost(ctx context.Context) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	annotations := s.scope.BareMetalMachine.ObjectMeta.GetAnnotations()
	// if no annotations exist on machine, no host can be associated
	if annotations == nil {
		return nil, nil, nil
	}

	// check if host annotation is set and return if not
	hostKey, ok := annotations[infrav1.HostAnnotation]
	if !ok {
		return nil, nil, nil
	}

	// find associated host object and return it
	hostNamespace, hostName := splitHostKey(hostKey)

	host := infrav1.HetznerBareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}

	if err := s.scope.Client.Get(ctx, key, &host); err != nil {
		return nil, nil, fmt.Errorf("failed to get host object: %w", err)
	}

	helper, err := patch.NewHelper(&host, s.scope.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create patch helper: %w", err)
	}

	return &host, helper, nil
}

func (s *Service) chooseHost(ctx context.Context) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	// get list of hosts scoped to namespace of machine
	hosts := infrav1.HetznerBareMetalHostList{}
	opts := &client.ListOptions{
		Namespace: s.scope.BareMetalMachine.Namespace,
	}

	if err := s.scope.Client.List(ctx, &hosts, opts); err != nil {
		return nil, nil, fmt.Errorf("failed to list hosts: %w", err)
	}

	labelSelector := s.getLabelSelector()

	availableHosts := make([]*infrav1.HetznerBareMetalHost, 0, len(hosts.Items))

	for i, host := range hosts.Items {
		if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, s.scope.BareMetalMachine) {
			helper, err := patch.NewHelper(&hosts.Items[i], s.scope.Client)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create patch helper: %w", err)
			}
			return &hosts.Items[i], helper, nil
		}
		if host.Spec.ConsumerRef != nil {
			continue
		}
		if host.Spec.MaintenanceMode != nil && *host.Spec.MaintenanceMode {
			continue
		}
		if host.GetDeletionTimestamp() != nil {
			continue
		}
		if host.Spec.Status.ErrorMessage != "" {
			continue
		}

		if !labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
			continue
		}

		if host.Spec.Status.ProvisioningState != infrav1.StateNone {
			continue
		}

		availableHosts = append(availableHosts, &hosts.Items[i])
	}

	if len(availableHosts) == 0 {
		return nil, nil, nil
	}

	// choose a host
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(availableHosts))))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create random number: %w", err)
	}

	chosenHost := availableHosts[randomNumber.Int64()]

	helper, err := patch.NewHelper(chosenHost, s.scope.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create patch helper: %w", err)
	}

	return chosenHost, helper, nil
}

func (s *Service) reconcileLoadBalancerAttachment(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		return nil
	}

	// check whether IPs of host have been added as load balancer targets already
	var foundIPv4 bool
	var foundIPv6 bool

	for _, target := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
		if target.Type == infrav1.LoadBalancerTargetTypeIP {
			if target.IP == host.Spec.Status.IPv4 {
				foundIPv4 = true
			} else if target.IP == host.Spec.Status.IPv6 {
				foundIPv6 = true
			}
		}
	}

	// IPv4 or IPv6 of host might be empty, in that case we don't want to add them
	if host.Spec.Status.IPv4 == "" {
		foundIPv4 = true
	}
	if host.Spec.Status.IPv6 == "" {
		foundIPv6 = true
	}

	// if both IPs are already added as target, then do nothing
	if foundIPv4 && foundIPv6 {
		return nil
	}

	newIPTargets := make([]string, 0, 2)
	if !foundIPv4 {
		newIPTargets = append(newIPTargets, host.Spec.Status.IPv4)
	}
	if !foundIPv6 {
		newIPTargets = append(newIPTargets, host.Spec.Status.IPv6)
	}

	lb := &hcloud.LoadBalancer{ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID}

	for _, ip := range newIPTargets {
		opts := hcloud.LoadBalancerAddIPTargetOpts{
			IP: net.ParseIP(ip),
		}

		if err := s.scope.HCloudClient.AddIPTargetToLoadBalancer(ctx, opts, lb); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "AddIPTargetToLoadBalancer")
			if hcloud.IsError(err, hcloud.ErrorCodeTargetAlreadyDefined) {
				return nil
			}
			return fmt.Errorf("failed to add IP %q as target to load balancer: %w", ip, err)
		}
	}

	record.Eventf(
		s.scope.HetznerCluster,
		"AddedAsTargetToLoadBalancer",
		"Added IPs of server %d as targets to the loadbalancer %v",
		host.Spec.ServerID, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
	)

	return nil
}

func (s *Service) removeAttachedServerOfLoadBalancer(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	lb := &hcloud.LoadBalancer{ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID}

	// remove host IPv4 as target
	if host.Spec.Status.IPv4 != "" {
		if err := s.scope.HCloudClient.DeleteIPTargetOfLoadBalancer(ctx, lb, net.ParseIP(host.Spec.Status.IPv4)); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeleteIPTargetOfLoadBalancer")
			// ignore not found errors
			if !strings.Contains(err.Error(), "load_balancer_target_not_found") {
				return fmt.Errorf("failed to remove IPv4 %v as target of load balancer: %w", host.Spec.Status.IPv4, err)
			}
		}
	}

	// remove host IPv6 as target
	if host.Spec.Status.IPv6 != "" {
		if err := s.scope.HCloudClient.DeleteIPTargetOfLoadBalancer(ctx, lb, net.ParseIP(host.Spec.Status.IPv6)); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeleteIPTargetOfLoadBalancer")
			// ignore not found errors
			if !strings.Contains(err.Error(), "load_balancer_target_not_found") {
				return fmt.Errorf("failed to remove IPv6 %v as target of load balancer: %w", host.Spec.Status.IPv6, err)
			}
		}
	}

	record.Eventf(
		s.scope.HetznerCluster,
		"DeletedTargetOfLoadBalancer",
		"Deleted IPs of server %d as targets of the loadbalancer %v",
		host.Spec.ServerID, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
	)
	return nil
}

func (s *Service) getLabelSelector() labels.Selector {
	labelSelector := labels.NewSelector()
	var reqs labels.Requirements

	for labelKey, labelVal := range s.scope.BareMetalMachine.Spec.HostSelector.MatchLabels {
		r, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal})
		if err == nil { // ignore invalid host selector
			reqs = append(reqs, *r)
		}
	}
	for _, req := range s.scope.BareMetalMachine.Spec.HostSelector.MatchExpressions {
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		r, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values)
		if err == nil { // ignore invalid host selector
			reqs = append(reqs, *r)
		}
	}

	return labelSelector.Add(reqs...)
}

func (s *Service) setProviderID(ctx context.Context) error {
	// nothing to do if providerID is set
	if s.scope.BareMetalMachine.Spec.ProviderID != nil {
		return nil
	}

	// providerID is based on the ID of the host machine
	host, _, err := s.getAssociatedHost(ctx)
	if err != nil {
		return fmt.Errorf("failed to get host: %w", err)
	}

	if host == nil {
		s.scope.BareMetalMachine.SetFailure(capierrors.UpdateMachineError, "host not found")
		return fmt.Errorf("host not found for machine %s: %w", s.scope.Machine.Name, err)
	}

	if host.Spec.Status.ProvisioningState != infrav1.StateProvisioned {
		// no need for requeue error since host update will trigger a reconciliation
		return nil
	}

	// set providerID
	providerID := providerIDFromServerID(host.Spec.ServerID)
	s.scope.BareMetalMachine.Spec.ProviderID = &providerID
	return nil
}

// setHostSpec will ensure the host's Spec is set according to the machine's details.
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

// setReferencesOnHost will ensure the host is set to link to this HetznerBareMetalMachine.
func (s *Service) setReferencesOnHost(host *infrav1.HetznerBareMetalHost) {
	// set consumer ref if it is nil or pointing to another HetznerBareMetalMachine
	if host.Spec.ConsumerRef == nil || host.Spec.ConsumerRef.Name != s.scope.BareMetalMachine.Name {
		host.Spec.ConsumerRef = &corev1.ObjectReference{
			Kind:       "HetznerBareMetalMachine",
			Name:       s.scope.BareMetalMachine.Name,
			Namespace:  s.scope.BareMetalMachine.Namespace,
			APIVersion: s.scope.BareMetalMachine.APIVersion,
		}
	}
	// set owner ref
	host.OwnerReferences = s.setOwnerRef(host.OwnerReferences)
}

func (s *Service) updateMachineAddresses(host *infrav1.HetznerBareMetalHost) {
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

// setOwnerRef adds an owner reference of this HetznerBareMetalMachine.
func (s *Service) setOwnerRef(refList []metav1.OwnerReference) []metav1.OwnerReference {
	return setOwnerRefInList(refList, s.scope.BareMetalMachine.TypeMeta, s.scope.BareMetalMachine.ObjectMeta)
}

// setOwnerRefInList adds an owner reference of a Kubernetes object.
func setOwnerRefInList(refList []metav1.OwnerReference, objType metav1.TypeMeta, objMeta metav1.ObjectMeta) []metav1.OwnerReference {
	isController := true
	index, found := findOwnerRefFromList(refList, objType, objMeta)
	if !found {
		// set new owner ref
		refList = append(refList, metav1.OwnerReference{
			APIVersion: objType.APIVersion,
			Kind:       objType.Kind,
			Name:       objMeta.Name,
			UID:        objMeta.UID,
			Controller: pointer.Bool(isController),
		})
	} else {
		// update existing owner ref because the UID and the APIVersion might change due to move or version upgrade
		refList[index].APIVersion = objType.APIVersion
		refList[index].UID = objMeta.UID
		refList[index].Controller = pointer.Bool(isController)
	}
	return refList
}

// removeOwnerRef removes the owner reference of this BareMetalMachine.
func (s *Service) removeOwnerRef(refList []metav1.OwnerReference) []metav1.OwnerReference {
	return removeOwnerRefFromList(refList, s.scope.BareMetalMachine.TypeMeta, s.scope.BareMetalMachine.ObjectMeta)
}

// removeOwnerRefFromList removes the owner reference of a Kubernetes object.
func removeOwnerRefFromList(
	refList []metav1.OwnerReference,
	objType metav1.TypeMeta,
	objMeta metav1.ObjectMeta,
) []metav1.OwnerReference {
	if len(refList) == 0 {
		return refList
	}
	index, found := findOwnerRefFromList(refList, objType, objMeta)
	// if owner ref is not found, return
	if !found {
		return refList
	}

	// if it is the only owner ref, we can return an empty slice
	if len(refList) == 1 {
		return []metav1.OwnerReference{}
	}

	// remove owner ref from slice
	refListLen := len(refList) - 1
	refList[index] = refList[refListLen]
	refList = refList[:refListLen]

	return removeOwnerRefFromList(refList, objType, objMeta)
}

// findOwnerRefFromList finds the owner ref of a Kubernetes object in a list of owner refs.
func findOwnerRefFromList(refList []metav1.OwnerReference, objType metav1.TypeMeta, objMeta metav1.ObjectMeta) (ref int, found bool) {
	bGV, err := schema.ParseGroupVersion(objType.APIVersion)
	if err != nil {
		panic("object has invalid group version")
	}

	for i, curOwnerRef := range refList {
		aGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			// ignore owner ref if it has invalid group version
			continue
		}

		// not matching on UID since when pivoting it might change
		// Not matching on API version as this might change
		if curOwnerRef.Name == objMeta.Name &&
			curOwnerRef.Kind == objType.Kind &&
			aGV.Group == bGV.Group {
			return i, true
		}
	}
	return 0, false
}

// ensureMachineAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (s *Service) ensureMachineAnnotation(host *infrav1.HetznerBareMetalHost) {
	annotations := s.scope.BareMetalMachine.ObjectMeta.GetAnnotations()
	updatedAnnotations := updateHostAnnotation(annotations, hostKey(host), s.scope.Logger)
	s.scope.BareMetalMachine.ObjectMeta.SetAnnotations(updatedAnnotations)
}

func updateHostAnnotation(annotations map[string]string, hostKey string, log logr.Logger) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	existing, ok := annotations[infrav1.HostAnnotation]
	if ok {
		if existing == hostKey {
			return annotations
		}
		log.V(1).Info("Warning: found stray annotation for host on machine - overwriting", "current annotation", existing)
	}
	annotations[infrav1.HostAnnotation] = hostKey
	return annotations
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
	emptySSHStatus := infrav1.SSHStatus{}
	if host.Spec.Status.SSHStatus != emptySSHStatus {
		host.Spec.Status.SSHStatus = emptySSHStatus
		updatedHost = true
	}
	return updatedHost
}

func ensureClusterLabel(host *infrav1.HetznerBareMetalHost, clusterName string) {
	// set cluster label on host
	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}
	host.Labels[clusterv1.ClusterNameLabel] = clusterName
}

// nodeAddresses returns a slice of clusterv1.MachineAddress objects for a given host.
func nodeAddresses(host *infrav1.HetznerBareMetalHost, bareMetalMachineName string) []clusterv1.MachineAddress {
	// if there are no hw details, return
	if host.Spec.Status.HardwareDetails == nil {
		return nil
	}

	addrs := make([]clusterv1.MachineAddress, 0, len(host.Spec.Status.HardwareDetails.NIC)+2)

	for _, nic := range host.Spec.Status.HardwareDetails.NIC {
		address := clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalIP,
			Address: nic.IP,
		}
		addrs = append(addrs, address)
	}

	// Add hostname == bareMetalMachineName as well
	addrs = append(
		addrs,
		clusterv1.MachineAddress{
			Type:    clusterv1.MachineHostName,
			Address: bareMetalMachineName,
		},
		clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalDNS,
			Address: bareMetalMachineName,
		},
	)

	return addrs
}

// consumerRefMatches returns a boolean based on whether the consumer reference and bare metal machine metadata match.
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

func hostKey(host *infrav1.HetznerBareMetalHost) string {
	return host.GetNamespace() + "/" + host.GetName()
}

func splitHostKey(key string) (namespace, name string) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		panic("unexpected host key")
	}
	return parts[0], parts[1]
}

func checkForRequeueError(err error, errMessage string) (res reconcile.Result, reterr error) {
	if err == nil {
		return res, nil
	}
	var requeueError *scope.RequeueAfterError
	if ok := errors.As(err, &requeueError); ok {
		return reconcile.Result{Requeue: true, RequeueAfter: requeueError.GetRequeueAfter()}, nil
	}

	return res, fmt.Errorf("%s: %w", errMessage, err)
}

func providerIDFromServerID(serverID int) string {
	return fmt.Sprintf("%s%s%d", providerIDPrefix, infrav1.BareMetalHostNamePrefix, serverID)
}

func analyzePatchError(err error, ignoreNotFound bool) error {
	if apierrors.IsConflict(err) {
		return &scope.RequeueAfterError{}
	}
	if apierrors.IsNotFound(err) && ignoreNotFound {
		return nil
	}
	return err
}
