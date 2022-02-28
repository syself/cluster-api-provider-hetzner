// Package baremetal implements functions to manage the lifecycle of baremetal machines as inventory
package baremetal

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/go-logr/logr"
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
	hoursBeforeDeletion      time.Duration = 36
	rateLimitTimeOut         time.Duration = 660
	rateLimitTimeOutDeletion time.Duration = 120
)

const (
	// ProviderIDPrefix is a prefix for ProviderID.
	ProviderIDPrefix = "hetznerrobot://"
	// nodeReuseLabelName is the label set on BMH when node reuse feature is enabled.
	nodeReuseLabelName = "infrastructure.cluster.x-k8s.io/node-reuse"
	requeueAfter       = time.Second * 30
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
		//Associate the baremetalhost hosting the machine
		err := s.Associate(ctx, log)
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
			providerID = fmt.Sprintf("%s%s", ProviderIDPrefix, *bmhID)
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

	record.Eventf(
		s.scope.BareMetalMachine,
		"BareMetalMachineDeleted",
		"Bare metal machine with name %s deleted",
		s.scope.Name(),
	)
	return nil, nil
}

// update updates a machine and is invoked by the Machine Controller
func (s *Service) update(ctx context.Context, log logr.Logger) error {
	log.V(1).Info("Updating machine")

	// clear any error message that was previously set. This method doesn't set
	// error messages yet, so we know that it's incorrect to have one here.
	s.scope.ClearError()

	host, helper, err := s.getHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		return errors.Errorf("host not found for machine %s", s.scope.Machine.Name)
	}

	// ensure that the host's consumer ref is correctly set
	err = s.setHostConsumerRef(ctx, host, log)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to associate the BaremetalHost to the Hetzner bare metalMachine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// ensure that the host's specs are correctly set
	err = s.setHostSpec(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to associate the BaremetalHost to the Hetzner bare metalMachine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	err = helper.Patch(ctx, host)
	if err != nil {
		return err
	}

	err = s.ensureAnnotation(ctx, host, log)
	if err != nil {
		return err
	}

	if err := s.updateMachineStatus(ctx, host); err != nil {
		return err
	}

	log.Info("Finished updating machine")
	return nil
}

