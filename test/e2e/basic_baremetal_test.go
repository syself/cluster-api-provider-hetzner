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

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func logBareMetalHostStatusContinously(ctx context.Context, c client.Client) {
	caphDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "caph-controller-manager",
			Namespace: "caph-system",
		},
	}
	err := c.Get(ctx, client.ObjectKeyFromObject(&caphDeployment), &caphDeployment)
	if err != nil {
		By(fmt.Sprintf("Error getting caph-controller-manager deployment: %v", err))
		return
	}
	By(fmt.Sprintf("caph-controller-manager image: %v", caphDeployment.Spec.Template.Spec.Containers[0].Image))
	for {
		t := time.After(30 * time.Second)
		select {
		case <-ctx.Done():
			return
		case <-t:
			err := logBareMetalHostStatus(ctx, c)
			if err != nil {
				By(fmt.Sprintf("Error logging BareMetalHost status: %v", err))
			}
		}
	}
}

func logBareMetalHostStatus(ctx context.Context, c client.Client) error {
	hbmhList := &infrav1.HetznerBareMetalHostList{}
	err := c.List(ctx, hbmhList)
	if err != nil {
		return err
	}
	for i := range hbmhList.Items {
		hbmh := &hbmhList.Items[i]
		if hbmh.Spec.Status.ProvisioningState == "" {
			continue
		}
		By("BareMetalHost: " + hbmh.Name + " " + fmt.Sprint(hbmh.Spec.ServerID))
		By("  ProvisioningState: " + string(hbmh.Spec.Status.ProvisioningState))
		readyC := conditions.Get(hbmh, clusterv1.ReadyCondition)
		msg := ""
		reason := ""
		state := "?"
		if readyC != nil {
			msg = readyC.Message
			reason = readyC.Reason
			state = string(readyC.Status)
		}
		By("  Ready Condition: " + state + " " + reason + " " + msg)
	}
	By("---------------------------------------------------")
	return nil
}

var _ = Describe("[Baremetal] Testing Cluster 1x control-planes 1x worker ", func() {
	ctx := context.TODO()

	Context("Running the CaphClusterDeploymentSpec in Hetzner Baremetal", func() {
		CaphClusterDeploymentSpec(ctx, func() CaphClusterDeploymentSpecInput {
			go logBareMetalHostStatusContinously(ctx, bootstrapClusterProxy.GetClient())
			return CaphClusterDeploymentSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				ControlPlaneMachineCount: 1,
				WorkerMachineCount:       1,
				Flavor:                   "hetzner-baremetal",
			}
		})
	})
})
