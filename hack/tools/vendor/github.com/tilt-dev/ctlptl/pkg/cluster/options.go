package cluster

import (
	"k8s.io/apimachinery/pkg/fields"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

type ListOptions struct {
	FieldSelector string
}

type clusterFields api.Cluster

func (cf *clusterFields) Has(field string) bool {
	return field == "name" || field == "product"
}

func (cf *clusterFields) Get(field string) string {
	if field == "name" {
		return (*api.Cluster)(cf).Name
	}
	if field == "product" {
		return (*api.Cluster)(cf).Product
	}
	return ""
}

var _ fields.Fields = &clusterFields{}
