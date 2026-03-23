package provisioncheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHostsFromHBMHYAMLFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		wantNames []string
	}{
		{
			name: "multi document",
			content: `apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerBareMetalHost
metadata:
  name: alpha
spec:
  rootDeviceHints:
    wwn: "0x1"
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerBareMetalHost
metadata:
  name: beta
spec:
  rootDeviceHints:
    wwn: "0x2"
`,
			wantNames: []string{"alpha", "beta"},
		},
		{
			name: "top level items list",
			content: `apiVersion: v1
items:
- apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: HetznerBareMetalHost
  metadata:
    name: alpha
  spec:
    rootDeviceHints:
      wwn: "0x1"
- apiVersion: v1
  kind: Secret
  metadata:
    name: ignored
- apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: HetznerBareMetalHost
  metadata:
    name: beta
  spec:
    rootDeviceHints:
      wwn: "0x2"
`,
			wantNames: []string{"alpha", "beta"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "hosts.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write test yaml: %v", err)
			}

			hosts, err := loadHostsFromHBMHYAMLFile(path)
			if err != nil {
				t.Fatalf("loadHostsFromHBMHYAMLFile() error = %v", err)
			}
			if len(hosts) != len(tt.wantNames) {
				t.Fatalf("len(hosts) = %d, want %d", len(hosts), len(tt.wantNames))
			}
			for i, wantName := range tt.wantNames {
				if hosts[i].Name != wantName {
					t.Fatalf("hosts[%d].Name = %q, want %q", i, hosts[i].Name, wantName)
				}
			}
		})
	}
}
