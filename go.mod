module github.com/syself/cluster-api-provider-hetzner

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/hetznercloud/hcloud-go v1.32.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.26.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/apiserver v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/cluster-api v1.0.0
	sigs.k8s.io/controller-runtime v0.10.2
)