package loadbalancer

import (
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"k8s.io/utils/ptr"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func TestHealthCheckDiffers(t *testing.T) {
	const servicePort = 6443
	tests := []struct {
		name     string
		observed hcloud.LoadBalancerServiceHealthCheck
		desired  *infrav1.LoadBalancerServiceHealthCheck
		want     bool
	}{
		{
			name:    "nil desired never differs",
			desired: nil,
			want:    false,
		},
		{
			name:     "matching https readyz check",
			observed: hcloud.LoadBalancerServiceHealthCheck{Protocol: "https", Port: servicePort, HTTP: &hcloud.LoadBalancerServiceHealthCheckHTTP{Path: "/readyz", TLS: true}},
			desired:  &infrav1.LoadBalancerServiceHealthCheck{Protocol: "https", Path: "/readyz"},
			want:     false,
		},
		{
			name:     "protocol change from tcp to https",
			observed: hcloud.LoadBalancerServiceHealthCheck{Protocol: "tcp", Port: servicePort},
			desired:  &infrav1.LoadBalancerServiceHealthCheck{Protocol: "https", Path: "/readyz"},
			want:     true,
		},
		{
			name:     "path change",
			observed: hcloud.LoadBalancerServiceHealthCheck{Protocol: "https", Port: servicePort, HTTP: &hcloud.LoadBalancerServiceHealthCheckHTTP{Path: "/healthz", TLS: true}},
			desired:  &infrav1.LoadBalancerServiceHealthCheck{Protocol: "https", Path: "/readyz"},
			want:     true,
		},
		{
			name:     "tls flips when https desired but observed is plain http",
			observed: hcloud.LoadBalancerServiceHealthCheck{Protocol: "https", Port: servicePort, HTTP: &hcloud.LoadBalancerServiceHealthCheckHTTP{Path: "/readyz", TLS: false}},
			desired:  &infrav1.LoadBalancerServiceHealthCheck{Protocol: "https", Path: "/readyz"},
			want:     true,
		},
		{
			name:     "custom port differs from default",
			observed: hcloud.LoadBalancerServiceHealthCheck{Protocol: "tcp", Port: servicePort},
			desired:  &infrav1.LoadBalancerServiceHealthCheck{Protocol: "tcp", Port: ptr.To(8443)},
			want:     true,
		},
		{
			name:     "interval differs",
			observed: hcloud.LoadBalancerServiceHealthCheck{Protocol: "tcp", Port: servicePort, Interval: 15 * time.Second},
			desired:  &infrav1.LoadBalancerServiceHealthCheck{Protocol: "tcp", IntervalSeconds: ptr.To(5)},
			want:     true,
		},
		{
			name:     "status codes differ",
			observed: hcloud.LoadBalancerServiceHealthCheck{Protocol: "https", Port: servicePort, HTTP: &hcloud.LoadBalancerServiceHealthCheckHTTP{Path: "/readyz", TLS: true, StatusCodes: []string{"2xx"}}},
			desired:  &infrav1.LoadBalancerServiceHealthCheck{Protocol: "https", Path: "/readyz", StatusCodes: []string{"200"}},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthCheckDiffers(tt.observed, tt.desired, servicePort); got != tt.want {
				t.Errorf("healthCheckDiffers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthCheckAddOpts(t *testing.T) {
	if got := healthCheckAddOpts(nil, 6443); got != nil {
		t.Fatalf("healthCheckAddOpts(nil) = %v, want nil so the service keeps the default TCP check", got)
	}

	hc := &infrav1.LoadBalancerServiceHealthCheck{
		Protocol:    "https",
		Path:        "/readyz",
		StatusCodes: []string{"200"},
		Retries:     ptr.To(2),
	}
	got := healthCheckAddOpts(hc, 6443)
	if got == nil {
		t.Fatal("healthCheckAddOpts() = nil, want opts")
	}
	if string(got.Protocol) != "https" {
		t.Errorf("Protocol = %q, want https", got.Protocol)
	}
	if got.Port == nil || *got.Port != 6443 {
		t.Errorf("Port = %v, want the service port 6443", got.Port)
	}
	if got.Retries == nil || *got.Retries != 2 {
		t.Errorf("Retries = %v, want 2", got.Retries)
	}
	if got.HTTP == nil || got.HTTP.Path == nil || *got.HTTP.Path != "/readyz" {
		t.Fatalf("HTTP path = %v, want /readyz", got.HTTP)
	}
	if got.HTTP.TLS == nil || !*got.HTTP.TLS {
		t.Errorf("HTTP.TLS = %v, want true for https", got.HTTP.TLS)
	}
}

func TestHealthCheckAddOptsTCPHasNoHTTP(t *testing.T) {
	got := healthCheckAddOpts(&infrav1.LoadBalancerServiceHealthCheck{Protocol: "tcp"}, 6443)
	if got == nil {
		t.Fatal("healthCheckAddOpts() = nil, want opts")
	}
	if got.HTTP != nil {
		t.Errorf("HTTP = %v, want nil for a tcp check", got.HTTP)
	}
}
