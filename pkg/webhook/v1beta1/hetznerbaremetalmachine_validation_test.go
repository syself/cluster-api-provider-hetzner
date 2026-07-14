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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func TestValidateHetznerBareMetalMachineSpecCreate(t *testing.T) {
	type args struct {
		spec infrav1.HetznerBareMetalMachineSpec
	}
	tests := []struct {
		name string
		args args
		want *field.Error
	}{
		{
			name: "Valid Image",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
				},
			},
			want: nil,
		},
		{
			name: "Valid Image Path",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Path: "path/to/image.tar.gz",
						},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
				},
			},
			want: nil,
		},
		{
			name: "Valid Image URL Command",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						ImageURLCommand: "image-url-command-bm-test.sh",
						Image: infrav1.Image{
							URL: "oci://ghcr.io/example/ubuntu:v1",
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Valid Image URL Command with DeviceStringType wwn",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						ImageURLCommand:  "image-url-command-bm-test.sh",
						DeviceStringType: infrav1.DeviceStringTypeWWN,
						Image: infrav1.Image{
							URL: "oci://ghcr.io/example/ubuntu:v1",
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Invalid DeviceStringType wwn without imageURLCommand",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						DeviceStringType: infrav1.DeviceStringTypeWWN,
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "installImage", "deviceStringType"), infrav1.DeviceStringTypeWWN, "deviceStringType is only valid when imageURLCommand is set"),
		},
		{
			name: "Invalid Image",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image:      infrav1.Image{},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "installImage", "image"), infrav1.Image{}, "have to specify either image name and url or path"),
		},
		{
			name: "Invalid Image URL",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.invalid",
						},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "installImage", "image", "url"), "https://example.com/ubuntu-24.04.invalid", "unknown image type in URL"),
		},
		{
			name: "Invalid Image URL Command Without URL",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						ImageURLCommand: "image-url-command-bm-test.sh",
						Image:           infrav1.Image{},
					},
				},
			},
			want: field.Required(field.NewPath("spec", "installImage", "image", "url"), "url is required when imageURLCommand is set"),
		},
		{
			name: "Invalid Image URL Command With Image Name",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						ImageURLCommand: "image-url-command-bm-test.sh",
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "oci://ghcr.io/example/ubuntu:v1",
						},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "installImage", "image", "name"), "ubuntu-24.04", "name must be empty when imageURLCommand is set"),
		},
		{
			name: "Invalid Image URL Command With Slash",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						ImageURLCommand: "/shared/image-url-command-bm-test.sh",
						Image: infrav1.Image{
							URL: "oci://ghcr.io/example/ubuntu:v1",
						},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "installImage", "imageURLCommand"), "/shared/image-url-command-bm-test.sh", "must be a basename without slashes"),
		},
		{
			name: "Invalid Image URL Command Without Prefix",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						ImageURLCommand: "1bad-command",
						Image: infrav1.Image{
							URL: "oci://ghcr.io/example/ubuntu:v1",
						},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "installImage", "imageURLCommand"), "1bad-command", "must match the regex ^[a-z][a-z0-9._-]*$"),
		},
		{
			name: "Invalid Image URL Command With Dot Dot",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						ImageURLCommand: "image-url-command-bm..test.sh",
						Image: infrav1.Image{
							URL: "oci://ghcr.io/example/ubuntu:v1",
						},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "installImage", "imageURLCommand"), "image-url-command-bm..test.sh", "must not contain '..'"),
		},
		{
			name: "Valid HostSelector MatchLabels",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
					HostSelector: infrav1.HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Valid HostSelector MatchExpressions",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
					HostSelector: infrav1.HostSelector{
						MatchExpressions: []infrav1.HostSelectorRequirement{
							{
								Key:      "key1",
								Operator: selection.In,
								Values:   []string{"value1", "value2"},
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Invalid HostSelector MatchExpressions - Invalid Operator",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
					HostSelector: infrav1.HostSelector{
						MatchExpressions: []infrav1.HostSelectorRequirement{
							{
								Key:      "key1",
								Operator: selection.Operator("Invalid"),
								Values:   []string{"value1", "value2"},
							},
						},
					},
				},
			},
			want: field.Invalid(
				field.NewPath("spec", "hostSelector", "matchExpressions"),
				[]infrav1.HostSelectorRequirement{
					{
						Key:      "key1",
						Operator: selection.Operator("Invalid"),
						Values:   []string{"value1", "value2"},
					},
				},
				`invalid match expression: operator: Unsupported value: "invalid": supported values: "in", "notin", "=", "==", "!=", "gt", "lt", "exists", "!"`,
			),
		},
		{
			name: "Invalid HostSelector MatchExpressions - Empty Key",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
						Partitions: []infrav1.Partition{{Mount: "/", FileSystem: "ext4", Size: "all"}},
					},
					HostSelector: infrav1.HostSelector{
						MatchExpressions: []infrav1.HostSelectorRequirement{
							{
								Key:      "",
								Operator: selection.In,
								Values:   []string{"value1", "value2"},
							},
						},
					},
				},
			},
			want: field.Invalid(
				field.NewPath("spec", "hostSelector", "matchExpressions"),
				[]infrav1.HostSelectorRequirement{
					{
						Key:      "",
						Operator: selection.In,
						Values:   []string{"value1", "value2"},
					},
				},
				`invalid match expression: key: Invalid value: "": name part must be non-empty; name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
			),
		},
		{
			name: "Partitions required without imageURLCommand",
			args: args{
				spec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
					},
				},
			},
			want: field.Required(field.NewPath("spec", "installImage", "partitions"),
				"partitions must be set when imageURLCommand is not set"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHetznerBareMetalMachineSpecCreate(tt.args.spec)

			if tt.want == nil {
				assert.Empty(t, got)
				return
			}

			if assert.Len(t, got, 1) {
				assert.Equal(t, tt.want.Type, got[0].Type)
				assert.Equal(t, tt.want.Field, got[0].Field)
				assert.Equal(t, tt.want.Detail, got[0].Detail)
			}
		})
	}
}

func TestValidateHetznerBareMetalMachineSpecUpdate(t *testing.T) {
	type args struct {
		oldSpec infrav1.HetznerBareMetalMachineSpec
		newSpec infrav1.HetznerBareMetalMachineSpec
	}
	tests := []struct {
		name string
		args args
		want *field.Error
	}{
		{
			name: "Immutable InstallImage",
			args: args{
				oldSpec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
					},
				},
				newSpec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "centos-7",
							URL:  "https://example.com/centos-7.tar.gz",
						},
					},
				},
			},
			want: field.Forbidden(field.NewPath("spec", "installImage"), "installImage is immutable"),
		},
		{
			name: "Immutable SSHSpec",
			args: args{
				oldSpec: infrav1.HetznerBareMetalMachineSpec{
					SSHSpec: infrav1.SSHSpec{
						SecretRef: infrav1.SSHSecretRef{
							Name: "ssh-secret",
							Key: infrav1.SSHSecretKeyRef{
								Name:       "ssh-key-name",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 22,
					},
				},
				newSpec: infrav1.HetznerBareMetalMachineSpec{
					SSHSpec: infrav1.SSHSpec{
						SecretRef: infrav1.SSHSecretRef{
							Name: "ssh-secret-new",
							Key: infrav1.SSHSecretKeyRef{
								Name:       "ssh-key-name-new",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 2222,
					},
				},
			},
			want: field.Forbidden(field.NewPath("spec", "sshSpec"), "sshSpec is immutable"),
		},
		{
			name: "Immutable HostSelector",
			args: args{
				oldSpec: infrav1.HetznerBareMetalMachineSpec{
					HostSelector: infrav1.HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
						MatchExpressions: []infrav1.HostSelectorRequirement{
							{
								Key:      "key2",
								Operator: selection.In,
								Values:   []string{"value2"},
							},
						},
					},
				},
				newSpec: infrav1.HetznerBareMetalMachineSpec{
					HostSelector: infrav1.HostSelector{
						MatchLabels: map[string]string{
							"key3": "value3",
						},
						MatchExpressions: []infrav1.HostSelectorRequirement{
							{
								Key:      "key4",
								Operator: selection.In,
								Values:   []string{"value4"},
							},
						},
					},
				},
			},
			want: field.Forbidden(field.NewPath("spec", "hostSelector"), "hostSelector is immutable"),
		},
		{
			name: "No Errors",
			args: args{
				oldSpec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
					},
					SSHSpec: infrav1.SSHSpec{
						SecretRef: infrav1.SSHSecretRef{
							Name: "ssh-secret",
							Key: infrav1.SSHSecretKeyRef{
								Name:       "ssh-key-name",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 22,
					},
					HostSelector: infrav1.HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
						MatchExpressions: []infrav1.HostSelectorRequirement{
							{
								Key:      "key2",
								Operator: selection.In,
								Values:   []string{"value2"},
							},
						},
					},
				},
				newSpec: infrav1.HetznerBareMetalMachineSpec{
					InstallImage: infrav1.InstallImage{
						Image: infrav1.Image{
							Name: "ubuntu-24.04",
							URL:  "https://example.com/ubuntu-24.04.tar.gz",
						},
					},
					SSHSpec: infrav1.SSHSpec{
						SecretRef: infrav1.SSHSecretRef{
							Name: "ssh-secret",
							Key: infrav1.SSHSecretKeyRef{
								Name:       "ssh-key-name",
								PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
								PrivateKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
							},
						},
						PortAfterInstallImage: 22,
					},
					HostSelector: infrav1.HostSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
						MatchExpressions: []infrav1.HostSelectorRequirement{
							{
								Key:      "key2",
								Operator: selection.In,
								Values:   []string{"value2"},
							},
						},
					},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHetznerBareMetalMachineSpecUpdate(tt.args.oldSpec, tt.args.newSpec)

			if len(got) == 0 {
				assert.Empty(t, got)
			}

			if len(got) > 1 {
				t.Errorf("got length: %d greater than 1", len(got))
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

func TestValidateHetznerBareMetalMachineSpecUpdate_ProviderID(t *testing.T) {
	got := validateHetznerBareMetalMachineSpecUpdate(
		infrav1.HetznerBareMetalMachineSpec{
			ProviderID: ptr.To("provider://foo"),
		},
		infrav1.HetznerBareMetalMachineSpec{})
	require.Equal(t, `[spec.providerID: Forbidden: providerID is immutable]`, fmt.Sprintf("%+v", got))

	got = validateHetznerBareMetalMachineSpecUpdate(
		infrav1.HetznerBareMetalMachineSpec{
			ProviderID: ptr.To("provider://foo"),
		},
		infrav1.HetznerBareMetalMachineSpec{
			ProviderID: ptr.To("provider://bar"),
		})
	require.Equal(t, `[spec.providerID: Forbidden: providerID is immutable]`, fmt.Sprintf("%+v", got))

	// Allowed Updates
	got = validateHetznerBareMetalMachineSpecUpdate(
		infrav1.HetznerBareMetalMachineSpec{},
		infrav1.HetznerBareMetalMachineSpec{})
	require.Equal(t, `[]`, fmt.Sprintf("%+v", got))

	got = validateHetznerBareMetalMachineSpecUpdate(
		infrav1.HetznerBareMetalMachineSpec{},
		infrav1.HetznerBareMetalMachineSpec{
			ProviderID: ptr.To("provider://bar"),
		})
	require.Equal(t, `[]`, fmt.Sprintf("%+v", got))

	got = validateHetznerBareMetalMachineSpecUpdate(
		infrav1.HetznerBareMetalMachineSpec{
			ProviderID: ptr.To("provider://bar"),
		},
		infrav1.HetznerBareMetalMachineSpec{
			ProviderID: ptr.To("provider://bar"),
		})
	require.Equal(t, `[]`, fmt.Sprintf("%+v", got))
}
