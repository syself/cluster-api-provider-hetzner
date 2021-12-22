// Package baremetal implements functions to manage the lifecycle of baremetal machines as inventory
package baremetal

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
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
		return s.checkMachineError(s.update(ctx, log), "Failed to update the Metal3Machine", errType)
	}

	// Make sure bootstrap data is available and populated. If not, return, we
	// will get an event from the machine update when the flag is set to true.
	if !s.scope.IsBootstrapReady(ctx) {
		return &ctrl.Result{}, nil
	}

	errType := capierrors.CreateMachineError

	// Check if the metal3machine was associated with a baremetalhost
	if !s.scope.HasAnnotation() {
		//Associate the baremetalhost hosting the machine
		err := s.Associate(ctx, log)
		if err != nil {
			return s.checkMachineError(err, "failed to associate the Metal3Machine to a BaremetalHost", errType)
		}
	}

	err = s.update(ctx, log)
	if err != nil {
		return s.checkMachineError(err, "failed to update BaremetalHost", errType)
	}

	// TODO: Do we need this?
	// providerID, bmhID := machineMgr.GetProviderIDAndBMHID()
	// if bmhID == nil {
	// 	bmhID, err = machineMgr.GetBaremetalHostID(ctx)
	// 	if err != nil {
	// 		return checkMachineError(machineMgr, err,
	// 			"failed to get the providerID for the metal3machine", errType,
	// 		)
	// 	}
	// 	if bmhID != nil {
	// 		providerID = fmt.Sprintf("%s%s", baremetal.ProviderIDPrefix, *bmhID)
	// 	}
	// }
	// if bmhID != nil {
	// 	// Set the providerID on the node if no Cloud provider
	// 	err = machineMgr.SetNodeProviderID(ctx, *bmhID, providerID, r.CapiClientGetter)
	// 	if err != nil {
	// 		return checkMachineError(machineMgr, err,
	// 			"failed to set the target node providerID", errType,
	// 		)
	// 	}
	// 	// Make sure Spec.ProviderID is set and mark the capm3Machine ready
	// 	machineMgr.SetProviderID(providerID)
	// }

	return &ctrl.Result{}, err
}

// Delete implements delete method of bare metal machine.
func (s *Service) Delete(ctx context.Context) (_ *ctrl.Result, err error) {

	record.Eventf(
		s.scope.BareMetalMachine,
		"BareMetalMachineDeleted",
		"Bare metal inventory machine with ID %s deleted",
		s.scope.ID(),
	)
	return nil, nil
}

