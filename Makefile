
#
# Directories.
#
# Full directory of where the Makefile resides
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
EXP_DIR := exp
BIN_DIR := bin
TEST_DIR := test
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
GO_APIDIFF_BIN := $(BIN_DIR)/go-apidiff
GO_APIDIFF := $(TOOLS_DIR)/$(GO_APIDIFF_BIN)
ENVSUBST_BIN := $(BIN_DIR)/envsubst
ENVSUBST := $(TOOLS_DIR)/$(ENVSUBST_BIN)

export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

TIMEOUT := $(shell command -v timeout || command -v gtimeout)

# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

# Define Docker related variables. Releases should modify and double check these vars.
# Image URL to use all building/pushing image targets
IMG ?= quay.io/syself/cluster-api-provider-hetzner:latest


REGISTRY ?= quay.io/syself
STAGING_REGISTRY := quay.io/syself
PROD_REGISTRY := quay.io/syself
IMAGE_NAME ?= cluster-api-provider-hetzner
export CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
export TAG ?= latest
export ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x
# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always


# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

CAPH_WORKER_CLUSTER_KUBECONFIG ?= "/tmp/workload-kubeconfig"

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec


#
# Go.
#
GO_VERSION ?= 1.16.8
GO_CONTAINER_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/sigs.k8s.io/cluster-api)
	CONVERSION_GEN_OUTPUT_BASE := --output-base=$(ROOT_DIR)
else
	export GOPATH := $(shell go env GOPATH)
endif

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: help

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Binaries / Software

.PHONY: install-ctlptl
install-ctlptl: ## Installs CTLPTL (CLI for declaratively setting up local Kubernetes clusters)
	./hack/ensure-ctlptl.sh

.PHONY: install-helm
install-helm: ## Installs Helm (Kubernetes package manager)
	./hack/ensure-helm.sh

.PHONY: check-go
check-go: ## Checks go version
	./hack/ensure-go.sh

.PHONY: install-helmfile
install-helmfile: ## Installs Helmfile (Helmfile is like a helm for your helm)
	./hack/ensure-helmfile.sh

.PHONY: install-packer
install-packer: ## Installs Hashicorp Packer
	./hack/ensure-packer.sh

.PHONY: install-hcloud
install-hcloud: ## Installs hcloud (CLI for Hetzner)
	./hack/ensure-hcloud.sh

.PHONY: install-helm-plugins
install-helm-plugins: ## Installs Helm Plugins (helm-git)
	./hack/ensure-helm-plugins.sh

install-kind: ## Installs Kind (Kubernetes-in-Docker)
	./hack/ensure-kind.sh

.PHONY: install-kubectl
install-kubectl: ## Installs Kubectl (CLI for kubernetes)
	./hack/ensure-kubectl.sh

.PHONY: install-tilt
install-tilt: ## Installs Tilt (watches files, builds containers, ships to k8s)
	./hack/ensure-tilt.sh

.PHONY: install-clusterctl
install-clusterctl: ## Installs clusterctl
	./hack/ensure-clusterctl.sh

install-dev-prerequisites:
	@echo "Start checking dependencies"
	$(MAKE) install-ctlptl
	$(MAKE) install-helm
	$(MAKE) check-go
	$(MAKE) install-helmfile
	$(MAKE) install-helm-plugins
	$(MAKE) install-kind
	$(MAKE) install-kubectl
	$(MAKE) install-tilt
	$(MAKE) install-clusterctl
	$(MAKE) install-packer
	$(MAKE) install-hcloud
	@echo "Finished: All dependencies up to date"

CONTROLLER_GEN = $(shell pwd)/hack/tools/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

KUSTOMIZE = $(shell pwd)/hack/tools/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

ENVSUBST = $(shell pwd)/hack/tools/bin/envsubst 
envsubst: ## Download envsubst locally if neccessary.
	$(call go-get-tool,$(ENVSUBST),github.com/a8m/envsubst/cmd/envsubst@v1.2.0)

GOLANGCI_LINT = $(shell pwd)/hack/tools/bin/golangci-lint
golang-ci-lint: ## Download golang-ci-lint locally if neccessary.
	$(call go-get-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@v1.43.0)

HELMIFY = $(shell pwd)/hack/tools/bin/helmify
helmify: ## Download helmify locally if necessary.
	$(call go-get-tool,$(HELMIFY),github.com/arttor/helmify/cmd/helmify@v0.3.3)

YQ = $(shell pwd)/hack/tools/bin/yq
yq: ## Download yq locally if necessary.
	$(call go-get-tool,$(YQ),github.com/mikefarah/yq/v4@v4.13.5)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/hack/tools/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef



##@ Generate

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

helm: manifests kustomize helmify
	$(KUSTOMIZE) build config/default | $(HELMIFY) manifests/charts/cluster-api-provider-hetzner

dry-run: manifests
	cd config/manager && kustomize edit set image controller=${CONTROLLER_IMG}:${TAG}
	mkdir -p dry-run
	kustomize build config/default > dry-run/manifests.yaml


##@ Cleanup / Verification

.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin

.PHONY: clean-bin
clean-bin: ## Remove all generated helper binaries
	rm -rf $(BIN_DIR)
	rm -rf $(TOOLS_BIN_DIR)

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-docker-all
clean-docker-all: ## Erases all container and images
	./hack/erase-docker-all.sh

