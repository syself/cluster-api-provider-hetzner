package visitor

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/ctlptl/pkg/encoding"
)

func DecodeAll(vs []Interface) ([]runtime.Object, error) {
	result := []runtime.Object{}
	for _, v := range vs {
		objs, err := Decode(v)
		if err != nil {
			return nil, err
		}
		result = append(result, objs...)
	}
	return result, nil
}

func Decode(v Interface) ([]runtime.Object, error) {
	r, err := v.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	result, err := encoding.ParseStream(r)
	if err != nil {
		return nil, errors.Wrapf(err, "visiting %s", v.Name())
	}
	return result, nil
}
