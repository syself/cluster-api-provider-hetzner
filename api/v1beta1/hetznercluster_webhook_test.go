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

package v1beta1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validHetznerCluster(lb LoadBalancerSpec) *HetznerCluster {
	return &HetznerCluster{
		Spec: HetznerClusterSpec{
			ControlPlaneLoadBalancer: lb,
			HetznerSecret: HetznerSecretRef{
				Key: HetznerSecretKeyRef{
					HCloudToken: "token",
				},
			},
		},
	}
}

func TestValidateUpdateProxyProtocol(t *testing.T) {
	webhook := &hetznerClusterWebhook{}

	tests := []struct {
		name        string
		oldLB       LoadBalancerSpec
		newLB       LoadBalancerSpec
		expectError bool
	}{
		{
			name:        "disabling proxy protocol is not allowed",
			oldLB:       LoadBalancerSpec{EnableProxyProtocol: true},
			newLB:       LoadBalancerSpec{EnableProxyProtocol: false},
			expectError: true,
		},
		{
			name:        "enabling proxy protocol is allowed",
			oldLB:       LoadBalancerSpec{EnableProxyProtocol: false},
			newLB:       LoadBalancerSpec{EnableProxyProtocol: true},
			expectError: false,
		},
		{
			name:        "keeping proxy protocol enabled is allowed",
			oldLB:       LoadBalancerSpec{EnableProxyProtocol: true},
			newLB:       LoadBalancerSpec{EnableProxyProtocol: true},
			expectError: false,
		},
		{
			name:        "keeping proxy protocol disabled is allowed",
			oldLB:       LoadBalancerSpec{EnableProxyProtocol: false},
			newLB:       LoadBalancerSpec{EnableProxyProtocol: false},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := webhook.ValidateUpdate(context.Background(), validHetznerCluster(tt.oldLB), validHetznerCluster(tt.newLB))
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "proxy protocol cannot be disabled once enabled")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
