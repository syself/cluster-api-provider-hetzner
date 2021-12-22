// Package remediation implements functions to manage the lifecycle of baremetal remediation.
package remediation

import (
	"context"

	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Service defines struct with machine scope to reconcile Hetzner bare metal remediation.
type Service struct {
	scope *scope.BareMetalRemediationScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.BareMetalRemediationScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of Hetzner bare metal remediation.
func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {

	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal remediation", "name", s.scope.BareMetalRemediation.Name)

	return &ctrl.Result{}, err
}

// Delete implements delete method of bare metal machine.
func (s *Service) Delete(ctx context.Context) (_ *ctrl.Result, err error) {

	record.Eventf(
		s.scope.BareMetalRemediation,
		"BareMetalRemediationDeleted",
		"Bare metal remediation with ID %s deleted",
		s.scope.ID(),
	)
	return nil, nil
}
