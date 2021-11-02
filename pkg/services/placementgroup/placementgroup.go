package placementgroup

import (
	"context"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	errorutil "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

type Service struct {
	scope *scope.ClusterScope
}

func NewService(scope *scope.ClusterScope) *Service {
	return &Service{
		scope: scope,
	}
}

func (s *Service) Reconcile(ctx context.Context) (err error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconcile placement groups")

	// find placement groups
	placementGroups, err := s.findPlacementGroups()
	if err != nil {
		return errors.Wrap(err, "failed to find placement group")
	}
	s.scope.HetznerCluster.Status.PlacementGroup = s.apiToStatus(placementGroups)

	// Make a diff between placement groups in status and in specs, to see if some need to be deleted or created

	// Delete placement groups which are not in status but in specs
	var multierr []error
	for _, pg := range placementGroups {
		var foundInSpecs bool
		for _, pgSpec := range s.scope.HetznerCluster.Spec.PlacementGroupSpec {
			if pg.Name == pgSpec.Name {
				foundInSpecs = true
				break
			}
		}
		if !foundInSpecs {
			if _, err := s.scope.HCloudClient().DeletePlacementGroup(ctx, pg.ID); err != nil {
				multierr = append(multierr, err)
			}
		}
	}

	if err := errorutil.NewAggregate(multierr); err != nil {
		log.Error(err, "aggregate error - deleting placement groups")
	}

	// Create placement groups which are in specs but not in status
	multierr = []error{}
	for _, pgSpec := range s.scope.HetznerCluster.Spec.PlacementGroupSpec {
		var foundInStatus bool
		for _, pg := range placementGroups {
			if pg.Name == pgSpec.Name {
				foundInStatus = true
				break
			}
		}
		if !foundInStatus {
			clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
			if _, _, err := s.scope.HCloudClient().CreatePlacementGroup(ctx, hcloud.PlacementGroupCreateOpts{
				Name:   pgSpec.Name,
				Type:   hcloud.PlacementGroupType(pgSpec.Type),
				Labels: map[string]string{clusterTagKey: string(infrav1.ResourceLifecycleOwned)},
			}); err != nil {
				multierr = append(multierr, err)
			}
		}
	}

	if err := errorutil.NewAggregate(multierr); err != nil {
		log.Error(err, "aggregate error - creating placement groups")
	}

	// find placement groups
	placementGroups, err = s.findPlacementGroups()
	if err != nil {
		return errors.Wrap(err, "failed to find placement group")
	}

	s.scope.HetznerCluster.Status.PlacementGroup = s.apiToStatus(placementGroups)

	return nil
}

func (s *Service) Delete(ctx context.Context) (err error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Delete placement groups")

	// Delete placement groups which are not in status but in specs
	var multierr []error
	for _, pg := range s.scope.HetznerCluster.Status.PlacementGroup {
		if _, err := s.scope.HCloudClient().DeletePlacementGroup(ctx, pg.ID); err != nil {
			multierr = append(multierr, err)
		}
	}

	if err := errorutil.NewAggregate(multierr); err != nil {
		log.Error(err, "aggregate error - deleting placement groups")
		return err
	}

	record.Eventf(s.scope.HetznerCluster, "PlacementGroupsDeleted", "Deleted placement groups")

	return nil
}

func (s *Service) findPlacementGroups() ([]*hcloud.PlacementGroup, error) {
	clusterTagKey := infrav1.ClusterTagKey(s.scope.HetznerCluster.Name)
	labels := map[string]string{clusterTagKey: string(infrav1.ResourceLifecycleOwned)}
	opts := hcloud.PlacementGroupListOpts{}
	opts.LabelSelector = utils.LabelsToLabelSelector(labels)

	return s.scope.HCloudClient().ListPlacementGroups(s.scope.Ctx, opts)
}

// gets the information of the Hetzner load balancer object and returns it in our status object.
func (s *Service) apiToStatus(placementGroups []*hcloud.PlacementGroup) (status []infrav1.HCloudPlacementGroupStatus) {
	for _, pg := range placementGroups {
		status = append(status, infrav1.HCloudPlacementGroupStatus{
			ID:     pg.ID,
			Server: pg.Servers,
			Name:   pg.Name,
			Type:   string(pg.Type),
		})
	}

	return status
}
