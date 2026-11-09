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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// TestIsDefined checks every IsDefined helper with a nil receiver, the zero value and one set field.
// The helpers that compare with reflect.DeepEqual also count a pointer to a zero value as set,
// because the pointer is not nil.
func TestIsDefined(t *testing.T) {
	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{name: "HCloudCustomProvisioner nil is not defined", got: (*HCloudCustomProvisioner)(nil).IsDefined(), want: false},
		{name: "HCloudCustomProvisioner zero value is not defined", got: (&HCloudCustomProvisioner{}).IsDefined(), want: false},
		{name: "HCloudCustomProvisioner with url is defined", got: (&HCloudCustomProvisioner{URL: "https://example.com/image"}).IsDefined(), want: true},
		{name: "HCloudCustomProvisioner with command is defined", got: (&HCloudCustomProvisioner{Command: "provision.sh"}).IsDefined(), want: true},

		{name: "OngoingReboot nil is not defined", got: (*OngoingReboot)(nil).IsDefined(), want: false},
		{name: "OngoingReboot zero value is not defined", got: (&OngoingReboot{}).IsDefined(), want: false},
		{name: "OngoingReboot with type is defined", got: (&OngoingReboot{Type: RebootTypeSoftware}).IsDefined(), want: true},
		{name: "OngoingReboot with triggeredAt is defined", got: (&OngoingReboot{TriggeredAt: metav1.Now()}).IsDefined(), want: true},

		{name: "HetznerBareMetalHostConsumerReference nil is not defined", got: (*HetznerBareMetalHostConsumerReference)(nil).IsDefined(), want: false},
		{name: "HetznerBareMetalHostConsumerReference zero value is not defined", got: (&HetznerBareMetalHostConsumerReference{}).IsDefined(), want: false},
		{name: "HetznerBareMetalHostConsumerReference with kind is defined", got: (&HetznerBareMetalHostConsumerReference{Kind: "HetznerBareMetalMachine"}).IsDefined(), want: true},
		{name: "HetznerBareMetalHostConsumerReference with name is defined", got: (&HetznerBareMetalHostConsumerReference{Name: "bm-machine"}).IsDefined(), want: true},
		{name: "HetznerBareMetalHostConsumerReference with apiGroup is defined", got: (&HetznerBareMetalHostConsumerReference{APIGroup: GroupVersion.Group}).IsDefined(), want: true},

		{name: "SSHKey nil is not defined", got: (*SSHKey)(nil).IsDefined(), want: false},
		{name: "SSHKey zero value is not defined", got: (&SSHKey{}).IsDefined(), want: false},
		{name: "SSHKey with name is defined", got: (&SSHKey{Name: "ssh-key"}).IsDefined(), want: true},
		{name: "SSHKey with fingerprint is defined", got: (&SSHKey{Fingerprint: "aa:bb"}).IsDefined(), want: true},

		{name: "InstallImage nil is not defined", got: (*InstallImage)(nil).IsDefined(), want: false},
		{name: "InstallImage zero value is not defined", got: (&InstallImage{}).IsDefined(), want: false},
		{name: "InstallImage with image is defined", got: (&InstallImage{Image: Image{Name: "ubuntu"}}).IsDefined(), want: true},
		{name: "InstallImage with only swraid pointing to 0 is defined", got: (&InstallImage{Swraid: ptr.To[int32](0)}).IsDefined(), want: true},

		{name: "CustomProvisioner nil is not defined", got: (*CustomProvisioner)(nil).IsDefined(), want: false},
		{name: "CustomProvisioner zero value is not defined", got: (&CustomProvisioner{}).IsDefined(), want: false},
		{name: "CustomProvisioner with url is defined", got: (&CustomProvisioner{URL: "https://example.com/image"}).IsDefined(), want: true},
		{name: "CustomProvisioner with only swraid pointing to 0 is defined", got: (&CustomProvisioner{Swraid: ptr.To[int32](0)}).IsDefined(), want: true},

		{name: "RemediationStrategy nil is not defined", got: (*RemediationStrategy)(nil).IsDefined(), want: false},
		{name: "RemediationStrategy zero value is not defined", got: (&RemediationStrategy{}).IsDefined(), want: false},
		{name: "RemediationStrategy with type is defined", got: (&RemediationStrategy{Type: RemediationTypeReboot}).IsDefined(), want: true},
		{name: "RemediationStrategy with only timeoutSeconds pointing to 0 is defined", got: (&RemediationStrategy{TimeoutSeconds: ptr.To[int32](0)}).IsDefined(), want: true},

		{name: "BareMetalRemediationStrategy nil is not defined", got: (*BareMetalRemediationStrategy)(nil).IsDefined(), want: false},
		{name: "BareMetalRemediationStrategy zero value is not defined", got: (&BareMetalRemediationStrategy{}).IsDefined(), want: false},
		{name: "BareMetalRemediationStrategy with onExhaustion is defined", got: (&BareMetalRemediationStrategy{OnExhaustion: OnExhaustionRetire}).IsDefined(), want: true},
		{
			name: "BareMetalRemediationStrategy with only timeoutSeconds pointing to 0 is defined",
			got:  (&BareMetalRemediationStrategy{RemediationStrategy: RemediationStrategy{TimeoutSeconds: ptr.To[int32](0)}}).IsDefined(),
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("IsDefined() = %v, want %v", tc.got, tc.want)
			}
		})
	}
}
