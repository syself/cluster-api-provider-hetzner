package registry

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

type ListOptions struct {
	FieldSelector string
}

type registryFields api.Registry

func (cf *registryFields) Has(field string) bool {
	return field == "name"
}

func (cf *registryFields) Get(field string) string {
	if field == "name" {
		return (*api.Registry)(cf).Name
	}
	if field == "port" {
		return fmt.Sprintf("%d", (*api.Registry)(cf).Port)
	}
	return ""
}

var _ fields.Fields = &registryFields{}
