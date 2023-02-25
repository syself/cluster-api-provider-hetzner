package encoding

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

// Parses a stream of YAML.
func ParseStream(r io.Reader) ([]runtime.Object, error) {
	var current bytes.Buffer
	reader := io.TeeReader(bufio.NewReader(r), &current)

	objDecoder := yaml.NewDecoder(&current)
	objDecoder.KnownFields(true)

	typeDecoder := yaml.NewDecoder(reader)
	result := []runtime.Object{}
	for {
		tm := api.TypeMeta{}
		if err := typeDecoder.Decode(&tm); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		obj, err := determineObj(tm)
		if err != nil {
			return nil, err
		}

		if err := objDecoder.Decode(obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "decoding %s", tm)
		}

		result = append(result, obj)
	}
	return result, nil
}

// Determines the object corresponding to this type meta
func determineObj(tm api.TypeMeta) (runtime.Object, error) {
	// decode specific (apiVersion, kind)
	switch tm.APIVersion {
	// Currently we only support ctlptl.dev/v1alpha1
	case "ctlptl.dev/v1alpha1":
		switch tm.Kind {
		case "Cluster":
			return &api.Cluster{}, nil
		case "Registry":
			return &api.Registry{}, nil
		default:
			return nil, fmt.Errorf("ctlptl config must contain: `kind: Cluster` or `kind: Registry`")
		}
	default:
		return nil, fmt.Errorf("ctlptl config must contain: `apiVersion: ctlptl.dev/v1alpha1`")
	}
}
