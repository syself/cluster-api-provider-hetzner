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

package controllers_test

import (
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/controllers"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

const (
	defaultPodNamespace = "caph-system"
	timeout             = time.Second * 60
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var (
	testEnv      *helpers.TestEnvironment
	hcloudClient hcloudclient.Client
	ctx          = ctrl.SetupSignalHandler()
	wg           sync.WaitGroup
)

var _ = BeforeSuite(func() {
	utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))

	testEnv = helpers.NewTestEnvironment()
	hcloudClient = testEnv.HCloudClientFactory.NewClient("")

	wg.Add(1)

	Expect((&controllers.HetznerClusterReconciler{
		Client:                         testEnv.Manager.GetClient(),
		HCloudClientFactory:            testEnv.HCloudClientFactory,
		WatchFilterValue:               "",
		TargetClusterManagersWaitGroup: &wg,
	}).SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	Expect((&controllers.HCloudMachineReconciler{
		Client:              testEnv.Manager.GetClient(),
		HCloudClientFactory: testEnv.HCloudClientFactory,
		WatchFilterValue:    "",
	}).SetupWithManager(ctx, testEnv.Manager, controller.Options{})).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(testEnv.StartManager(ctx)).To(Succeed())
	}()

	<-testEnv.Manager.Elected()

	// wait for webhook port to be open prior to running tests
	testEnv.WaitForWebhooks()

	// create manager pod namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPodNamespace,
		},
	}

	Expect(testEnv.Create(ctx, ns)).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
	wg.Done() // Main manager has been stopped
	wg.Wait() // Wait for target cluster manager
})

func getDefaultHetznerClusterSpec() infrav1.HetznerClusterSpec {
	return infrav1.HetznerClusterSpec{
		ControlPlaneLoadBalancer: infrav1.LoadBalancerSpec{
			Algorithm: "round_robin",
			ExtraTargets: []infrav1.LoadBalancerTargetSpec{
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
				Name: "control-plane",
				Type: "spread",
			},
			{
				Name: "md-0",
				Type: "spread",
			},
		},
		HetznerSecret: infrav1.HetznerSecretRef{
			Key: infrav1.HetznerSecretKeyRef{
				HCloudToken: "hcloud",
			},
			Name: "hetzner-secret",
		},
		SSHKeys: infrav1.HetznerSSHKeys{
			HCloud: []infrav1.SSHKey{
				{
					Name: "testsshkey",
				},
			},
		},
	}
}

func getDefaultHetznerSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hetzner-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"hcloud": []byte("my-token"),
		},
	}
}

func getDefaultBootstrapSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value": []byte("my-bootstrap"),
		},
	}
}
