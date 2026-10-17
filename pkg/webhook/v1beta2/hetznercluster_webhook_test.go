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

package v1beta2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

func validHetznerCluster(lb infrav2.LoadBalancerSpec) *infrav2.HetznerCluster {
	return &infrav2.HetznerCluster{
		Spec: infrav2.HetznerClusterSpec{
			ControlPlaneLoadBalancer: lb,
			HetznerSecret: infrav2.HetznerSecretRef{
				Key: infrav2.HetznerSecretKeyRef{
					HCloudToken: ptr.To("token"),
				},
			},
		},
	}
}

func TestValidateUpdateProxyProtocol(t *testing.T) {
	webhook := &HetznerClusterWebhook{}

	tests := []struct {
		name        string
		oldLB       infrav2.LoadBalancerSpec
		newLB       infrav2.LoadBalancerSpec
		expectError bool
	}{
		{
			name:        "disabling proxy protocol is not allowed",
			oldLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(true)},
			newLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(false)},
			expectError: true,
		},
		{
			name:        "enabling proxy protocol is allowed",
			oldLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(false)},
			newLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(true)},
			expectError: false,
		},
		{
			name:        "keeping proxy protocol enabled is allowed",
			oldLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(true)},
			newLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(true)},
			expectError: false,
		},
		{
			name:        "keeping proxy protocol disabled is allowed",
			oldLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(false)},
			newLB:       infrav2.LoadBalancerSpec{EnableProxyProtocol: ptr.To(false)},
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
		healthCheck *infrav2.LoadBalancerHealthCheckSpec
		expectError bool
	}{
		{name: "no health check is allowed", healthCheck: nil},
		{name: "tcp health check without path/domain is allowed", healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Protocol: "tcp"}},
		{name: "http health check with path is allowed", healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Protocol: "http", Path: ptr.To("/readyz")}},
		{name: "https health check with domain is allowed", healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Protocol: "https", Domain: ptr.To("example.com")}},
		{
			name:        "tcp health check with path is rejected",
			healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Protocol: "tcp", Path: ptr.To("/readyz")},
			expectError: true,
		},
		{
			name:        "tcp health check with domain is rejected",
			healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Protocol: "tcp", Domain: ptr.To("example.com")},
			expectError: true,
		},
		{
			name:        "tcp health check with response is rejected",
			healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Protocol: "tcp", Response: ptr.To("ok")},
			expectError: true,
		},
		{
			name:        "tcp health check with statusCodes is rejected",
			healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Protocol: "tcp", StatusCodes: []string{"200"}},
			expectError: true,
		},
		{
			name:        "unset protocol with path is treated as tcp and rejected",
			healthCheck: &infrav2.LoadBalancerHealthCheckSpec{Path: ptr.To("/readyz")},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldCluster := validHetznerCluster(infrav2.LoadBalancerSpec{})
			newCluster := validHetznerCluster(infrav2.LoadBalancerSpec{HealthCheck: tt.healthCheck})
			_, err := webhook.ValidateUpdate(context.Background(), oldCluster, newCluster)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must not be set when protocol is tcp")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateUpdateHCloudNetworkUnsetEnabled(t *testing.T) {
	webhook := &HetznerClusterWebhook{}
	cluster := func(enabled *bool) *infrav2.HetznerCluster {
		c := validHetznerCluster(infrav2.LoadBalancerSpec{})
		c.Spec.HCloudNetwork.Enabled = enabled
		return c
	}

	// An object read from storage has enabled false. An update request that leaves enabled out means false too.
	_, err := webhook.ValidateUpdate(context.Background(), cluster(ptr.To(false)), cluster(nil))
	require.NoError(t, err)

	_, err = webhook.ValidateUpdate(context.Background(), cluster(ptr.To(false)), cluster(ptr.To(true)))
	require.ErrorContains(t, err, "spec.hcloudNetwork: Invalid value")
}

// TestValidateUpdateLoadBalancerEnabled checks that controlPlaneLoadBalancer.enabled is immutable,
// and that an unset enabled means false.
func TestValidateUpdateLoadBalancerEnabled(t *testing.T) {
	webhook := &HetznerClusterWebhook{}

	tests := []struct {
		name    string
		oldLB   infrav2.LoadBalancerSpec
		newLB   infrav2.LoadBalancerSpec
		wantErr bool
	}{
		{
			name:  "unset enabled after false is accepted",
			oldLB: infrav2.LoadBalancerSpec{Enabled: ptr.To(false)},
			newLB: infrav2.LoadBalancerSpec{},
		},
		{
			name:  "false after unset enabled is accepted",
			oldLB: infrav2.LoadBalancerSpec{},
			newLB: infrav2.LoadBalancerSpec{Enabled: ptr.To(false)},
		},
		{
			name:  "true after true is accepted",
			oldLB: infrav2.LoadBalancerSpec{Enabled: ptr.To(true)},
			newLB: infrav2.LoadBalancerSpec{Enabled: ptr.To(true)},
		},
		{
			name:    "true after false is rejected",
			oldLB:   infrav2.LoadBalancerSpec{Enabled: ptr.To(false)},
			newLB:   infrav2.LoadBalancerSpec{Enabled: ptr.To(true)},
			wantErr: true,
		},
		{
			name:    "unset enabled after true is rejected",
			oldLB:   infrav2.LoadBalancerSpec{Enabled: ptr.To(true)},
			newLB:   infrav2.LoadBalancerSpec{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := webhook.ValidateUpdate(context.Background(), validHetznerCluster(tt.oldLB), validHetznerCluster(tt.newLB))
			if tt.wantErr {
				require.ErrorContains(t, err, "spec.controlPlaneLoadBalancer.enabled: Invalid value")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHetznerClusterTemplateValidateUpdateUnsetFields(t *testing.T) {
	template := func(spec infrav2.HetznerClusterSpec) *infrav2.HetznerClusterTemplate {
		return &infrav2.HetznerClusterTemplate{
			Spec: infrav2.HetznerClusterTemplateSpec{
				Template: infrav2.HetznerClusterTemplateResource{Spec: spec},
			},
		}
	}
	// stored is a spec as it is read from storage: the conversion from v1beta1 sets every field that
	// has a meaning when unset. A controlPlaneLoadBalancer or hcloudNetwork that was left out stays a
	// zero struct.
	stored := infrav2.HetznerClusterSpec{
		HetznerSecret: infrav2.HetznerSecretRef{
			Name: "hetzner",
			Key: infrav2.HetznerSecretKeyRef{
				HCloudToken:          ptr.To("token"),
				HetznerRobotUser:     ptr.To("hetzner-robot-user"),
				HetznerRobotPassword: ptr.To("hetzner-robot-password"),
				SSHKey:               ptr.To("hcloud-ssh-key-name"),
			},
		},
		SkipCreatingHetznerSecretInWorkloadCluster: ptr.To(false),
	}
	unset := infrav2.HetznerClusterSpec{
		SSHKeys:       infrav2.HetznerSSHKeys{RescueSecretRef: &infrav2.SSHSecretRef{}},
		HetznerSecret: infrav2.HetznerSecretRef{Name: "hetzner"},
	}
	webhook := &HetznerClusterTemplateWebhook{}

	_, err := webhook.ValidateUpdate(context.Background(), template(stored), template(unset))
	require.NoError(t, err)

	changed := unset
	changed.SkipCreatingHetznerSecretInWorkloadCluster = ptr.To(true)
	_, err = webhook.ValidateUpdate(context.Background(), template(stored), template(changed))
	require.ErrorContains(t, err, "HetznerClusterTemplate.Spec is immutable")
}
