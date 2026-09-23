/*
Copyright 2022 The Kubernetes Authors.

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

package helpers

import (
	"fmt"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

const (
	sshFingerprint          = "my-fingerprint"
	defaultOSSSHKeyName     = "os-sshkey"
	defaultRescueSSHKeyName = "rescue-sshkey"

	// DefaultWWN specifies the default WWN.
	DefaultWWN = "eui.002538b411b2cee8"
	// DefaultWWN2 specifies the default WWN.
	DefaultWWN2 = "eui.0025388801b4dff2"
)

var defaultPlacementGroupName = "caph-placement-group"

var globalServerIDCounter int32

// BareMetalHost returns a bare metal host given options.
func BareMetalHost(name, namespace string, opts ...HostOpts) *infrav2.HetznerBareMetalHost {
	serverID := atomic.AddInt32(&globalServerIDCounter, 1)
	host := &infrav2.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrav2.HetznerBareMetalHostSpec{
			ServerID: int(serverID),
		},
	}
	for _, o := range opts {
		o(host)
	}
	return host
}

// HostOpts define options to customize the host spec.
type HostOpts func(*infrav2.HetznerBareMetalHost)

// WithError gives the option to define a host with an error in the status.
func WithError(errorType infrav2.ErrorType, errorMessage string) HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.SetError(errorType, errorMessage)
	}
}

// WithRebootTriggeredAt gives the option to define a host with a reboot timestamp set.
func WithRebootTriggeredAt(t metav1.Time) HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.Status.RebootTriggeredAt = t
	}
}

// WithRebootTypes gives the option to define a host with custom reboot types.
func WithRebootTypes(rebootTypes []infrav2.RebootType) HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.Status.RebootTypes = rebootTypes
	}
}

// WithRootDeviceHintWWN gives the option to define a host with root device hints.
func WithRootDeviceHintWWN() HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.Spec.RootDeviceHints = &infrav2.RootDeviceHints{
			WWN: DefaultWWN,
		}
	}
}

// WithRootDeviceHintRaid gives the option to define a host with root device hints.
func WithRootDeviceHintRaid() HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.Spec.RootDeviceHints = &infrav2.RootDeviceHints{
			Raid: infrav2.Raid{WWN: []string{DefaultWWN, DefaultWWN2}},
		}
	}
}

// WithClusterNameLabel gives the option to define a host with the cluster-name label. The host
// controller finds the Cluster through this label.
func WithClusterNameLabel(clusterName string) HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		if host.Labels == nil {
			host.Labels = make(map[string]string)
		}
		host.Labels[clusterv1.ClusterNameLabel] = clusterName
	}
}

// WithSSHStatus gives the option to define a host with ssh status.
func WithSSHStatus() HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.Status.SSHStatus = infrav2.SSHStatus{
			OSKey: &infrav2.SSHKey{
				Name:        defaultOSSSHKeyName,
				Fingerprint: sshFingerprint,
			},
			RescueKey: &infrav2.SSHKey{
				Name:        defaultRescueSSHKeyName,
				Fingerprint: sshFingerprint,
			},
		}
	}
}

// WithIPv4 gives the option to define a host with IP.
func WithIPv4() HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.Status.IPv4 = "1.2.3.4"
	}
}

// WithConsumerRef gives the option to define a host with consumer ref.
func WithConsumerRef() HostOpts {
	return func(host *infrav2.HetznerBareMetalHost) {
		host.Spec.ConsumerRef = &infrav2.HetznerBareMetalHostConsumerReference{
			Name:     "bm-machine",
			Kind:     "HetznerBareMetalMachine",
			APIGroup: infrav2.GroupVersion.Group,
		}
	}
}

// BareMetalMachineSSHSpec returns the SSH spec for a HetznerBareMetalMachine that consumes a test
// host. The key names match the data of GetDefaultSSHSecret. The host reads the SSH spec live from
// the HetznerBareMetalMachine.
func BareMetalMachineSSHSpec(portAfterInstallImage int) infrav2.SSHSpec {
	return infrav2.SSHSpec{
		SecretRef: infrav2.SSHSecretRef{
			Name: defaultOSSSHKeyName,
			Key: infrav2.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
		},
		PortAfterInstallImage: portAfterInstallImage,
	}
}

// GetDefaultHetznerClusterSpec returns the default Hetzner cluster spec.
func GetDefaultHetznerClusterSpec() infrav2.HetznerClusterSpec {
	return infrav2.HetznerClusterSpec{
		ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
			Enabled:   true,
			Algorithm: "round_robin",
			ExtraServices: []infrav2.LoadBalancerServiceSpec{
				{
					DestinationPort: 8132,
					ListenPort:      8132,
					Protocol:        "tcp",
				},
				{
					DestinationPort: 8133,
					ListenPort:      8133,
					Protocol:        "tcp",
				},
			},
			Port:   6443,
			Region: "fsn1",
			Type:   "lb11",
		},
		ControlPlaneRegions: []infrav2.Region{"fsn1"},
		HCloudNetwork: infrav2.HCloudNetworkSpec{
			CIDRBlock:       "10.0.0.0/16",
			Enabled:         true,
			NetworkZone:     "eu-central",
			SubnetCIDRBlock: "10.0.0.0/24",
		},
		HCloudPlacementGroups: []infrav2.HCloudPlacementGroupSpec{
			{
				Name: defaultPlacementGroupName,
				Type: "spread",
			},
			{
				Name: "md-0",
				Type: "spread",
			},
		},
		HetznerSecret: infrav2.HetznerSecretRef{
			Key: infrav2.HetznerSecretKeyRef{
				HCloudToken:          "hcloud",
				HetznerRobotUser:     "robot-user",
				HetznerRobotPassword: "robot-password",
			},
			Name: "hetzner-secret",
		},
		SSHKeys: infrav2.HetznerSSHKeys{
			HCloud: []infrav2.SSHKey{
				{
					Name: "testsshkey",
				},
			},
			RescueSecretRef: infrav2.SSHSecretRef{
				Name: "rescue-ssh-secret",
				Key: infrav2.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
		},
	}
}

// GetDefaultHetznerClusterSpecV1Beta1 returns the default Hetzner cluster spec as v1beta1, for the
// tests that still build the v1beta1 type. It converts the v1beta2 spec so that there is one literal
// to change when the default changes.
func GetDefaultHetznerClusterSpecV1Beta1() infrav1.HetznerClusterSpec {
	v2 := GetDefaultHetznerClusterSpec()
	var v1 infrav1.HetznerClusterSpec
	if err := infrav1.Convert_v1beta2_HetznerClusterSpec_To_v1beta1_HetznerClusterSpec(&v2, &v1, nil); err != nil {
		panic(err)
	}
	return v1
}

// GetDefaultSSHSecret returns the default ssh secret given name and namespace.
func GetDefaultSSHSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"private-key": []byte(fmt.Sprintf("%s-private-key", name)),
			"sshkey-name": []byte("my-name"),
			"public-key":  []byte("my-public-key"),
		},
	}
}
