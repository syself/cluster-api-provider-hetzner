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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type args struct {
	oldSpec HCloudMachineSpec
	newSpec HCloudMachineSpec
}

func TestValidateHCloudMachineSpec(t *testing.T) {
	tests := []struct {
		name string
		args args
		want field.ErrorList
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
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "type"), "cx21", "field is immutable"),
			},
		},
		{
			name: "Immutable ImageName",
			args: args{
				oldSpec: HCloudMachineSpec{
					ImageName: "ubuntu-20.04",
				},
				newSpec: HCloudMachineSpec{
					ImageName: "centos-7",
				},
			},
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "imageName"), "centos-7", "field is immutable"),
			},
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
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "sshKeys"), []SSHKey{
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
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "placementGroupName"), "placement-group-2", "field is immutable"),
			},
		},
		{
			name: "No Errors",
			args: args{
				oldSpec: HCloudMachineSpec{
					Type:               "cpx11",
					ImageName:          "ubuntu-20.04",
					SSHKeys:            []SSHKey{{Name: "ssh-key-1", Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC"}},
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
				newSpec: HCloudMachineSpec{
					Type:               "cpx11",
					ImageName:          "ubuntu-20.04",
					SSHKeys:            []SSHKey{{Name: "ssh-key-1", Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC"}},
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
			},
			want: field.ErrorList{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHCloudMachineSpec(tt.args.oldSpec, tt.args.newSpec)

			if len(got) != len(tt.want) {
				t.Errorf("validateHCloudMachineSpec() = %v, want %v", got, tt.want)
				return
			}

			for i, err := range got {
				assert.Equal(t, tt.want[i].Type, err.Type)
				assert.Equal(t, tt.want[i].Field, err.Field)
				assert.Equal(t, tt.want[i].Detail, err.Detail)
			}
		})
	}
}

func createPlacementGroupName(name string) *string {
	return &name
}
