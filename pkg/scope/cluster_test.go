/*
Copyright 2024 The Kubernetes Authors.

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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func controlPlaneObjectMeta(namespace, name, clusterName string, annotated bool) metav1.ObjectMeta {
	annotations := map[string]string{}
	if annotated {
		annotations[infrav1.ProxyProtocolForControlPlaneLoadBalancerAnnotation] = "true"
	}

	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels: map[string]string{
			clusterv1.ClusterNameLabel:         clusterName,
			clusterv1.MachineControlPlaneLabel: "",
		},
		Annotations: annotations,
	}
}

func controlPlaneHCloudMachine(namespace, name, clusterName string, annotated bool) *infrav1.HCloudMachine {
	return &infrav1.HCloudMachine{
		ObjectMeta: controlPlaneObjectMeta(namespace, name, clusterName, annotated),
	}
}

func controlPlaneBareMetalMachine(namespace, name, clusterName string, annotated bool) *infrav1.HetznerBareMetalMachine {
	return &infrav1.HetznerBareMetalMachine{
		ObjectMeta: controlPlaneObjectMeta(namespace, name, clusterName, annotated),
	}
}

func TestAllControlPlaneInfraMachinesAnnotatedForProxyProtocol(t *testing.T) {
	const (
		namespace   = "default"
		clusterName = "test-cluster"
	)

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(infrav1.AddToScheme(scheme))

	tests := []struct {
		name     string
		machines []client.Object
		want     bool
	}{
		{
			name:     "no control-plane infrastructure machines yet",
			machines: nil,
			want:     false,
		},
		{
			name: "all hcloud control planes annotated",
			machines: []client.Object{
				controlPlaneHCloudMachine(namespace, "cp-1", clusterName, true),
				controlPlaneHCloudMachine(namespace, "cp-2", clusterName, true),
				controlPlaneHCloudMachine(namespace, "cp-3", clusterName, true),
			},
			want: true,
		},
		{
			name: "mixed hcloud and bare-metal control planes all annotated",
			machines: []client.Object{
				controlPlaneHCloudMachine(namespace, "cp-1", clusterName, true),
				controlPlaneBareMetalMachine(namespace, "cp-2", clusterName, true),
			},
			want: true,
		},
		{
			name: "one hcloud machine still from the old template misses the annotation",
			machines: []client.Object{
				controlPlaneHCloudMachine(namespace, "cp-1", clusterName, true),
				controlPlaneHCloudMachine(namespace, "cp-2", clusterName, false),
			},
			want: false,
		},
		{
			name: "one bare-metal machine still from the old template misses the annotation",
			machines: []client.Object{
				controlPlaneHCloudMachine(namespace, "cp-1", clusterName, true),
				controlPlaneBareMetalMachine(namespace, "cp-2", clusterName, false),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := append([]client.Object{}, tt.machines...)
			// A worker infrastructure machine of the same cluster (no control-plane label) must never
			// affect the result.
			objects = append(objects, &infrav1.HCloudMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-1",
					Namespace: namespace,
					Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
				},
			})

			s := &ClusterScope{
				Logger: klog.Background(),
				Client: fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
				},
				HetznerCluster: &infrav1.HetznerCluster{
					ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
				},
			}

			got, err := s.AllControlPlaneInfraMachinesAnnotatedForProxyProtocol(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
