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
	"k8s.io/utils/ptr"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func validHetznerCluster(lb infrav1.LoadBalancerSpec) *infrav1.HetznerCluster {
	return &infrav1.HetznerCluster{
		Spec: infrav1.HetznerClusterSpec{
			ControlPlaneLoadBalancer: lb,
			HetznerSecret: infrav1.HetznerSecretRef{
				Key: infrav1.HetznerSecretKeyRef{
					HCloudToken: "token",
				},
			},
		},
	}
}

func TestValidateUpdateProxyProtocol(t *testing.T) {
	webhook := &HetznerClusterWebhook{}

	tests := []struct {
		name        string
		oldLB       infrav1.LoadBalancerSpec
		newLB       infrav1.LoadBalancerSpec
		expectError bool
	}{
		{
			name:        "disabling proxy protocol is not allowed",
			oldLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: true},
			newLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: false},
			expectError: true,
		},
		{
			name:        "enabling proxy protocol is allowed",
			oldLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: false},
			newLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: true},
			expectError: false,
		},
		{
			name:        "keeping proxy protocol enabled is allowed",
			oldLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: true},
			newLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: true},
			expectError: false,
		},
		{
			name:        "keeping proxy protocol disabled is allowed",
			oldLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: false},
			newLB:       infrav1.LoadBalancerSpec{EnableProxyProtocol: false},
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

func TestValidateLoadBalancerHealthCheck(t *testing.T) {
	webhook := &HetznerClusterWebhook{}

	tests := []struct {
		name        string
		healthCheck *infrav1.LoadBalancerHealthCheckSpec
		expectError bool
	}{
		{name: "no health check is allowed", healthCheck: nil},
		{name: "tcp health check without path/domain is allowed", healthCheck: &infrav1.LoadBalancerHealthCheckSpec{Type: "tcp"}},
		{name: "http health check with path is allowed", healthCheck: &infrav1.LoadBalancerHealthCheckSpec{Type: "http", Path: ptr.To("/readyz")}},
		{name: "https health check with domain is allowed", healthCheck: &infrav1.LoadBalancerHealthCheckSpec{Type: "https", Domain: ptr.To("example.com")}},
		{
			name:        "tcp health check with path is rejected",
			healthCheck: &infrav1.LoadBalancerHealthCheckSpec{Type: "tcp", Path: ptr.To("/readyz")},
			expectError: true,
		},
		{
			name:        "tcp health check with domain is rejected",
			healthCheck: &infrav1.LoadBalancerHealthCheckSpec{Type: "tcp", Domain: ptr.To("example.com")},
			expectError: true,
		},
		{
			name:        "unset type with path is treated as tcp and rejected",
			healthCheck: &infrav1.LoadBalancerHealthCheckSpec{Path: ptr.To("/readyz")},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldCluster := validHetznerCluster(infrav1.LoadBalancerSpec{})
			newCluster := validHetznerCluster(infrav1.LoadBalancerSpec{HealthCheck: tt.healthCheck})
			_, err := webhook.ValidateUpdate(context.Background(), oldCluster, newCluster)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must not be set when type is tcp")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
