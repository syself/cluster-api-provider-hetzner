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
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateHetznerBareMetalMachineSpecCreate(t *testing.T) {
	type args struct {
		spec HetznerBareMetalMachineSpec
	}
	tests := []struct {
		name string
		args args
		want field.ErrorList
	}{
		{
			name: "Valid Image",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "Valid Image Path",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Path: "path/to/image.tar.gz",
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "Invalid Image",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{},
					},
				},
			},
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "installImage", "image"), Image{}, "have to specify either image name and url or path"),
			},
		},
		{
			name: "Invalid Image URL",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.invalid",
						},
					},
				},
			},
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "installImage", "image", "url"), "https://example.com/ubuntu-20.04.invalid", "unknown image type in URL"),
			},
		},
		{
			name: "Valid HostSelector MatchLabels",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
					HostSelector: HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "Valid HostSelector MatchExpressions",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
					HostSelector: HostSelector{
						MatchExpressions: []HostSelectorRequirement{
							{
								Key:      "key1",
								Operator: selection.In,
								Values:   []string{"value1", "value2"},
							},
						},
					},
				},
			},
			want: field.ErrorList{},
		},
		{
			name: "Invalid HostSelector MatchExpressions - Invalid Operator",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
					HostSelector: HostSelector{
						MatchExpressions: []HostSelectorRequirement{
							{
								Key:      "key1",
								Operator: selection.Operator("Invalid"),
								Values:   []string{"value1", "value2"},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "hostSelector", "matchExpressions"),
					[]HostSelectorRequirement{
						{
							Key:      "key1",
							Operator: selection.Operator("Invalid"),
							Values:   []string{"value1", "value2"},
						},
					},
					`invalid match expression: operator: Unsupported value: "invalid": supported values: "in", "notin", "=", "==", "!=", "gt", "lt", "exists", "!"`,
				),
			},
		},
		{
			name: "Invalid HostSelector MatchExpressions - Empty Key",
			args: args{
				spec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
					HostSelector: HostSelector{
						MatchExpressions: []HostSelectorRequirement{
							{
								Key:      "",
								Operator: selection.In,
								Values:   []string{"value1", "value2"},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "hostSelector", "matchExpressions"),
					[]HostSelectorRequirement{
						{
							Key:      "",
							Operator: selection.In,
							Values:   []string{"value1", "value2"},
						},
					},
					`invalid match expression: key: Invalid value: "": name part must be non-empty; name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHetznerBareMetalMachineSpecCreate(tt.args.spec)
			if len(got) != len(tt.want) {
				t.Errorf("validateHetznerBareMetalMachineSpecCreate() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				assert.Equal(t, tt.want[i].Type, got[i].Type)
				assert.Equal(t, tt.want[i].Field, got[i].Field)
				assert.Equal(t, tt.want[i].Detail, got[i].Detail)
			}
		})
	}
}

func TestValidateHetznerBareMetalMachineSpecUpdate(t *testing.T) {
	type args struct {
		oldSpec HetznerBareMetalMachineSpec
		newSpec HetznerBareMetalMachineSpec
	}
	tests := []struct {
		name string
		args args
		want field.ErrorList
	}{
		{
			name: "Immutable InstallImage",
			args: args{
				oldSpec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
				},
				newSpec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "centos-7",
							URL:  "https://example.com/centos-7.tar.gz",
						},
					},
				},
			},
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "installImage"), InstallImage{
					Image: Image{
						Name: "centos-7",
						URL:  "https://example.com/centos-7.tar.gz",
					},
				}, "installImage immutable"),
			},
		},
		{
			name: "Immutable SSHSpec",
			args: args{
				oldSpec: HetznerBareMetalMachineSpec{
					SSHSpec: SSHSpec{
						SecretRef: SSHSecretRef{
							Name: "ssh-secret",
							Key: SSHSecretKeyRef{
								Name:       "ssh-key-name",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 22,
						PortAfterCloudInit:    22,
					},
				},
				newSpec: HetznerBareMetalMachineSpec{
					SSHSpec: SSHSpec{
						SecretRef: SSHSecretRef{
							Name: "ssh-secret-new",
							Key: SSHSecretKeyRef{
								Name:       "ssh-key-name-new",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 2222,
						PortAfterCloudInit:    2222,
					},
				},
			},
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "sshSpec"), SSHSpec{
					SecretRef: SSHSecretRef{
						Name: "ssh-secret-new",
						Key: SSHSecretKeyRef{
							Name:       "ssh-key-name-new",
							PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
						},
					},
					PortAfterInstallImage: 2222,
					PortAfterCloudInit:    2222,
				}, "sshSpec immutable"),
			},
		},
		{
			name: "Immutable HostSelector",
			args: args{
				oldSpec: HetznerBareMetalMachineSpec{
					HostSelector: HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
						MatchExpressions: []HostSelectorRequirement{
							{
								Key:      "key2",
								Operator: selection.In,
								Values:   []string{"value2"},
							},
						},
					},
				},
				newSpec: HetznerBareMetalMachineSpec{
					HostSelector: HostSelector{
						MatchLabels: map[string]string{
							"key3": "value3",
						},
						MatchExpressions: []HostSelectorRequirement{
							{
								Key:      "key4",
								Operator: selection.In,
								Values:   []string{"value4"},
							},
						},
					},
				},
			},
			want: field.ErrorList{
				field.Invalid(field.NewPath("spec", "hostSelector"), HostSelector{
					MatchLabels: map[string]string{
						"key3": "value3",
					},
					MatchExpressions: []HostSelectorRequirement{
						{
							Key:      "key4",
							Operator: selection.In,
							Values:   []string{"value4"},
						},
					},
				}, "hostSelector immutable"),
			},
		},
		{
			name: "No Errors",
			args: args{
				oldSpec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
					SSHSpec: SSHSpec{
						SecretRef: SSHSecretRef{
							Name: "ssh-secret",
							Key: SSHSecretKeyRef{
								Name:       "ssh-key-name",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 22,
						PortAfterCloudInit:    22,
					},
					HostSelector: HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
						MatchExpressions: []HostSelectorRequirement{
							{
								Key:      "key2",
								Operator: selection.In,
								Values:   []string{"value2"},
							},
						},
					},
				},
				newSpec: HetznerBareMetalMachineSpec{
					InstallImage: InstallImage{
						Image: Image{
							Name: "ubuntu-20.04",
							URL:  "https://example.com/ubuntu-20.04.tar.gz",
						},
					},
					SSHSpec: SSHSpec{
						SecretRef: SSHSecretRef{
							Name: "ssh-secret",
							Key: SSHSecretKeyRef{
								Name:       "ssh-key-name",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 22,
						PortAfterCloudInit:    22,
					},
					HostSelector: HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
						MatchExpressions: []HostSelectorRequirement{
							{
								Key:      "key2",
								Operator: selection.In,
								Values:   []string{"value2"},
							},
						},
					},
				},
			},
			want: field.ErrorList{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHetznerBareMetalMachineSpecUpdate(tt.args.oldSpec, tt.args.newSpec)
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
