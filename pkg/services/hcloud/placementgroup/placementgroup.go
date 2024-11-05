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
	"errors"
	"fmt"
	"strings"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
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
	defer func() {
		if err != nil {
			conditions.MarkFalse(
				s.scope.HetznerCluster,
				infrav1.PlacementGroupsSyncedCondition,
				infrav1.PlacementGroupsSyncFailedReason,
				clusterv1.ConditionSeverityWarning,
				"%s",
				err.Error(),
			)
		}
	}()

	// find placement groups
	placementGroups, err := s.findPlacementGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to find placement groups: %w", err)
	}

	placementGroupsSpec := s.scope.HetznerCluster.Spec.HCloudPlacementGroups
	placementGroupsStatus := statusFromHCloudPlacementGroups(placementGroups, s.scope.HetznerCluster.Name)

	// Create arrays and maps to make diff
	placementGroupNamesExisting := make([]string, len(placementGroupsStatus))
	placementGroupNamesDesired := make([]string, len(placementGroupsSpec))
	placementGroupExistingMap := make(map[string]infrav1.HCloudPlacementGroupStatus)
	placementGroupDesiredMap := make(map[string]infrav1.HCloudPlacementGroupSpec)

	for i, pgSpec := range placementGroupsSpec {
		placementGroupNamesDesired[i] = pgSpec.Name
		placementGroupDesiredMap[pgSpec.Name] = placementGroupsSpec[i]
	}

	for i, pgSts := range placementGroupsStatus {
		placementGroupNamesExisting[i] = pgSts.Name
		placementGroupExistingMap[pgSts.Name] = placementGroupsStatus[i]
	}

	// make diff of existing and desired placement groups
	toCreate, toDelete := utils.DifferenceOfStringSlices(placementGroupNamesDesired, placementGroupNamesExisting)

	var multierr error

	// create new placement groups
	for _, pgName := range toCreate {
		name := fmt.Sprintf("%s-%s", s.scope.HetznerCluster.Name, pgName)
		clusterTagKey := s.scope.HetznerCluster.ClusterTagKey()
		opts := hcloud.PlacementGroupCreateOpts{
			Name:   name,
			Type:   hcloud.PlacementGroupType(placementGroupDesiredMap[pgName].Type),
			Labels: map[string]string{clusterTagKey: string(infrav1.ResourceLifecycleOwned)},
		}

		if _, err := s.scope.HCloudClient.CreatePlacementGroup(ctx, opts); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "CreatePlacementGroup")
			multierr = errors.Join(multierr, fmt.Errorf("failed to create placement group %q: %w", pgName, err))
		}
	}

	// delete old placement groups
	for _, pgName := range toDelete {
		id := placementGroupExistingMap[pgName].ID
		if err := s.scope.HCloudClient.DeletePlacementGroup(ctx, id); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeletePlacementGroup")
			multierr = errors.Join(multierr, fmt.Errorf("failed to delete placement group %v: %w", id, err))
		}
	}

	if multierr != nil {
		return fmt.Errorf("aggregate error - creating/deleting placement groups: %w", multierr)
	}

	// Update status
	if len(toCreate) > 0 || len(toDelete) > 0 {
		// No need to update status if nothing changed
		placementGroups, err = s.findPlacementGroups(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to find placement groups: %w", err)
	}

	s.scope.HetznerCluster.Status.HCloudPlacementGroups = statusFromHCloudPlacementGroups(placementGroups, s.scope.HetznerCluster.Name)
	conditions.MarkTrue(s.scope.HetznerCluster, infrav1.PlacementGroupsSyncedCondition)

	return nil
}

// Delete implements deletion of placement groups.
func (s *Service) Delete(ctx context.Context) (err error) {
	// Delete placement groups which are not in status but in specs
	var multierr error
	for _, pg := range s.scope.HetznerCluster.Status.HCloudPlacementGroups {
		if err := s.scope.HCloudClient.DeletePlacementGroup(ctx, pg.ID); err != nil {
			hcloudutil.HandleRateLimitExceeded(s.scope.HetznerCluster, err, "DeletePlacementGroup")
			if !hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
				multierr = errors.Join(multierr, err)
			}
		}
	}

	if multierr != nil {
		return fmt.Errorf("aggregate error - deleting placement groups: %w", err)
	}

	record.Eventf(s.scope.HetznerCluster, "PlacementGroupsDeleted", "Deleted placement groups")

	return nil
}

func (s *Service) findPlacementGroups(ctx context.Context) ([]*hcloud.PlacementGroup, error) {
	clusterTagKey := s.scope.HetznerCluster.ClusterTagKey()
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

// statusFromHCloudPlacementGroups gets the information of the Hetzner placement groups and returns it in our status object.
func statusFromHCloudPlacementGroups(placementGroups []*hcloud.PlacementGroup, clusterName string) []infrav1.HCloudPlacementGroupStatus {
	status := make([]infrav1.HCloudPlacementGroupStatus, len(placementGroups))
	for i, pg := range placementGroups {
		status[i] = infrav1.HCloudPlacementGroupStatus{
			ID:     pg.ID,
			Server: pg.Servers,
			Name:   strings.TrimPrefix(pg.Name, clusterName+"-"),
			Type:   string(pg.Type),
		}
	}
	return status
}
