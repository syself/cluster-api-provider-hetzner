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

package scope

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var _ = Describe("BareMetalHostScope", func() {
	type testCaseHostname struct {
		clusterAnnotations map[string]string
		machineAnnotations map[string]string
		expectedHostname   string
	}

	DescribeTable("Hostname",
		func(tc testCaseHostname) {
			hostScope := BareMetalHostScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-cluster",
						Annotations: tc.clusterAnnotations,
					},
				},
				HetznerBareMetalHost: &infrav1.HetznerBareMetalHost{
					Spec: infrav1.HetznerBareMetalHostSpec{
						ServerID: 42,
						ConsumerRef: &corev1.ObjectReference{
							Name: "worker-0",
						},
					},
				},
				HetznerBareMetalMachine: &infrav1.HetznerBareMetalMachine{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: tc.machineAnnotations,
					},
				},
			}

			Expect(hostScope.Hostname()).To(Equal(tc.expectedHostname))
		},
		Entry("uses a constant hostname when enabled on the cluster", testCaseHostname{
			clusterAnnotations: map[string]string{
				infrav1.ConstantBareMetalHostnameAnnotation: "true",
			},
			expectedHostname: "bm-test-cluster-42",
		}),
		Entry("uses a constant hostname when enabled on the machine", testCaseHostname{
			machineAnnotations: map[string]string{
				infrav1.ConstantBareMetalHostnameAnnotation: "true",
			},
			expectedHostname: "bm-test-cluster-42",
		}),
		Entry("does not use a constant hostname for control-plane machines", testCaseHostname{
			clusterAnnotations: map[string]string{
				infrav1.ConstantBareMetalHostnameAnnotation: "true",
			},
			machineAnnotations: map[string]string{
				clusterv1.MachineControlPlaneLabel: "true",
			},
			expectedHostname: "bm-worker-0",
		}),
	)
})
