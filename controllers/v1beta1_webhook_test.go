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

package controllers

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// The webhooks are served for v1beta2 only. The API server converts a v1beta1 request to v1beta2
// before it calls them, so these tests send v1beta1 objects and check the v1beta2 webhook answers.
var _ = Describe("Admission webhooks for v1beta1 requests", func() {
	var testNs *corev1.Namespace

	BeforeEach(func() {
		var err error
		var finish func()
		testNs, finish, err = testEnv.ResetAndCreateNamespace(ctx, "v1beta1-webhook")
		defer finish()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs)).To(Succeed())
	})

	It("rejects an HCloudMachine without imageName", func() {
		hcloudMachine := &infrav1beta1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "hcloud-machine", Namespace: testNs.Name},
			Spec:       infrav1beta1.HCloudMachineSpec{Type: "cpx32"},
		}
		err := testEnv.Create(ctx, hcloudMachine)
		expectDeniedBy(err, "validation.hcloudmachine.infrastructure.cluster.x-k8s.io", "imageName and customProvisioner empty")
	})

	It("rejects a change to the spec of an HCloudMachineTemplate", func() {
		hcloudMachineTemplate := &infrav1beta1.HCloudMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "hcloud-machine-template", Namespace: testNs.Name},
			Spec: infrav1beta1.HCloudMachineTemplateSpec{
				Template: infrav1beta1.HCloudMachineTemplateResource{
					Spec: infrav1beta1.HCloudMachineSpec{Type: "cpx32", ImageName: "my-image"},
				},
			},
		}
		Expect(testEnv.Create(ctx, hcloudMachineTemplate)).To(Succeed())

		patch := client.MergeFrom(hcloudMachineTemplate.DeepCopy())
		hcloudMachineTemplate.Labels = map[string]string{"test": "label"}
		Expect(testEnv.Patch(ctx, hcloudMachineTemplate, patch)).To(Succeed())

		patch = client.MergeFrom(hcloudMachineTemplate.DeepCopy())
		hcloudMachineTemplate.Spec.Template.Spec.Type = "cpx42"
		err := testEnv.Patch(ctx, hcloudMachineTemplate, patch)
		expectDeniedBy(err, "validation.hcloudmachinetemplate.infrastructure.x-k8s.io", "HCloudMachineTemplate.Spec is immutable")
	})

	It("accepts an HCloudRemediation", func() {
		hcloudRemediation := &infrav1beta1.HCloudRemediation{
			ObjectMeta: metav1.ObjectMeta{Name: "hcloud-remediation", Namespace: testNs.Name},
		}
		Expect(testEnv.Create(ctx, hcloudRemediation)).To(Succeed())
	})

	It("accepts an HCloudRemediationTemplate", func() {
		hcloudRemediationTemplate := &infrav1beta1.HCloudRemediationTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "hcloud-remediation-template", Namespace: testNs.Name},
		}
		Expect(testEnv.Create(ctx, hcloudRemediationTemplate)).To(Succeed())
	})

	It("rejects a change to the serverID of a HetznerBareMetalHost", func() {
		host := &infrav1beta1.HetznerBareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "host", Namespace: testNs.Name},
			Spec:       infrav1beta1.HetznerBareMetalHostSpec{ServerID: 23181},
		}
		Expect(testEnv.Create(ctx, host)).To(Succeed())

		patch := client.MergeFrom(host.DeepCopy())
		host.Spec.ServerID = 23182
		err := testEnv.Patch(ctx, host, patch)
		expectDeniedBy(err, "validation.hetznerbaremetalhost.infrastructure.cluster.x-k8s.io", "serverID is immutable")
	})

	It("rejects a HetznerBareMetalMachine without an image", func() {
		bmMachine := &infrav1beta1.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-machine", Namespace: testNs.Name},
			Spec:       v1beta1HetznerBareMetalMachineSpec(),
		}
		bmMachine.Spec.InstallImage.Image = infrav1beta1.Image{}
		err := testEnv.Create(ctx, bmMachine)
		expectDeniedBy(err, "validation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io", "have to specify either image name and url or path")
	})

	It("rejects a change to the spec of a HetznerBareMetalMachineTemplate", func() {
		bmMachineTemplate := &infrav1beta1.HetznerBareMetalMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-machine-template", Namespace: testNs.Name},
			Spec: infrav1beta1.HetznerBareMetalMachineTemplateSpec{
				Template: infrav1beta1.HetznerBareMetalMachineTemplateResource{
					Spec: v1beta1HetznerBareMetalMachineSpec(),
				},
			},
		}
		Expect(testEnv.Create(ctx, bmMachineTemplate)).To(Succeed())

		patch := client.MergeFrom(bmMachineTemplate.DeepCopy())
		bmMachineTemplate.Labels = map[string]string{"test": "label"}
		Expect(testEnv.Patch(ctx, bmMachineTemplate, patch)).To(Succeed())

		patch = client.MergeFrom(bmMachineTemplate.DeepCopy())
		bmMachineTemplate.Spec.Template.Spec.InstallImage.Image.Name = "other-image"
		err := testEnv.Patch(ctx, bmMachineTemplate, patch)
		expectDeniedBy(err, "validation.hetznerbaremetalmachinetemplate.infrastructure.cluster.x-k8s.io", "HetznerBareMetalMachineTemplate.Spec is immutable")
	})

	It("accepts a HetznerBareMetalRemediation", func() {
		bmRemediation := &infrav1beta1.HetznerBareMetalRemediation{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-remediation", Namespace: testNs.Name},
		}
		Expect(testEnv.Create(ctx, bmRemediation)).To(Succeed())
	})

	It("accepts a HetznerBareMetalRemediationTemplate", func() {
		bmRemediationTemplate := &infrav1beta1.HetznerBareMetalRemediationTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "bm-remediation-template", Namespace: testNs.Name},
		}
		Expect(testEnv.Create(ctx, bmRemediationTemplate)).To(Succeed())
	})

	It("rejects a HetznerCluster with control plane regions in two network zones", func() {
		hetznerCluster := &infrav1beta1.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "hetzner-cluster", Namespace: testNs.Name},
			Spec:       v1beta1HetznerClusterSpec(),
		}
		hetznerCluster.Spec.ControlPlaneRegions = []infrav1beta1.Region{"fsn1", "ash"}
		err := testEnv.Create(ctx, hetznerCluster)
		expectDeniedBy(err, "validation.hetznercluster.infrastructure.cluster.x-k8s.io", "regions are not in one network zone")
	})

	It("rejects a change to the spec of a HetznerClusterTemplate", func() {
		hetznerClusterTemplate := &infrav1beta1.HetznerClusterTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "hetzner-cluster-template", Namespace: testNs.Name},
			Spec: infrav1beta1.HetznerClusterTemplateSpec{
				Template: infrav1beta1.HetznerClusterTemplateResource{
					Spec: v1beta1HetznerClusterSpec(),
				},
			},
		}
		Expect(testEnv.Create(ctx, hetznerClusterTemplate)).To(Succeed())

		patch := client.MergeFrom(hetznerClusterTemplate.DeepCopy())
		hetznerClusterTemplate.Labels = map[string]string{"test": "label"}
		Expect(testEnv.Patch(ctx, hetznerClusterTemplate, patch)).To(Succeed())

		patch = client.MergeFrom(hetznerClusterTemplate.DeepCopy())
		hetznerClusterTemplate.Spec.Template.Spec.ControlPlaneRegions = []infrav1beta1.Region{"nbg1"}
		err := testEnv.Patch(ctx, hetznerClusterTemplate, patch)
		expectDeniedBy(err, "validation.hetznerclustertemplate.infrastructure.cluster.x-k8s.io", "HetznerClusterTemplate.Spec is immutable")
	})
})

