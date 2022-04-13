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

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	bareMetalHostID         = 1
	sshFingerprint          = "my-fingerprint"
	defaultOSSSHKeyName     = "os-sshkey"
	defaultRescueSSHKeyName = "rescue-sshkey"

	// DefaultWWN specifies the default WWN.
	DefaultWWN = "eui.002538b411b2cee8"
)

var (
	defaultPlacementGroupName = "caph-placement-group"
)

// BareMetalHost returns a bare metal host given options.
func BareMetalHost(name, namespace string, opts ...HostOpts) *infrav1.HetznerBareMetalHost {
	host := &infrav1.HetznerBareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrav1.HetznerBareMetalHostSpec{
			ServerID: bareMetalHostID,
		},
	}
	for _, o := range opts {
		o(host)
	}
	return host
}

// HostOpts define options to customize the host spec.
type HostOpts func(*infrav1.HetznerBareMetalHost)

// WithError gives the option to define a host with error in spec.status.
func WithError(errorType infrav1.ErrorType, errorMessage string, errorCount int, lastUpdated metav1.Time) HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.ErrorType = errorType
		host.Spec.Status.ErrorMessage = errorMessage
		host.Spec.Status.ErrorCount = errorCount
		host.Spec.Status.LastUpdated = &lastUpdated
	}
}

// WithRebootTypes gives the option to define a host with custom reboot types.
func WithRebootTypes(rebootTypes []infrav1.RebootType) HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.RebootTypes = rebootTypes
	}
}

// WithRootDeviceHints gives the option to define a host with root device hints.
func WithRootDeviceHints() HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.RootDeviceHints = &infrav1.RootDeviceHints{
			WWN: DefaultWWN,
		}
	}
}

// WithHetznerClusterRef gives the option to define a host with cluster ref.
func WithHetznerClusterRef(hetznerClusterRef string) HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.HetznerClusterRef = hetznerClusterRef
	}
}

// WithSSHSpec gives the option to define a host with ssh spec.
func WithSSHSpec() HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.SSHSpec = &infrav1.SSHSpec{
			SecretRef: infrav1.SSHSecretRef{
				Name: defaultOSSSHKeyName,
				Key: infrav1.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
		}
	}
}

// WithSSHSpecInclPorts gives the option to define a host with ssh spec incl. ports.
func WithSSHSpecInclPorts(portAfterInstallImage, portAfterCloudInit int) HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.SSHSpec = &infrav1.SSHSpec{
			SecretRef: infrav1.SSHSecretRef{
				Name: defaultOSSSHKeyName,
				Key: infrav1.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
			PortAfterInstallImage: portAfterInstallImage,
			PortAfterCloudInit:    portAfterCloudInit,
		}
	}
}

// WithSSHStatus gives the option to define a host with ssh status.
func WithSSHStatus() HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.SSHStatus = infrav1.SSHStatus{
			OSKey: &infrav1.SSHKey{
				Name:        defaultOSSSHKeyName,
				Fingerprint: sshFingerprint,
			},
			RescueKey: &infrav1.SSHKey{
				Name:        defaultRescueSSHKeyName,
				Fingerprint: sshFingerprint,
			},
		}
	}
}

// WithIPv4 gives the option to define a host with IP.
func WithIPv4() HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.Status.IPv4 = "1.2.3.4"
	}
}

// WithConsumerRef gives the option to define a host with consumer ref.
func WithConsumerRef() HostOpts {
	return func(host *infrav1.HetznerBareMetalHost) {
		host.Spec.ConsumerRef = &corev1.ObjectReference{
			Name:      "bm-machine",
			Namespace: "default",
			Kind:      "HetznerBareMetalMachine",
		}
	}
}

// GetDefaultHetznerClusterSpec returns the default Hetzner cluster spec.
func GetDefaultHetznerClusterSpec() infrav1.HetznerClusterSpec {
	return infrav1.HetznerClusterSpec{
		ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
			Algorithm: "round_robin",
			ExtraServices: []infrav1.LoadBalancerServiceSpec{
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
		ControlPlaneEndpoint: &clusterv1.APIEndpoint{},
		ControlPlaneRegions:  []infrav1.Region{"fsn1"},
		HCloudNetwork: infrav1.HCloudNetworkSpec{
			CIDRBlock:       "10.0.0.0/16",
			Enabled:         true,
			NetworkZone:     "eu-central",
			SubnetCIDRBlock: "10.0.0.0/24",
		},
		HCloudPlacementGroup: []infrav1.HCloudPlacementGroupSpec{
			{
				Name: defaultPlacementGroupName,
				Type: "spread",
			},
			{
				Name: "md-0",
				Type: "spread",
			},
		},
		HetznerSecret: infrav1.HetznerSecretRef{
			Key: infrav1.HetznerSecretKeyRef{
				HCloudToken:          "hcloud",
				HetznerRobotUser:     "robot-user",
				HetznerRobotPassword: "robot-password",
			},
			Name: "hetzner-secret",
		},
		SSHKeys: infrav1.HetznerSSHKeys{
			HCloud: []infrav1.SSHKey{
				{
					Name: "testsshkey",
				},
			},
			RobotRescueSecretRef: infrav1.SSHSecretRef{
				Name: "rescue-ssh-secret",
				Key: infrav1.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
		},
	}
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
