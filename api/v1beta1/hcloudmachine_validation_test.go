/*
Copyright 2024 The Kubernetes Authors.

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

package v1beta1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type args struct {
	oldSpec HCloudMachineSpec
	newSpec HCloudMachineSpec
}

func TestValidateHCloudMachineSpecUpdate(t *testing.T) {
	tests := []struct {
		name string
		args args
		want *field.Error
	}{
		{
			name: "Immutable Type",
			args: args{
				oldSpec: HCloudMachineSpec{
					Type: "cpx11",
				},
				newSpec: HCloudMachineSpec{
					Type: "cx21",
				},
			},
			want: field.Invalid(field.NewPath("spec", "type"), "cx21", "field is immutable"),
		},
		{
			name: "Immutable ImageName",
			args: args{
				oldSpec: HCloudMachineSpec{
					ImageName: "ubuntu-24.04",
				},
				newSpec: HCloudMachineSpec{
					ImageName: "centos-7",
				},
			},
			want: field.Invalid(field.NewPath("spec", "imageName"), "centos-7", "field is immutable"),
		},
		{
			name: "Immutable ImageURL",
			args: args{
				oldSpec: HCloudMachineSpec{
					ImageURL: "oci://ghcr.io/example/foo:v1",
				},
				newSpec: HCloudMachineSpec{
					ImageURL: "oci://ghcr.io/example/foo:v2",
				},
			},
			want: field.Invalid(field.NewPath("spec", "imageURL"), "oci://ghcr.io/example/foo:v2", "field is immutable"),
		},
		{
			name: "ImageURL and ImageName mutually exclusive",
			args: args{
				oldSpec: HCloudMachineSpec{
					ImageURL:  "oci://ghcr.io/example/foo:v1",
					ImageName: "dummy-name",
				},
				newSpec: HCloudMachineSpec{
					ImageURL:  "oci://ghcr.io/example/foo:v1",
					ImageName: "dummy-name",
				},
			},
			want: field.Invalid(field.NewPath("spec", "imageName"), "dummy-name", "imageName and imageURL are mutually exclusive"),
		},
		{
			name: "Immutable SSHKeys",
			args: args{
				oldSpec: HCloudMachineSpec{
					SSHKeys: []SSHKey{
						{
							Name:        "ssh-key-1",
							Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
						},
					},
				},
				newSpec: HCloudMachineSpec{
					SSHKeys: []SSHKey{
						{
							Name:        "ssh-key-1",
							Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
						},
						{
							Name:        "ssh-key-2",
							Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
						},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "sshKeys"), []SSHKey{
				{
					Name:        "ssh-key-1",
					Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
				},
				{
					Name:        "ssh-key-2",
					Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
				},
			}, "field is immutable"),
		},
		{
			name: "Immutable PlacementGroupName",
			args: args{
				oldSpec: HCloudMachineSpec{
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
				newSpec: HCloudMachineSpec{
					PlacementGroupName: createPlacementGroupName("placement-group-2"),
				},
			},
			want: field.Invalid(field.NewPath("spec", "placementGroupName"), "placement-group-2", "field is immutable"),
		},
		{
			name: "No Errors",
			args: args{
				oldSpec: HCloudMachineSpec{
					Type:               "cpx11",
					ImageName:          "ubuntu-24.04",
					SSHKeys:            []SSHKey{{Name: "ssh-key-1"}},
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
				newSpec: HCloudMachineSpec{
					Type:               "cpx11",
					ImageName:          "ubuntu-24.04",
					SSHKeys:            []SSHKey{{Name: "ssh-key-1"}},
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHCloudMachineSpecUpdate(tt.args.oldSpec, tt.args.newSpec)

			if len(got) == 0 {
				assert.Empty(t, tt.want)
			}

			if len(got) > 1 {
				t.Errorf("got length: %d greater than 1: %+v", len(got), got)
			}

			// assert if length of got is 1
			if len(got) == 1 {
				assert.Equal(t, tt.want.Type, got[0].Type)
				assert.Equal(t, tt.want.Field, got[0].Field)
				assert.Equal(t, tt.want.Detail, got[0].Detail)
			}
		})
	}
}

func createPlacementGroupName(name string) *string {
	return &name
}

func TestValidateHCloudMachineSpec(t *testing.T) {
	allErrs := validateHCloudMachineSpec(HCloudMachineSpec{
		ImageURL: "not-a-valid-url",
	})
	require.Equal(t, `spec.imageURL: Invalid value: "not-a-valid-url": parse "not-a-valid-url": invalid URI for request`, errorsToString(allErrs))

	allErrs = validateHCloudMachineSpec(HCloudMachineSpec{
		ImageName: "foo-name",
		ImageURL:  "oci://ghcr.io/example/foo:v1",
	})
	require.Equal(t, `spec.imageName: Invalid value: "foo-name": imageName and imageURL are mutually exclusive`, errorsToString(allErrs))
}

func errorsToString(allErrs field.ErrorList) string {
	var s []string
	for _, err := range allErrs {
		s = append(s, err.Error())
	}
	return strings.Join(s, "\n")
}
