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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/controllers"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	defaultPodNamespace = "caph-system"
	timeout             = time.Second * 30
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

var (
	testEnv *helpers.TestEnvironment
	ctx     = ctrl.SetupSignalHandler()
	wg      sync.WaitGroup
)

var _ = BeforeSuite(func() {
	utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))

	testEnv = helpers.NewTestEnvironment()

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
