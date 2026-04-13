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

package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func Test_shouldSkipReconciliationForNamespace(t *testing.T) {
	testCases := []struct {
		name          string
		namespace     string
		nsAnnotations map[string]string
		createNS      bool
		wantSkip      bool
	}{
		{
			name:      "request has no namespace",
			namespace: "",
			wantSkip:  false,
		},
		{
			name:      "namespace does not exist",
			namespace: "missing",
			wantSkip:  false,
		},
		{
			name:      "namespace exists without annotation",
			namespace: "default",
			createNS:  true,
			wantSkip:  false,
		},
		{
			name:      "namespace exists with skip annotation set to true",
			namespace: "default",
			createNS:  true,
			nsAnnotations: map[string]string{
				infrav1.SkipNamespaceAnnotation: "true",
			},
			wantSkip: true,
		},
		{
			name:      "namespace exists with skip annotation set to false",
			namespace: "default",
			createNS:  true,
			nsAnnotations: map[string]string{
				infrav1.SkipNamespaceAnnotation: "false",
			},
			wantSkip: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("add corev1 scheme: %v", err)
			}

			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			if tc.createNS {
				builder = builder.WithObjects(&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:        tc.namespace,
						Annotations: tc.nsAnnotations,
					},
				})
			}

			c := builder.Build()

			gotSkip, err := shouldSkipReconciliationForNamespace(context.Background(), c, tc.namespace)
			if err != nil {
				t.Fatalf("shouldSkipReconciliationForNamespace returned error: %v", err)
			}
			if gotSkip != tc.wantSkip {
				t.Fatalf("shouldSkipReconciliationForNamespace(%q) = %v, want %v", tc.namespace, gotSkip, tc.wantSkip)
			}
		})
	}
}
