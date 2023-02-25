package cmd

import (
	"context"

	"github.com/tilt-dev/clusterid"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

type clusterGetter interface {
	Get(ctx context.Context, name string) (*api.Cluster, error)
}

// We create clusters like:
// ctlptl create cluster kind
// For most clusters, the name of the cluster will match the name of the product.
// But for cases where they don't match, we want
// `ctlptl delete cluster kind` to automatically map to `ctlptl delete cluster kind-kind`
func normalizedGet(ctx context.Context, controller clusterGetter, name string) (*api.Cluster, error) {
	cluster, err := controller.Get(ctx, name)
	if err == nil {
		return cluster, nil
	}

	if !errors.IsNotFound(err) {
		return nil, err
	}

	origErr := err
	retryName := ""
	if name == string(clusterid.ProductKIND) {
		retryName = clusterid.ProductKIND.DefaultClusterName()
	} else if name == string(clusterid.ProductK3D) {
		retryName = clusterid.ProductK3D.DefaultClusterName()
	}

	if retryName == "" {
		return nil, origErr
	}

	cluster, err = controller.Get(ctx, retryName)
	if err == nil {
		return cluster, nil
	}
	return nil, origErr
}