func expectDeniedBy(err error, webhookName, message string) {
	GinkgoHelper()
	Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("admission webhook %q denied the request", webhookName))))
	Expect(err).To(MatchError(ContainSubstring(message)))
}

func v1beta1HetznerBareMetalMachineSpec() infrav1beta1.HetznerBareMetalMachineSpec {
	return infrav1beta1.HetznerBareMetalMachineSpec{
		InstallImage: infrav1beta1.InstallImage{
			Image: infrav1beta1.Image{
				Name: "image-name",
				URL:  "https://myfile.tar.gz",
			},
			Partitions: []infrav1beta1.Partition{
				{Mount: "lvm", FileSystem: "ext2", Size: "1G"},
			},
		},
		SSHSpec: infrav1beta1.SSHSpec{
			SecretRef: infrav1beta1.SSHSecretRef{
				Name: "os-ssh-secret",
				Key: infrav1beta1.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
			PortAfterInstallImage: 22,
		},
	}
}

func v1beta1HetznerClusterSpec() infrav1beta1.HetznerClusterSpec {
	return infrav1beta1.HetznerClusterSpec{
		ControlPlaneRegions: []infrav1beta1.Region{"fsn1"},
		ControlPlaneLoadBalancer: infrav1beta1.LoadBalancerSpec{
			Enabled: true,
			Region:  "fsn1",
		},
		HetznerSecret: infrav1beta1.HetznerSecretRef{
			Name: "hetzner-secret",
			Key: infrav1beta1.HetznerSecretKeyRef{
				HCloudToken:          "hcloud",
				HetznerRobotUser:     "robot-user",
				HetznerRobotPassword: "robot-password",
			},
		},
	}
}