func (s *Service) Associate(ctx context.Context, log logr.Logger) error {
	log.Info("Associating machine", "machine", s.scope.Machine.Name)

	// load and validate the config
	if s.scope.BareMetalMachine == nil {
		// Should have been picked earlier. Do not requeue
		return nil
	}

	// clear an error if one was previously set
	s.scope.ClearError()

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
		host, helper, err = s.chooseHost(ctx, log)
		if err != nil {
			if _, ok := err.(HasRequeueAfterError); !ok {
				s.scope.SetError("Failed to pick a BaremetalHost for the HetznerBareMetalMachine",
					capierrors.CreateMachineError,
				)
			}
			return err
		}
		if host == nil {
			log.Info("No available host found. Requeuing.")
			return &RequeueAfterError{RequeueAfter: requeueAfter}
		}
		log.Info("Associating machine with host", "host", host.Name)
	} else {
		log.Info("Machine already associated with host", "host", host.Name)
	}

	err = s.setHostLabel(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to set the Cluster label in the BareMetalHost",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	err = s.setHostConsumerRef(ctx, host, log)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to associate the BaremetalHost to the HetznerBareMetalMachine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// ensure that the host's specs are correctly set
	err = s.setHostSpec(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			s.scope.SetError("Failed to associate the BaremetalHost to the Hetzner bare metalMachine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	err = helper.Patch(ctx, host)
	if err != nil {
		if aggr, ok := err.(kerrors.Aggregate); ok {
			for _, kerr := range aggr.Errors() {
				if apierrors.IsConflict(kerr) {
					return &RequeueAfterError{}
				}
			}
		}
		return err
	}

	err = s.ensureAnnotation(ctx, host, log)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
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
	hostKey, ok := annotations[scope.HostAnnotation]
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

func (s *Service) chooseHost(ctx context.Context, log logr.Logger) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	// get list of BMH
	hosts := infrav1.HetznerBareMetalHostList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: s.scope.BareMetalMachine.Namespace,
	}

	err := s.scope.Client.List(ctx, &hosts, opts)
	if err != nil {
		return nil, nil, err
	}

	// Using the label selector on ListOptions above doesn't seem to work.
	// I think it's because we have a local cache of all BareMetalHosts.
	labelSelector := labels.NewSelector()
	var reqs labels.Requirements

	for labelKey, labelVal := range s.scope.BareMetalMachine.Spec.HostSelector.MatchLabels {
		log.Info("Adding requirement to match label",
			"label key", labelKey,
			"label value", labelVal)
		r, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal})
		if err != nil {
			log.Error(err, "Failed to create MatchLabel requirement, not choosing host")
			return nil, nil, err
		}
		reqs = append(reqs, *r)
	}
	for _, req := range s.scope.BareMetalMachine.Spec.HostSelector.MatchExpressions {
		log.Info("Adding requirement to match label",
			"label key", req.Key,
			"label operator", req.Operator,
			"label value", req.Values)
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		r, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values)
		if err != nil {
			log.Error(err, "Failed to create MatchExpression requirement, not choosing host")
			return nil, nil, err
		}
		reqs = append(reqs, *r)
	}
	labelSelector = labelSelector.Add(reqs...)

	availableHosts := []*infrav1.HetznerBareMetalHost{}
	availableHostsWithNodeReuse := []*infrav1.HetznerBareMetalHost{}

	for i, host := range hosts.Items {
		if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, s.scope.BareMetalMachine) {
			log.Info("Found host with existing ConsumerRef", "host", host.Name)
			helper, err := patch.NewHelper(&hosts.Items[i], s.scope.Client)
			return &hosts.Items[i], helper, err
		}
		if host.Spec.ConsumerRef != nil {
			continue
		}
		if host.GetDeletionTimestamp() != nil {
			continue
		}
		if host.Spec.Status.ErrorMessage != "" {
			continue
		}

		if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
			switch host.Spec.Status.ProvisioningState {
			case infrav1.StateAvailable:
			default:
				continue
			}
			log.Info(fmt.Sprintf("Host %v matched hostSelector for HetznerBareMetalMachine, adding it to availableHosts list", host.Name))
			availableHosts = append(availableHosts, &hosts.Items[i])
		} else {
			log.Info(fmt.Sprintf("Host %v did not match hostSelector for HetznerBareMetalMachine", host.Name))
		}
	}

	log.Info(fmt.Sprintf("%d hosts available with nodeReuseLabelName while choosing host for HetznerBareMetal machine", len(availableHostsWithNodeReuse)))
	log.Info(fmt.Sprintf("%d hosts available while choosing host for HetznerBareMetal machine", len(availableHosts)))
	if len(availableHostsWithNodeReuse) == 0 && len(availableHosts) == 0 {
		return nil, nil, nil
	}

	// choose a host
	rand.Seed(time.Now().Unix())
	var chosenHost *infrav1.HetznerBareMetalHost

	// If there are hosts with nodeReuseLabelName:
	if len(availableHostsWithNodeReuse) != 0 {
		for _, host := range availableHostsWithNodeReuse {
			// Build list of hosts in Ready state with nodeReuseLabelName
			hostsInAvailableStateWithNodeReuse := []*infrav1.HetznerBareMetalHost{}
			// Build list of hosts in any other state than Ready state with nodeReuseLabelName
			hostsInNotAvailableStateWithNodeReuse := []*infrav1.HetznerBareMetalHost{}
			if host.Spec.Status.ProvisioningState == infrav1.StateAvailable {
				hostsInAvailableStateWithNodeReuse = append(hostsInAvailableStateWithNodeReuse, host)
			} else {
				hostsInNotAvailableStateWithNodeReuse = append(hostsInNotAvailableStateWithNodeReuse, host)
			}

			// If host is found in `Ready` state, pick it
			if len(hostsInAvailableStateWithNodeReuse) != 0 {
				log.Info(fmt.Sprintf("Found %v host(s) with nodeReuseLabelName in Ready/Available state, choosing the host %v", len(hostsInAvailableStateWithNodeReuse), host.Name))
				chosenHost = hostsInAvailableStateWithNodeReuse[rand.Intn(len(hostsInAvailableStateWithNodeReuse))]
			} else if len(hostsInNotAvailableStateWithNodeReuse) != 0 {
				log.Info(fmt.Sprintf("Found %v host(s) with nodeReuseLabelName in %v state, requeuing the host %v", len(hostsInNotAvailableStateWithNodeReuse), host.Spec.Status.ProvisioningState, host.Name))
				return nil, nil, &RequeueAfterError{RequeueAfter: requeueAfter}
			}
		}
	} else {
		// If there are no hosts with nodeReuseLabelName, fall back
		// to the current flow and select hosts randomly.
		log.Info(fmt.Sprintf("%d host(s) available, choosing a random host", len(availableHosts)))
		chosenHost = availableHosts[rand.Intn(len(availableHosts))]
	}

	helper, err := patch.NewHelper(chosenHost, s.scope.Client)
	return chosenHost, helper, err
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
		return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	if host.Spec.Status.ProvisioningState == infrav1.StateProvisioned {
		return pointer.StringPtr(string(host.ObjectMeta.UID)), nil
	}
	s.scope.Logger.Info("Provisioning BaremetalHost, requeuing")
	// Do not requeue since BMH update will trigger a reconciliation
	return nil, nil
}

