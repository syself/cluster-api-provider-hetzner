module github.com/syself/cluster-api-provider-hetzner/hack/tools

go 1.16

require (
	github.com/drone/envsubst/v2 v2.0.0-20210615175204-7bf45dbf5372
	github.com/joelanford/go-apidiff v0.1.0
	github.com/mikefarah/yq/v4 v4.14.1
	github.com/onsi/ginkgo v1.16.4
	github.com/sergi/go-diff v1.2.0 // indirect
	golang.org/x/exp v0.0.0-20210625193404-fa9d1d177d71 // indirect
	gotest.tools/gotestsum v1.6.4
	k8s.io/code-generator v0.22.2
	sigs.k8s.io/controller-runtime/tools/setup-envtest v0.0.0-20210827150604-1730628f118b
	sigs.k8s.io/controller-tools v0.7.0
)
