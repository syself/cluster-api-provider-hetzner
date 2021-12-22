package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
)

const (
	hoursBeforeDeletion      time.Duration = 36
	rateLimitTimeOut         time.Duration = 660
	rateLimitTimeOutDeletion time.Duration = 120
)

// Service defines struct with machine scope to reconcile Hcloud machines.
type Service struct {
	scope *scope.BareMetalMachineScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.BareMetalMachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of Hcloud machines.
func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {

	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal machine", "name", s.scope.BareMetalMachine.Name)

	// If no token information has been given, the server cannot be successfully reconciled
	if s.scope.HetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser == "" {
		record.Eventf(s.scope.BareMetalMachine, corev1.EventTypeWarning, "NoTokenFound", "No HRobot token found")
		return nil, fmt.Errorf("no token for Hetzner Robot provided - cannot reconcile bare metal server")
	}

	// Try to find an existing instance
	instance, err := s.scope.HetznerRobotClient().GetBMServer(s.scope.ID())
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			record.Eventf(s.scope.BareMetalMachine,
				corev1.EventTypeWarning,
				"HRobotRateLimitExceeded",
				"HRobot rate limit exceeded. Wait for %v sec before trying again.",
				rateLimitTimeOut)
			return &reconcile.Result{RequeueAfter: rateLimitTimeOut * time.Second}, nil
		}
		return nil, errors.Wrap(err, "failed to get instance")
	}

	if instance == nil {
		record.Eventf(
			s.scope.BareMetalMachine,
			"BareMetalMachineNotFound",
			"No matching bare metal machine found with ID %s",
			s.scope.ID(),
		)
		return nil, fmt.Errorf("no matching bare metal machine found with ID %s", s.scope.ID())
	}

	sts := setStatusFromAPI(instance)

	s.scope.BareMetalMachine.Status = sts
	s.scope.HetznerCluster.Status.BareMetalInventory[s.scope.ID()] = sts
	return nil, nil
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

func setStatusFromAPI(instance *models.Server) infrav1.BareMetalMachineHostStatus {
	return infrav1.BareMetalMachineHostStatus{
		HetznerStatus: instance.Status,
		ID:            instance.ServerNumber,
		Name:          instance.ServerName,
		DataCenter:    instance.Dc,
		PaidUntil:     instance.PaidUntil,
	}
}
