/*
Copyright 2026 The Kubernetes Authors.

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

package v1beta2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultConstantsMatchCRDDefaults checks that each Default constant has the same value as the
// default of its field in the generated CRD. The API server sets the CRD default. The conversion,
// the webhooks and the controllers use the constant when the field is not set. Both must give the
// same value.
func TestDefaultConstantsMatchCRDDefaults(t *testing.T) {
	const (
		hcloudMachineCRD    = "infrastructure.cluster.x-k8s.io_hcloudmachines.yaml"
		bareMetalMachineCRD = "infrastructure.cluster.x-k8s.io_hetznerbaremetalmachines.yaml"
		hetznerClusterCRD   = "infrastructure.cluster.x-k8s.io_hetznerclusters.yaml"
	)

	tests := []struct {
		name      string
		crdFile   string
		fieldPath []string
		constant  any
	}{
		{
			name:      "HCloudMachine publicNetwork.enableIPv4",
			crdFile:   hcloudMachineCRD,
			fieldPath: []string{"spec", "publicNetwork", "enableIPv4"},
			constant:  DefaultEnableIPv4,
		},
		{
			name:      "HCloudMachine publicNetwork.enableIPv6",
			crdFile:   hcloudMachineCRD,
			fieldPath: []string{"spec", "publicNetwork", "enableIPv6"},
			constant:  DefaultEnableIPv6,
		},
		{
			name:      "HetznerBareMetalMachine sshSpec.portAfterInstallImage",
			crdFile:   bareMetalMachineCRD,
			fieldPath: []string{"spec", "sshSpec", "portAfterInstallImage"},
			constant:  DefaultPortAfterInstallImage,
		},
		{
			name:      "HetznerBareMetalMachine installImage.swraid",
			crdFile:   bareMetalMachineCRD,
			fieldPath: []string{"spec", "installImage", "swraid"},
			constant:  DefaultSwraid,
		},
		{
			name:      "HetznerBareMetalMachine installImage.swraidLevel",
			crdFile:   bareMetalMachineCRD,
			fieldPath: []string{"spec", "installImage", "swraidLevel"},
			constant:  DefaultSwraidLevel,
		},
		{
			name:      "HetznerBareMetalMachine customProvisioner.swraid",
			crdFile:   bareMetalMachineCRD,
			fieldPath: []string{"spec", "customProvisioner", "swraid"},
			constant:  DefaultSwraid,
		},
		{
			name:      "HetznerCluster hetznerSecretRef.key.hcloudToken",
			crdFile:   hetznerClusterCRD,
			fieldPath: []string{"spec", "hetznerSecretRef", "key", "hcloudToken"},
			constant:  DefaultHCloudTokenKey,
		},
		{
			name:      "HetznerCluster hetznerSecretRef.key.hetznerRobotUser",
			crdFile:   hetznerClusterCRD,
			fieldPath: []string{"spec", "hetznerSecretRef", "key", "hetznerRobotUser"},
			constant:  DefaultHetznerRobotUserKey,
		},
		{
			name:      "HetznerCluster hetznerSecretRef.key.hetznerRobotPassword",
			crdFile:   hetznerClusterCRD,
			fieldPath: []string{"spec", "hetznerSecretRef", "key", "hetznerRobotPassword"},
			constant:  DefaultHetznerRobotPasswordKey,
		},
		{
			name:      "HetznerCluster hetznerSecretRef.key.sshKey",
			crdFile:   hetznerClusterCRD,
			fieldPath: []string{"spec", "hetznerSecretRef", "key", "sshKey"},
			constant:  DefaultSSHKeyNameKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema := crdFieldSchema(t, tc.crdFile, tc.fieldPath)
			require.NotNil(t, schema.Default, "the field has no default in the CRD")

			want, err := json.Marshal(tc.constant)
			require.NoError(t, err)
			require.JSONEq(t, string(want), string(schema.Default.Raw),
				"the Default constant does not match the +kubebuilder:default marker of the field")
		})
	}
}