// setHostSpec will ensure the host's Spec is set according to the machine's
// details. It will then update the host via the kube API. If UserData does not
// include a Namespace, it will default to the HetznerBareMetalMachine's namespace.
func (s *Service) setHostSpec(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	// We only want to update the image setting if the host does not
	// already have an image.
	//
	// A host with an existing image is already provisioned and
	// upgrades are not supported at this time. To re-provision a
	// host, we must fully deprovision it and then provision it again.
	// Not provisioning while we do not have the UserData.

	if host.Spec.Image == "" && s.scope.Machine.Spec.Bootstrap.DataSecretName != nil {
		host.Spec.Image = s.scope.BareMetalMachine.Spec.Image
		host.Spec.Status.UserData = &corev1.SecretReference{Namespace: s.scope.Namespace(), Name: *s.scope.Machine.Spec.Bootstrap.DataSecretName}
	}

	host.Spec.Online = true
	return nil
}

// setHostConsumerRef will ensure the host's Spec is set to link to this
// Hetzner bare metalMachine
func (s *Service) setHostConsumerRef(ctx context.Context, host *infrav1.HetznerBareMetalHost, log logr.Logger) error {

	host.Spec.ConsumerRef = &corev1.ObjectReference{
		Kind:       "HetznerBareMetalMachine",
		Name:       s.scope.BareMetalMachine.Name,
		Namespace:  s.scope.BareMetalMachine.Namespace,
		APIVersion: s.scope.BareMetalMachine.APIVersion,
	}

	// Set OwnerReferences
	hostOwnerReferences, err := s.SetOwnerRef(host.OwnerReferences, true)
	if err != nil {
		return err
	}
	host.OwnerReferences = hostOwnerReferences

	// Delete nodeReuseLabelName from host
	log.Info("Deleting nodeReuseLabelName from host, if any")

	labels := host.GetLabels()
	if labels != nil {
		if _, ok := labels[nodeReuseLabelName]; ok {
			delete(host.Labels, nodeReuseLabelName)
			log.Info("Finished deleting nodeReuseLabelName")
		}
	}

	return nil
}

