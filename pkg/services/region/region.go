package region

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
)

type Service struct {
	scope localScope
}

func NewService(scope localScope) *Service {
	return &Service{
		scope: scope,
	}
}

type localScope interface {
	HCloudClient() scope.HCloudClient
	GetSpecRegion() []infrav1.HCloudRegion
	SetStatusRegion(region []infrav1.HCloudRegion, networkZone infrav1.HCloudNetworkZone)
}

func (s *Service) Reconcile(ctx context.Context) (err error) {
	allRegions, err := s.scope.HCloudClient().ListLocation(ctx)
	if err != nil {
		return err
	}
	allRegionsMap := make(map[string]*hcloud.Location)
	for _, l := range allRegions {
		allRegionsMap[l.Name] = l
	}

	var regions []string
	var networkZone *infrav1.HCloudNetworkZone

	// if no regions have been specified, use the default networkZone
	specRegions := s.scope.GetSpecRegion()
	if len(specRegions) == 0 {
		nZ := infrav1.HCloudNetworkZone(hcloud.NetworkZoneEUCentral)
		networkZone = &nZ
		for _, l := range allRegions {
			if nZ == infrav1.HCloudNetworkZone(l.NetworkZone) {
				regions = append(regions, l.Name)
			}
		}
	}

	for _, l := range specRegions {
		region, ok := allRegionsMap[string(l)]
		if !ok {
			return fmt.Errorf("error region '%s' cannot be found", l)
		}
		nZ := infrav1.HCloudNetworkZone(region.NetworkZone)

		if networkZone == nil {
			networkZone = &nZ
		}

		if *networkZone != nZ {
			return fmt.Errorf(
				"error all regions need to be in the same NetworkZone. %s in NetworkZone %s, %s in NetworkZone %s",
				strings.Join(regions, ","),
				*networkZone,
				region.Name,
				nZ,
			)
		}
		regions = append(regions, region.Name)
	}

	if len(regions) == 0 {
		return fmt.Errorf("error regions is empty")
	}

	sort.Strings(regions)
	regionsTyped := make([]infrav1.HCloudRegion, len(regions))
	for pos := range regions {
		regionsTyped[pos] = infrav1.HCloudRegion(regions[pos])
	}

	s.scope.SetStatusRegion(regionsTyped, *networkZone)
	return nil
}