.PHONY: verify
verify: verify-modules verify-gen tiltfile-lint shell-lint ## Run all verifications

.PHONY: verify-gen
verify-gen: generate
	@if !(git diff --quiet HEAD); then \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi


##@ Release
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)

RELEASE_TAG := $(shell git describe --abbrev=0 2>/dev/null)
RELEASE_DIR ?= out
RELEASE_NOTES_DIR := _releasenotes

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests
	$(MAKE) release-metadata
	$(MAKE) release-templates

.PHONY: release-manifests
release-manifests: manifests $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

.PHONY: release-templates
release-templates: $(RELEASE_DIR)
	cp templates/cluster-template* $(RELEASE_DIR)/

.PHONY: release-metadata
release-metadata: $(RELEASE_DIR)
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

.PHONY: release-nightly
release-nightly: ## Builds and push container images to the prod bucket.
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(PROD_REGISTRY)/$(IMAGE_NAME):$(TAG)
	docker push $(PROD_REGISTRY)/$(IMAGE_NAME):$(TAG)

.PHONY: release-image
release-image: ## Builds and push container images to the prod bucket.
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(PROD_REGISTRY)/$(IMAGE_NAME):$(RELEASE_TAG)
	docker push $(PROD_REGISTRY)/$(IMAGE_NAME):$(RELEASE_TAG)

##@ Test

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out




##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-build
docker-build: ## Build the docker image for controller-manager
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(CONTROLLER_IMG):$(TAG)
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH))

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))
	$(MAKE) docker-push-manifest

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_image_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml


##@ Linting

.PHONY: go-lint
go-lint: golang-ci-lint $(GOLANGCI_LINT) ## Lint Golang codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS)

.PHONY: go-lint-fix
go-lint-fix: golang-ci-lint $(GOLANGCI_LINT) ## Lint the Go codebase and run auto-fixers if supported by the linter.
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

.PHONY: tiltfile-lint
tiltfile-lint: ## Lint Tiltfile
	./hack/verify-starlark.sh

.PHONY: tiltfile-fix
tiltfile-fix: ## Format Tiltfile
	./hack/verify-starlark.sh fix

.PHONY: shell-lint
shell-lint: ## Run all verifications
	./hack/verify-shellcheck.sh


##@ Development

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

.PHONY: modules
modules: ## Runs go mod to ensure modules are up to date.
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

install-crds: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall-crds: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy-controller: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}:${TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

.PHONY: tilt-up
tilt-up: $(ENVSUBST) $(KUSTOMIZE) $(KUBECTL) cluster ## Start a mgt-cluster & Tilt. Installs the CRDs and deploys the controllers
	EXP_CLUSTER_RESOURCE_SET=true tilt up

.PHONY: create-workload-cluster
create-workload-cluster: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	$(KUSTOMIZE) build templates | $(ENVSUBST) | kubectl apply -f -
	
	# Wait for the kubeconfig to become available.
	${TIMEOUT} 5m bash -c "while ! kubectl get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	kubectl get secrets $(CLUSTER_NAME)-kubeconfig -o json | jq -r .data.value | base64 --decode > $(CAPH_WORKER_CLUSTER_KUBECONFIG)
	${TIMEOUT} 15m bash -c "while ! kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) get nodes | grep master; do sleep 1; done"

	# Deploy cilium
	helm repo add cilium https://helm.cilium.io/
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install cilium cilium/cilium --version 1.10.5 \
  	--namespace kube-system \
	-f templates/cilium/cilium.yaml

	# Deploy Hcloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.0 \
	--namespace kube-system \
	--set secret.name=hetzner-token

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

create-talos-workload-cluster: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	$(ENVSUBST) -i templates/cluster-template-talos.yaml | kubectl apply -f -
	
	# Wait for the kubeconfig to become available.
	${TIMEOUT} 5m bash -c "while ! kubectl get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	kubectl get secrets $(CLUSTER_NAME)-kubeconfig -o json | jq -r .data.value | base64 --decode > $(CAPH_WORKER_CLUSTER_KUBECONFIG)
	${TIMEOUT} 15m bash -c "while ! kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) get nodes | grep master; do sleep 1; done"

	# Deploy cilium
	helm repo add cilium https://helm.cilium.io/
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install cilium cilium/cilium --version 1.10.5 \
  	--namespace kube-system \
	-f templates/cilium/cilium.yaml

	# Deploy Hcloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.0 \
	--namespace kube-system \
	--set secret.name=hetzner-token

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

.PHONY: delete-workload-cluster
delete-workload-cluster: ## Deletes the example workload Kubernetes cluster
	@echo 'Your Hetzner resources will now be deleted, this can take up to 20 minutes'
	kubectl delete cluster $(CLUSTER_NAME)


##@ Management Cluster

.PHONY: cluster
cluster: ## Creates kind-dev Cluster
	./hack/kind-dev.sh

.PHONY: delete-cluster
delete-cluster: ## Deletes Kind-dev Cluster (default)
	ctlptl delete cluster kind-caph

.PHONY: delete-registry
delete-registry: ## Deletes Kind-dev Cluster and the local registry
	ctlptl delete registry caph-registry

.PHONY: delete-cluster-registry
delete-cluster-registry: ## Deletes Kind-dev Cluster and the local registry
	ctlptl delete cluster kind-caph
	ctlptl delete registry caph-registry


