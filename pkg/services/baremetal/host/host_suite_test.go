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

package host

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2/textlogger"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

const (
	sshFingerprint   = "my-fingerprint"
	osSSHKeyName     = "os-sshkey"
	rescueSSHKeyName = "rescue-sshkey"
)

type timeoutError struct {
	error
}

func (timeoutError) Timeout() bool {
	return true
}

func (timeoutError) Error() string {
	return "timeout"
}

func TestHost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Host Suite")
}

var (
	log        = textlogger.NewLogger(textlogger.NewConfig())
	errTimeout = fmt.Errorf("timeout")
	timeout    = timeoutError{errTimeout}
)

func newTestHostStateMachine(host *infrav1.HetznerBareMetalHost, service *Service) *hostStateMachine {
	return newHostStateMachine(host, service, log)
}

var fakeBootID = "1234321"

func newTestService(
	host *infrav1.HetznerBareMetalHost,
	robotClient robotclient.Client,
	sshClientFactory sshclient.Factory,
	osSSHSecret *corev1.Secret,
	rescueSSHSecret *corev1.Secret,
) *Service {
	scheme := runtime.NewScheme()
	utilruntime.Must(infrav1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(host).Build()
	ctx := context.Background()

	capiMachine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      host.Name,
			Namespace: host.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		Status: clusterv1.MachineStatus{
			NodeRef: &corev1.ObjectReference{
				Kind:       "Node",
				Name:       host.Name,
				APIVersion: "v1",
			},
		},
	}
	err := c.Create(ctx, capiMachine)
	if err != nil {
		panic(err)
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: host.Name,
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				BootID: fakeBootID,
			},
		},
	}
	err = c.Create(ctx, node)
	if err != nil {
		panic(err)
	}

	hbmm := &infrav1.HetznerBareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      host.Name,
			Namespace: host.Namespace,
		},
	}

	hbmm.OwnerReferences = append(hbmm.OwnerReferences, metav1.OwnerReference{
		APIVersion: capiMachine.APIVersion,
		Kind:       capiMachine.Kind,
		Name:       capiMachine.Name,
		UID:        capiMachine.UID,
	})

	err = c.Create(ctx, hbmm)
	if err != nil {
		panic(err)
	}

	return &Service{
		&scope.BareMetalHostScope{
			Logger:                  log,
			Client:                  c,
			SecretManager:           secretutil.NewSecretManager(log, c, c),
			SSHClientFactory:        sshClientFactory,
			RobotClient:             robotClient,
			HetznerBareMetalHost:    host,
			HetznerBareMetalMachine: hbmm,
			HetznerCluster: &infrav1.HetznerCluster{
				Spec: helpers.GetDefaultHetznerClusterSpec(),
			},
			// Attention: this doesn't make sense if we test with constant node names
			Cluster:              &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}},
			OSSSHSecret:          osSSHSecret,
			RescueSSHSecret:      rescueSSHSecret,
			SSHAfterInstallImage: true,
			WorkloadClusterClientFactory: &fakeWorkloadClusterClientFactory{
				client: c,
			},
			ImageURLCommand: "image-url-command",
		},
	}
}

type fakeWorkloadClusterClientFactory struct {
	client client.Client
}

func (f *fakeWorkloadClusterClientFactory) NewWorkloadClient(_ context.Context) (client.Client, error) {
	return f.client, nil
}
