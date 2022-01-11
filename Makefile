# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Ensure Make is run with bash shell as some syntax below is bash-specific
# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.DEFAULT_GOAL:=help

#
# Go.
#
GO_VERSION ?= 1.17.6
GO_CONTAINER_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

#
# Kubebuilder.
#
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.22.0
export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT ?= 60s
export KUBEBUILDER_CONTROLPLANE_STOP_TIMEOUT ?= 60s

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

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

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/sigs.k8s.io/cluster-api)
	CONVERSION_GEN_OUTPUT_BASE := --output-base=$(ROOT_DIR)
else
	export GOPATH := $(shell go env GOPATH)
endif

TIMEOUT := $(shell command -v timeout || command -v gtimeout)

# Define Docker related variables. Releases should modify and double check these vars.
# Image URL to use all building/pushing image targets
IMG ?= quay.io/syself/cluster-api-provider-hetzner:latest

#
# Container related variables. Releases should modify and double check these vars.
#
REGISTRY ?= quay.io/syself
STAGING_REGISTRY := quay.io/syself
PROD_REGISTRY := quay.io/syself
IMAGE_NAME ?= cluster-api-provider-hetzner
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= latest
ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always

# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

# Default path for Kubeconfig File.
CAPH_WORKER_CLUSTER_KUBECONFIG ?= "/tmp/workload-kubeconfig"


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



##@ Generate / Manifests

.PHONY: generate
generate: ## Run all generate-manifests, generate-go-deepcopyand generate-go-conversions targets
	$(MAKE) generate-manifests generate-go-deepcopy

generate-manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
			paths=./api/... \
			crd:crdVersions=v1 \
			rbac:roleName=manager-role \
			output:crd:dir=./config/crd/bases \
			output:webhook:dir=./config/webhook \
			webhook

generate-go-deepcopy: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) \
		object:headerFile="./hack/boilerplate/boilerplate.generatego.txt" \
		paths="./api/..."

dry-run: generate
	cd config/manager && kustomize edit set image controller=${CONTROLLER_IMG}:${TAG}
	mkdir -p dry-run
	kustomize build config/default > dry-run/manifests.yaml


##@ Lint and Verify

.PHONY: modules
modules: ## Runs go mod to ensure modules are up to date.
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

.PHONY: lint
lint: golang-ci-lint $(GOLANGCI_LINT) ## Lint Golang codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS)

.PHONY: lint-fix
lint-fix: golang-ci-lint $(GOLANGCI_LINT) ## Lint the Go codebase and run auto-fixers if supported by the linter.
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

.PHONY: format-tiltfile
format-tiltfile: ## Format the Tiltfile
	./hack/verify-starlark.sh fix

ALL_VERIFY_CHECKS = boilerplate shellcheck tiltfile modules gen

.PHONY: verify
verify: $(addprefix verify-,$(ALL_VERIFY_CHECKS)) lint ## Run all verify-* targets
	@echo "All verify checks passed, congrats!"


.PHONY: verify-modules
verify-modules: modules  ## Verify go modules are up to date
	@if !(git diff --quiet HEAD -- go.sum go.mod $(TOOLS_DIR)/go.mod $(TOOLS_DIR)/go.sum $(TEST_DIR)/go.mod $(TEST_DIR)/go.sum); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi
	@if (find . -name 'go.mod' | xargs -n1 grep -q -i 'k8s.io/client-go.*+incompatible'); then \
		find . -name "go.mod" -exec grep -i 'k8s.io/client-go.*+incompatible' {} \; -print; \
		echo "go module contains an incompatible client-go version"; exit 1; \
	fi

.PHONY: verify-gen
verify-gen: generate  ## Verfiy go generated files are up to date
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: verify-boilerplate
verify-boilerplate: ## Verify boilerplate text exists in each file
	./hack/verify-boilerplate.sh

.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files
	./hack/verify-shellcheck.sh

.PHONY: verify-tiltfile
verify-tiltfile: ## Verify Tiltfile format
	./hack/verify-starlark.sh


##@ Clean

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

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_image_patch.yaml ./*manager_pull_policy.yaml

##@ Release

## latest git tag for the commit, e.g., v0.3.10
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
# the previous release tag, e.g., v0.3.9, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]." | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)
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
	## Build the manifests
	$(MAKE) release-manifests clean-release-git

.PHONY: release-manifests
release-manifests: generate kustomize $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	## Build caph-components (aggregate of all of the above).
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp templates/cluster-template* $(RELEASE_DIR)/

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

ARTIFACTS ?= ${ROOT_DIR}/_artifacts

KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))

.PHONY: test
test: $(SETUP_ENVTEST) ## Run unit and integration tests
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test ./... $(TEST_ARGS)

.PHONY: test-e2e
test-e2e: ## Run the e2e tests
	$(MAKE) -C $(TEST_DIR)/e2e run

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run: generate fmt vet ## Run a controller from your host.
	go run ./main.go

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-build
docker-build: ## Build the docker image for controller-manager
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(CONTROLLER_IMG):$(TAG)

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



##@ Development

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

install-crds: generate-manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall-crds: generate-manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy-controller: generate-manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}:${TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

.PHONY: tilt-up
tilt-up: envsubst yq kustomize cluster  ## Start a mgt-cluster & Tilt. Installs the CRDs and deploys the controllers
	EXP_CLUSTER_RESOURCE_SET=true tilt up

.PHONY: create-workload-cluster
create-workload-cluster-with-network-packer: kustomize envsubst ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	$(ENVSUBST) -i templates/cluster-template-packer-hcloud-network.yaml | kubectl apply -f -
	
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

	# Deploy HCloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.2 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=true

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

create-workload-cluster-with-network: kustomize envsubst ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	$(ENVSUBST) -i templates/cluster-template-hcloud-network.yaml | kubectl apply -f -
	
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

	# Deploy HCloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.2 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=true

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'


create-workload-cluster-packer: kustomize envsubst ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	$(ENVSUBST) -i templates/cluster-template-packer.yaml | kubectl apply -f -
	
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

	# Deploy HCloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.2 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=false

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

create-workload-cluster: kustomize envsubst ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	$(ENVSUBST) -i templates/cluster-template.yaml | kubectl apply -f -
	
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

	# Deploy HCloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.2 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=false

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'


move-to-workload-cluster:
	clusterctl init --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
	kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) -n cluster-api-provider-hetzner-system wait deploy/caph-controller-manager --for condition=available && sleep 15s
	clusterctl move --to-kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG)

create-talos-workload-cluster-packer: kustomize envsubst ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	$(ENVSUBST) -i templates/cluster-template-packer-talos.yaml | kubectl apply -f -
	
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

	# Deploy HCloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.2 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=false

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

.PHONY: delete-workload-cluster
delete-workload-cluster: ## Deletes the example workload Kubernetes cluster
	@echo 'Your Hetzner resources will now be deleted, this can take up to 20 minutes'
	kubectl delete cluster $(CLUSTER_NAME)
	kubectl patch cluster $(CLUSTER_NAME) --type=merge -p '{"spec":{"paused": "false"}}'

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