// update updates a machine and is invoked by the Machine Controller
func (s *Service) update(ctx context.Context, log logr.Logger) error {
	log.V(1).Info("Updating machine")

	// clear any error message that was previously set. This method doesn't set
	// error messages yet, so we know that it's incorrect to have one here.
	s.scope.ClearError()

	host, helper, err := s.getHost(ctx, log)
	if err != nil {
		return err
	}
	if host == nil {
		return errors.Errorf("host not found for machine %s", s.scope.Machine.Name)
	}

	// ensure that the host's consumer ref is correctly set
	// TODO: Do we need this?
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
	// TODO: Do we have to implement this?
	// err = s.setHostSpec(ctx, host)
	// if err != nil {
	// 	if _, ok := err.(HasRequeueAfterError); !ok {
	// 		s.scope.SetError("Failed to associate the BaremetalHost to the Hetzner bare metalMachine",
	// 			capierrors.CreateMachineError,
	// 		)
	// 	}
	// 	return err
	// }

	err = helper.Patch(ctx, host)
	if err != nil {
		return err
	}

	err = s.ensureAnnotation(ctx, host, log)
	if err != nil {
		return err
	}

	// TODO: Do we need this?
	// if err := m.updateMachineStatus(ctx, host); err != nil {
	// 	return err
	// }

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
	host, helper, err := s.getHost(ctx, log)
	if err != nil {
		s.scope.SetError("Failed to get the BaremetalHost for the Metal3Machine",
			capierrors.CreateMachineError,
		)
		return err
	}

	// no BMH found, trying to choose from available ones
	if host == nil {
		host, helper, err = s.chooseHost(ctx, log)
		if err != nil {
			if _, ok := err.(HasRequeueAfterError); !ok {
				s.scope.SetError("Failed to pick a BaremetalHost for the Metal3Machine",
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

	// A machine bootstrap not ready case is caught in the controller
	// ReconcileNormal function
	// TODO: Re-design this function according to our needs
	// err = s.getUserDataSecretName(ctx, host)
	// if err != nil {
	// 	if _, ok := err.(HasRequeueAfterError); !ok {
	// 		s.scope.SetError("Failed to set the UserData for the Metal3Machine",
	// 			capierrors.CreateMachineError,
	// 		)
	// 	}
	// 	return err
	// }

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
			s.scope.SetError("Failed to associate the BaremetalHost to the Metal3Machine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// ensure that the host's specs are correctly set
	// TODO: Do we have to implement this?
	// err = s.setHostSpec(ctx, host)
	// if err != nil {
	// 	if _, ok := err.(HasRequeueAfterError); !ok {
	// 		s.scope.SetError("Failed to associate the BaremetalHost to the Hetzner bare metalMachine",
	// 			capierrors.CreateMachineError,
	// 		)
	// 	}
	// 	return err
	// }

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
			s.scope.SetError("Failed to annotate the Metal3Machine",
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
func (s *Service) getHost(ctx context.Context, log logr.Logger) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	host, err := getHost(ctx, s.scope.BareMetalMachine, s.scope.Client, s.scope.Logger)
	if err != nil || host == nil {
		return host, nil, err
	}
	helper, err := patch.NewHelper(host, s.scope.Client)
	return host, helper, err
}

func getHost(ctx context.Context, bmMachine *infrav1.HetznerBareMetalMachine, cl client.Client,
	mLog logr.Logger,
) (*infrav1.HetznerBareMetalHost, error) {
	annotations := bmMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return nil, nil
	}
	hostKey, ok := annotations[scope.HostAnnotation]
	if !ok {
		return nil, nil
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		mLog.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return nil, err
	}

	host := infrav1.HetznerBareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = cl.Get(ctx, key, &host)
	if apierrors.IsNotFound(err) {
		mLog.Info("Annotated host not found", "host", hostKey)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *Service) chooseHost(ctx context.Context, log logr.Logger) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	// TODO: Implement this function
	// // get list of BMH
	// hosts := infrav1.HetznerBareMetalHostList{}
	// // without this ListOption, all namespaces would be including in the listing
	// opts := &client.ListOptions{
	// 	Namespace: s.scope.BareMetalMachine.Namespace,
	// }

	// err := s.scope.Client.List(ctx, &hosts, opts)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// // Using the label selector on ListOptions above doesn't seem to work.
	// // I think it's because we have a local cache of all BareMetalHosts.
	// labelSelector := labels.NewSelector()
	// var reqs labels.Requirements

	// for labelKey, labelVal := range s.scope.BareMetalMachine.Spec.HostSelector.MatchLabels {
	// 	log.Info("Adding requirement to match label",
	// 		"label key", labelKey,
	// 		"label value", labelVal)
	// 	r, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal})
	// 	if err != nil {
	// 		log.Error(err, "Failed to create MatchLabel requirement, not choosing host")
	// 		return nil, nil, err
	// 	}
	// 	reqs = append(reqs, *r)
	// }
	// for _, req := range s.scope.BareMetalMachine.Spec.HostSelector.MatchExpressions {
	// 	log.Info("Adding requirement to match label",
	// 		"label key", req.Key,
	// 		"label operator", req.Operator,
	// 		"label value", req.Values)
	// 	lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
	// 	r, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values)
	// 	if err != nil {
	// 		log.Error(err, "Failed to create MatchExpression requirement, not choosing host")
	// 		return nil, nil, err
	// 	}
	// 	reqs = append(reqs, *r)
	// }
	// labelSelector = labelSelector.Add(reqs...)

	// availableHosts := []*infrav1.HetznerBareMetalHost{}
	// availableHostsWithNodeReuse := []*infrav1.HetznerBareMetalHost{}

	// for i, host := range hosts.Items {
	// 	if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, s.scope.BareMetalMachine) {
	// 		log.Info("Found host with existing ConsumerRef", "host", host.Name)
	// 		helper, err := patch.NewHelper(&hosts.Items[i], s.scope.Client)
	// 		return &hosts.Items[i], helper, err
	// 	}
	// 	if host.Spec.ConsumerRef != nil ||
	// 		(s.nodeReuseLabelExists(ctx, &host) &&
	// 			!s.nodeReuseLabelMatches(ctx, &host)) {
	// 		continue
	// 	}
	// 	if host.GetDeletionTimestamp() != nil {
	// 		continue
	// 	}
	// 	if host.Status.ErrorMessage != "" {
	// 		continue
	// 	}

	// 	// continue if BaremetalHost is paused or marked with UnhealthyAnnotation
	// 	annotations := host.GetAnnotations()
	// 	if annotations != nil {
	// 		if _, ok := annotations[infrav1.PausedAnnotation]; ok {
	// 			continue
	// 		}
	// 		if _, ok := annotations[capm3.UnhealthyAnnotation]; ok {
	// 			continue
	// 		}
	// 	}

	// 	if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
	// 		if m.nodeReuseLabelExists(ctx, &host) && m.nodeReuseLabelMatches(ctx, &host) {
	// 			log.Info(fmt.Sprintf("Found host %v with nodeReuseLabelName and it matches, adding it to availableHostsWithNodeReuse list", host.Name))
	// 			availableHostsWithNodeReuse = append(availableHostsWithNodeReuse, &hosts.Items[i])
	// 		} else if !m.nodeReuseLabelExists(ctx, &host) {
	// 			switch host.Status.Provisioning.State {
	// 			case bmh.StateReady, bmh.StateAvailable:
	// 			default:
	// 				continue
	// 			}
	// 			log.Info(fmt.Sprintf("Host %v matched hostSelector for Metal3Machine, adding it to availableHosts list", host.Name))
	// 			availableHosts = append(availableHosts, &hosts.Items[i])
	// 		}
	// 	} else {
	// 		log.Info(fmt.Sprintf("Host %v did not match hostSelector for Metal3Machine", host.Name))
	// 	}
	// }

	// log.Info(fmt.Sprintf("%d hosts available with nodeReuseLabelName while choosing host for Metal3 machine", len(availableHostsWithNodeReuse)))
	// log.Info(fmt.Sprintf("%d hosts available while choosing host for Metal3 machine", len(availableHosts)))
	// if len(availableHostsWithNodeReuse) == 0 && len(availableHosts) == 0 {
	// 	return nil, nil, nil
	// }

	// // choose a host
	// rand.Seed(time.Now().Unix())
	// var chosenHost *infrav1.HetznerBareMetalHost

	// // If there are hosts with nodeReuseLabelName:
	// if len(availableHostsWithNodeReuse) != 0 {
	// 	for _, host := range availableHostsWithNodeReuse {
	// 		// Build list of hosts in Ready state with nodeReuseLabelName
	// 		hostsInAvailableStateWithNodeReuse := []*infrav1.HetznerBareMetalHost{}
	// 		// Build list of hosts in any other state than Ready state with nodeReuseLabelName
	// 		hostsInNotAvailableStateWithNodeReuse := []*infrav1.HetznerBareMetalHost{}
	// 		if host.Status.Provisioning.State == bmh.StateReady || host.Status.Provisioning.State == bmh.StateAvailable {
	// 			hostsInAvailableStateWithNodeReuse = append(hostsInAvailableStateWithNodeReuse, host)
	// 		} else {
	// 			hostsInNotAvailableStateWithNodeReuse = append(hostsInNotAvailableStateWithNodeReuse, host)
	// 		}

	// 		// If host is found in `Ready` state, pick it
	// 		if len(hostsInAvailableStateWithNodeReuse) != 0 {
	// 			log.Info(fmt.Sprintf("Found %v host(s) with nodeReuseLabelName in Ready/Available state, choosing the host %v", len(hostsInAvailableStateWithNodeReuse), host.Name))
	// 			chosenHost = hostsInAvailableStateWithNodeReuse[rand.Intn(len(hostsInAvailableStateWithNodeReuse))]
	// 		} else if len(hostsInNotAvailableStateWithNodeReuse) != 0 {
	// 			log.Info(fmt.Sprintf("Found %v host(s) with nodeReuseLabelName in %v state, requeuing the host %v", len(hostsInNotAvailableStateWithNodeReuse), host.Status.Provisioning.State, host.Name))
	// 			return nil, nil, &RequeueAfterError{RequeueAfter: requeueAfter}
	// 		}
	// 	}
	// } else {
	// 	// If there are no hosts with nodeReuseLabelName, fall back
	// 	// to the current flow and select hosts randomly.
	// 	log.Info(fmt.Sprintf("%d host(s) available, choosing a random host", len(availableHosts)))
	// 	chosenHost = availableHosts[rand.Intn(len(availableHosts))]
	// }

	// helper, err := patch.NewHelper(chosenHost, s.scope.Client)
	// return chosenHost, helper, err
	return nil, nil, nil
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
