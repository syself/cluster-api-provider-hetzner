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
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2/textlogger"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
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

func newTestService(
	host *infrav1.HetznerBareMetalHost,
	robotClient robotclient.Client,
	sshClientFactory sshclient.Factory,
	osSSHSecret *corev1.Secret,
	rescueSSHSecret *corev1.Secret,
) *Service {
	scheme := runtime.NewScheme()
	utilruntime.Must(infrav1.AddToScheme(scheme))
	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(host).Build()
	return &Service{
		&scope.BareMetalHostScope{
			Logger:               log,
			Client:               c,
			SSHClientFactory:     sshClientFactory,
			RobotClient:          robotClient,
			HetznerBareMetalHost: host,
			HetznerCluster: &infrav1.HetznerCluster{
				Spec: helpers.GetDefaultHetznerClusterSpec(),
			},
			OSSSHSecret:     osSSHSecret,
			RescueSSHSecret: rescueSSHSecret,
		},
	}
}
