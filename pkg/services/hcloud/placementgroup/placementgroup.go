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

// Package placementgroup implements the lifecycle of HCloud placement groups.
package placementgroup

import (
	"context"
	"fmt"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Service struct contains cluster scope to reconcile placement groups.
type Service struct {
	scope *scope.ClusterScope
}

// NewService creates new service object.
func NewService(scope *scope.ClusterScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements life cycle of placement groups.
func (s *Service) Reconcile(ctx context.Context) (err error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("Reconcile placement groups")

	// find placement groups
	placementGroups, err := s.findPlacementGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to find placement group: %w", err)
	}

	s.scope.HetznerCluster.Status.HCloudPlacementGroup = apiToStatus(placementGroups, s.scope.HetznerCluster.Name)

	placementGroupsSpec := s.scope.HetznerCluster.Spec.HCloudPlacementGroup
	placementGroupsStatus := s.scope.HetznerCluster.Status.HCloudPlacementGroup

	// Create arrays and maps to make diff
	placementGroupNamesStatus := make([]string, len(placementGroupsStatus))
	placementGroupNamesSpec := make([]string, len(placementGroupsSpec))
	placementGroupStatusMap := make(map[string]infrav1.HCloudPlacementGroupStatus)
	placementGroupSpecMap := make(map[string]infrav1.HCloudPlacementGroupSpec)

	for i, pgSpec := range placementGroupsSpec {
		placementGroupNamesSpec[i] = pgSpec.Name
		placementGroupSpecMap[pgSpec.Name] = placementGroupsSpec[i]
	}

	for i, pgSts := range placementGroupsStatus {
		placementGroupNamesStatus[i] = pgSts.Name
		placementGroupStatusMap[pgSts.Name] = placementGroupsStatus[i]
	}

	// Make diff and create/delete placement groups
	toCreate, toDelete := utils.DifferenceOfStringSlices(placementGroupNamesSpec, placementGroupNamesStatus)

	var multierr []error
	// Create
	for _, pgName := range toCreate {
		name := fmt.Sprintf("%s-%s", s.scope.HetznerCluster.Name, placementGroupSpecMap[pgName].Name)
		clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
		if _, err := s.scope.HCloudClient.CreatePlacementGroup(ctx, hcloud.PlacementGroupCreateOpts{
			Name:   name,
			Type:   hcloud.PlacementGroupType(placementGroupSpecMap[pgName].Type),
			Labels: map[string]string{clusterTagKey: string(infrav1.ResourceLifecycleOwned)},
		}); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "CreatePlacementGroup")
			multierr = append(multierr, err)
		}
	}

	// Delete
	for _, pgName := range toDelete {
		if err := s.scope.HCloudClient.DeletePlacementGroup(ctx, placementGroupStatusMap[pgName].ID); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeletePlacementGroup")
			multierr = append(multierr, err)
		}
	}

	if err := kerrors.NewAggregate(multierr); err != nil {
		return fmt.Errorf("aggregate error - creating/deleting placement groups: %w", err)
	}

	// Update status
	placementGroups, err = s.findPlacementGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to find placement group: %w", err)
	}

	s.scope.HetznerCluster.Status.HCloudPlacementGroup = apiToStatus(placementGroups, s.scope.HetznerCluster.Name)
	return nil
}

// Delete implements deletion of placement groups.
func (s *Service) Delete(ctx context.Context) (err error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("Delete placement groups")

	// Delete placement groups which are not in status but in specs
	var multierr []error
	for _, pg := range s.scope.HetznerCluster.Status.HCloudPlacementGroup {
		if err := s.scope.HCloudClient.DeletePlacementGroup(ctx, pg.ID); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeletePlacementGroup")
			if !hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
				multierr = append(multierr, err)
			}
		}
	}

	if err := kerrors.NewAggregate(multierr); err != nil {
		log.Error(err, "aggregate error - deleting placement groups")
		return err
	}

	record.Eventf(s.scope.HetznerCluster, "PlacementGroupsDeleted", "Deleted placement groups")

	return nil
}

func (s *Service) findPlacementGroups(ctx context.Context) ([]*hcloud.PlacementGroup, error) {
	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
	labels := map[string]string{clusterTagKey: string(infrav1.ResourceLifecycleOwned)}
	opts := hcloud.PlacementGroupListOpts{}
	opts.LabelSelector = utils.LabelsToLabelSelector(labels)

	placementGroups, err := s.scope.HCloudClient.ListPlacementGroups(ctx, opts)
	if err != nil {
		hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "ListPlacementGroups")
		return nil, fmt.Errorf("failed to list placement groups: %w", err)
	}
	return placementGroups, nil
}

// gets the information of the Hetzner load balancer object and returns it in our status object.
func apiToStatus(placementGroups []*hcloud.PlacementGroup, clusterName string) []infrav1.HCloudPlacementGroupStatus {
	status := make([]infrav1.HCloudPlacementGroupStatus, len(placementGroups))
	for i, pg := range placementGroups {
		status[i] = infrav1.HCloudPlacementGroupStatus{
			ID:     pg.ID,
			Server: pg.Servers,
			Name:   strings.TrimPrefix(pg.Name, fmt.Sprintf("%s-", clusterName)),
			Type:   string(pg.Type),
		}
	}
	return status
}