// updateMachineStatus updates a HetznerBareMetalMachine object's status.
func (s *Service) updateMachineStatus(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	addrs := s.nodeAddresses(host)

	bareMetalMachineOld := s.scope.BareMetalMachine.DeepCopy()

	s.scope.BareMetalMachine.Status.Addresses = addrs
	conditions.MarkTrue(s.scope.BareMetalMachine, infrav1.AssociateBMHCondition)

	if equality.Semantic.DeepEqual(s.scope.BareMetalMachine.Status, bareMetalMachineOld.Status) {
		// Status did not change
		return nil
	}

	now := metav1.Now()
	s.scope.BareMetalMachine.Status.LastUpdated = &now
	return nil
}

// NodeAddresses returns a slice of corev1.NodeAddress objects for a
// given HetznerBareMetal machine.
func (s *Service) nodeAddresses(host *infrav1.HetznerBareMetalHost) []capi.MachineAddress {
	addrs := []capi.MachineAddress{}

	// If the host is nil or we have no hw details, return an empty address array.
	if host == nil || host.Spec.Status.HardwareDetails == nil {
		return addrs
	}

	for _, nic := range host.Spec.Status.HardwareDetails.NIC {
		address := capi.MachineAddress{
			Type:    capi.MachineInternalIP,
			Address: nic.IP,
		}
		addrs = append(addrs, address)
	}

	// Add hostname == bareMetalMachineName as well
	addrs = append(addrs, capi.MachineAddress{
		Type:    capi.MachineHostName,
		Address: s.scope.Name(),
	})
	addrs = append(addrs, capi.MachineAddress{
		Type:    capi.MachineInternalDNS,
		Address: s.scope.Name(),
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

// SetOwnerRef adds an ownerreference to this Hetzner bare metal machine
func (s *Service) SetOwnerRef(refList []metav1.OwnerReference, controller bool) ([]metav1.OwnerReference, error) {
	return setOwnerRefInList(refList, controller, s.scope.BareMetalMachine.TypeMeta,
		s.scope.BareMetalMachine.ObjectMeta,
	)
}

// SetOwnerRef adds an ownerreference to this Hetzner bare metal machine
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
		//The UID and the APIVersion might change due to move or version upgrade
		refList[index].APIVersion = objType.APIVersion
		refList[index].UID = objMeta.UID
		refList[index].Controller = pointer.BoolPtr(controller)
	}
	return refList, nil
}

// findOwnerRefFromList finds OwnerRef to this Hetzner bare metal machine
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
func (s *Service) ensureAnnotation(ctx context.Context, host *infrav1.HetznerBareMetalHost, log logr.Logger) error {
	annotations := s.scope.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	hostKey, err := cache.MetaNamespaceKeyFunc(host)
	if err != nil {
		log.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return err
	}
	existing, ok := annotations[scope.HostAnnotation]
	if ok {
		if existing == hostKey {
			return nil
		}
		log.Info("Warning: found stray annotation for host on machine. Overwriting.", "host", existing)
	}
	annotations[scope.HostAnnotation] = hostKey
	s.scope.BareMetalMachine.ObjectMeta.SetAnnotations(annotations)

	return nil
}

// setHostLabel will set the set cluster.x-k8s.io/cluster-name to bmh
func (s *Service) setHostLabel(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {

	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}
	host.Labels[capi.ClusterLabelName] = s.scope.Machine.Spec.ClusterName

	return nil
}

func (s *Service) checkMachineError(err error, errMessage string, errType capierrors.MachineStatusError) (*ctrl.Result, error) {
	if err == nil {
		return &ctrl.Result{}, nil
	}
	if requeueErr, ok := errors.Cause(err).(HasRequeueAfterError); ok {
		return &ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
	}
	s.scope.SetError(errMessage, errType)
	return &ctrl.Result{}, errors.Wrap(err, errMessage)
}

// NotFoundError represents that an object was not found
type NotFoundError struct {
}

// Error implements the error interface
func (e *NotFoundError) Error() string {
	return "Object not found"
}

func parseProviderID(providerID string) string {
	return strings.TrimPrefix(providerID, ProviderIDPrefix)
}
