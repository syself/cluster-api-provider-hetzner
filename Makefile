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
# Directories.
#
# Full directory of where the Makefile resides
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
EXP_DIR := exp
BIN_DIR := bin
TEST_DIR := test
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)
# Default path for Kubeconfig File.
CAPH_WORKER_CLUSTER_KUBECONFIG ?= "/tmp/workload-kubeconfig"

INTEGRATION_CONF_FILE ?= "$(abspath test/integration/integration-dev.yaml)"
E2E_TEMPLATE_DIR := "$(abspath test/e2e/data/infrastructure-hetzner/)"
ARTIFACTS_PATH := $(ROOT_DIR)/_artifacts
CI_KIND ?= true
#
# Binaries.
#
MINIMUM_CLUSTERCTL_VERSION=1.1.2				# https://github.com/kubernetes-sigs/cluster-api/releases
MINIMUM_CTLPTL_VERSION=0.7.4						# https://github.com/tilt-dev/ctlptl/releases
MINIMUM_GO_VERSION=go$(GO_VERSION)			# Check current project go version
MINIMUM_HCLOUD_VERSION=1.29.0						# https://github.com/hetznercloud/cli/releases
MINIMUM_HELMFILE_VERSION=v0.143.0				# https://github.com/roboll/helmfile/releases
MINIMUM_KIND_VERSION=v0.11.1						# https://github.com/kubernetes-sigs/kind/releases
MINIMUM_KUBECTL_VERSION=v1.23.0					# https://github.com/kubernetes/kubernetes/releases
MINIMUM_PACKER_VERSION=1.7.10						# https://github.com/hashicorp/packer/releases
MINIMUM_TILT_VERSION=0.25.3							# https://github.com/tilt-dev/tilt/releases
CONTROLLER_GEN_VERSION=v.0.4.1					# https://github.com/kubernetes-sigs/controller-tools/releases
KUSTOMIZE_VERSION=4.5.1									# https://github.com/kubernetes-sigs/kustomize/releases

#
# Tooling Binaries.
#
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)
KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/kustomize)
GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/golangci-lint)
CONVERSION_GEN := $(abspath $(TOOLS_BIN_DIR)/conversion-gen)
ENVSUBST_BIN := $(BIN_DIR)/envsubst
ENVSUBST := $(TOOLS_DIR)/$(ENVSUBST_BIN)
SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/setup-envtest)
GINKGO := $(TOOLS_BIN_DIR)/ginkgo
GO_APIDIFF_BIN := $(BIN_DIR)/go-apidiff
GO_APIDIFF := $(TOOLS_DIR)/$(GO_APIDIFF_BIN)
TIMEOUT := $(shell command -v timeout || command -v gtimeout)
KIND := $(TOOLS_BIN_DIR)/kind

#
# HELM.
#
MINIMUM_HELM_VERSION=v3.8.0							# https://github.com/helm/helm/releases
HELM_GIT_VERSION=0.11.1									# https://github.com/aslafy-z/helm-git/releases
HELM_DIFF_VERSION=3.4.1									# https://github.com/databus23/helm-diff/releases

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
# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/sigs.k8s.io/cluster-api)
	CONVERSION_GEN_OUTPUT_BASE := --output-base=$(ROOT_DIR)
else
	export GOPATH := $(shell go env GOPATH)
endif

#
# Kubebuilder.
#
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.23.3
export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT ?= 60s
export KUBEBUILDER_CONTROLPLANE_STOP_TIMEOUT ?= 60s


#
# Container related variables. Releases should modify and double check these vars.
#
REGISTRY ?= quay.io/syself
PROD_REGISTRY := quay.io/syself
IMAGE_NAME ?= cluster-api-provider-hetzner
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= dev
ARCH ?= amd64
# Modify these according to your needs
PLATFORMS  = linux/amd64,linux/arm64
# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always
# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)
# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled


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
	MINIMUM_CTLPTL_VERSION=$(MINIMUM_CTLPTL_VERSION) ./hack/ensure-ctlptl.sh

.PHONY: install-helm
install-helm: ## Installs Helm (Kubernetes package manager)
	MINIMUM_HELM_VERSION=$(MINIMUM_HELM_VERSION) ./hack/ensure-helm.sh

.PHONY: check-go
check-go: ## Checks go version
	MINIMUM_GO_VERSION=$(MINIMUM_GO_VERSION) ./hack/ensure-go.sh

.PHONY: install-helmfile
install-helmfile: ## Installs Helmfile (Helmfile is like a helm for your helm)
	MINIMUM_HELMFILE_VERSION=$(MINIMUM_HELMFILE_VERSION) ./hack/ensure-helmfile.sh

.PHONY: install-packer
install-packer: ## Installs Hashicorp Packer
	MINIMUM_PACKER_VERSION=$(MINIMUM_PACKER_VERSION) ./hack/ensure-packer.sh

.PHONY: install-hcloud
install-hcloud: ## Installs hcloud (CLI for Hetzner)
	MINIMUM_HCLOUD_VERSION=$(MINIMUM_HCLOUD_VERSION) ./hack/ensure-hcloud.sh

.PHONY: install-helm-plugins
install-helm-plugins: ## Installs Helm Plugins (helm-git)
	HELM_GIT_VERSION=$(HELM_GIT_VERSION) HELM_DIFF_VERSION=$(HELM_DIFF_VERSION) ./hack/ensure-helm-plugins.sh

install-kind: ## Installs Kind (Kubernetes-in-Docker)
	MINIMUM_KIND_VERSION=$(MINIMUM_KIND_VERSION) ./hack/ensure-kind.sh

.PHONY: install-kubectl
install-kubectl: ## Installs Kubectl (CLI for kubernetes)
	MINIMUM_KUBECTL_VERSION=$(MINIMUM_KUBECTL_VERSION) ./hack/ensure-kubectl.sh

.PHONY: install-tilt
install-tilt: ## Installs Tilt (watches files, builds containers, ships to k8s)
	MINIMUM_TILT_VERSION=$(MINIMUM_TILT_VERSION) ./hack/ensure-tilt.sh

.PHONY: install-clusterctl
install-clusterctl: ## Installs clusterctl
	MINIMUM_CLUSTERCTL_VERSION=$(MINIMUM_CLUSTERCTL_VERSION) ./hack/ensure-clusterctl.sh

install-dev-prerequisites: ## Installs ctlptl, helm, helmfile, helm-plugins, kind, kubectl, tilt, clusterctl, packer, hcloud and checks installed go version
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

controller-gen: $(CONTROLLER_GEN) ## Build a local copy of controller-gen
$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

conversion-gen: $(CONVERSION_GEN) ## Build a local copy of conversion-gen
$(CONVERSION_GEN): $(TOOLS_DIR)/go.mod # Build conversion-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/conversion-gen k8s.io/code-generator/cmd/conversion-gen

conversion-verifier: $(CONVERSION_VERIFIER) ## Build a local copy of conversion-verifier
$(CONVERSION_VERIFIER): $(TOOLS_DIR)/go.mod # Build conversion-verifier from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/conversion-verifier sigs.k8s.io/cluster-api/hack/tools/conversion-verifier

go-apidiff: $(GO_APIDIFF) ## Build a local copy of apidiff
$(GO_APIDIFF): $(TOOLS_DIR)/go.mod # Build go-apidiff from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o $(GO_APIDIFF_BIN) github.com/joelanford/go-apidiff

envsubst: $(ENVSUBST) ## Build a local copy of envsubst
$(ENVSUBST): $(TOOLS_DIR)/go.mod # Build envsubst from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o $(ENVSUBST_BIN) github.com/drone/envsubst/v2/cmd/envsubst

kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize
$(KUSTOMIZE): # Download kustomize using hack script into tools folder.
	KUSTOMIZE_VERSION=$(KUSTOMIZE_VERSION) hack/ensure-kustomize.sh

golangci-lint: $(GOLANGCI_LINT) ## Build a local copy of golangci-lint
$(GOLANGCI_LINT): .github/workflows/golangci-lint.yml # Download golanci-lint using hack script into tools folder.
	hack/ensure-golangci-lint.sh \
		-b $(TOOLS_DIR)/$(BIN_DIR) \
		$(shell cat .github/workflows/golangci-lint.yml | grep version | sed 's/.*version: //')

setup-envtest: $(SETUP_ENVTEST) ## Build a local copy of setup-envtest
$(SETUP_ENVTEST): $(TOOLS_DIR)/go.mod # Build setup-envtest from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/setup-envtest sigs.k8s.io/controller-runtime/tools/setup-envtest

##@ Generate / Manifests

.PHONY: generate
generate: ## Run all generate-manifests, generate-go-deepcopyand generate-go-conversions targets
	$(MAKE) generate-manifests generate-go-deepcopy

generate-manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
			paths=./api/... \
			paths=./controllers/... \
			crd:crdVersions=v1 \
			rbac:roleName=manager-role \
			output:crd:dir=./config/crd/bases \
			output:webhook:dir=./config/webhook \
			webhook

generate-go-deepcopy: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) \
		object:headerFile="./hack/boilerplate/boilerplate.generatego.txt" \
		paths="./api/..."

dry-run: generate
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}:${TAG}
	mkdir -p dry-run
	$(KUSTOMIZE) build config/default > dry-run/manifests.yaml

.PHONY: ensure-boilerplate
ensure-boilerplate: ## Ensures that a boilerplate exists in each file by adding missing boilerplates
	./hack/ensure-boilerplate.sh


##@ Lint and Verify

.PHONY: modules
modules: ## Runs go mod to ensure modules are up to date.
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint Golang codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS)

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Lint the Go codebase and run auto-fixers if supported by the linter.
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

.PHONY: format-tiltfile
format-tiltfile: ## Format the Tiltfile
	./hack/verify-starlark.sh fix

yamllint: ## Lints YAML Files
	yamllint -c .github/linters/yaml-lint.yaml --strict .

ALL_VERIFY_CHECKS = boilerplate shellcheck tiltfile modules gen

.PHONY: verify
verify: lint $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Run all verify-* targets
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

.PHONY: test-release
test-release:
	$(MAKE) set-manifest-image MANIFEST_IMG=$(REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests

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
release-manifests: generate $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	## Build caph-components (aggregate of all of the above).
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp templates/cluster-template* $(RELEASE_DIR)/

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

.PHONY: release-nightly
release-nightly: ## Builds and push container images to the prod bucket.
	$(MAKE) CONTROLLER_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) TAG=latest docker-multiarch

.PHONY: release-image
release-image:  ## Builds and push container images to the prod bucket.
	$(MAKE) CONTROLLER_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) TAG=$(RELEASE_TAG) docker-multiarch

##@ Test

ARTIFACTS ?= _artifacts
$(ARTIFACTS):
	mkdir -p $(ARTIFACTS)/

KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))
REPO_ROOT := $(shell git rev-parse --show-toplevel)

E2E_DIR ?= $(REPO_ROOT)/test/e2e
E2E_CONF_FILE_SOURCE ?= $(E2E_DIR)/config/hetzner.yaml
E2E_CONF_FILE ?= $(E2E_DIR)/config/hetzner-ci-envsubst.yaml

.PHONY: e2e-image
e2e-image: ## Build the e2e manager image
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(CONTROLLER_IMG):e2e

.PHONY: $(E2E_CONF_FILE)
$(E2E_CONF_FILE): $(ENVSUBST) $(E2E_CONF_FILE_SOURCE)
	mkdir -p $(shell dirname $(E2E_CONF_FILE))
	$(ENVSUBST) < $(E2E_CONF_FILE_SOURCE) > $(E2E_CONF_FILE)

.PHONY: test-e2e
test-e2e: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	./hack/ci-e2e-capi.sh

.PHONY: test
test: $(SETUP_ENVTEST) ## Run unit and integration tests
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test ./controllers/... ./pkg/... $(TEST_ARGS)

.PHONY: test-verbose
test-verbose: ## Run tests with verbose settings
	$(MAKE) test TEST_ARGS="$(TEST_ARGS) -v"

.PHONY: test-cover
test-cover: $(RELEASE_DIR) ## Run tests with code coverage and code generate reports
	$(MAKE) test TEST_ARGS="$(TEST_ARGS) -coverprofile=out/coverage.out -covermode=atomic"
	go tool cover -func=out/coverage.out -o $(RELEASE_DIR)/coverage.txt
	go tool cover -html=out/coverage.out -o $(RELEASE_DIR)/coverage.html

.PHONY: test-junit
test-junit: $(SETUP_ENVTEST) $(GOTESTSUM) ## Run tests with verbose setting and generate a junit report
	set +o errexit; (KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test -json ./... $(TEST_ARGS); echo $$? > $(ARTIFACTS)/junit.exitcode) | tee $(ARTIFACTS)/junit.stdout
	$(GOTESTSUM) --junitfile $(ARTIFACTS)/junit.xml --raw-command cat $(ARTIFACTS)/junit.stdout
	exit $$(cat $(ARTIFACTS)/junit.exitcode)

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run: generate fmt vet ## Run a controller from your host.
	go run ./main.go

## --------------------------------------
## Docker
## --------------------------------------

# Create multi-platform docker image. If you have native systems around, using
# them will be much more efficient at build time. See e.g.
BUILDXDETECT = ${HOME}/.docker/cli-plugins/docker-buildx
# Just one of the many files created
QEMUDETECT = /proc/sys/fs/binfmt_misc/qemu-m68k

docker-multiarch: qemu buildx docker-multiarch-builder
	docker buildx build --builder docker-multiarch --pull --push \
		--platform ${PLATFORMS} \
		-t $(CONTROLLER_IMG):$(TAG) .

.PHONY: qemu buildx docker-multiarch-builder

qemu:	${QEMUDETECT}
${QEMUDETECT}:
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes

buildx: ${BUILDXDETECT}
${BUILDXDETECT}:
	@echo
# Output of `uname -m` is too different 
	@echo "*** 'docker buildx' missing. Install binary for this machine's architecture"
	@echo "*** from https://github.com/docker/buildx/releases/latest"
	@echo "*** to ~/.docker/cli-plugins/docker-buildx"
	@echo
	@exit 1

docker-multiarch-builder: qemu buildx
	if ! docker buildx ls | grep -w docker-multiarch > /dev/null; then \
		docker buildx create --name docker-multiarch && \
		docker buildx inspect --builder docker-multiarch --bootstrap; \
	fi

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

install-crds: generate-manifests $(KUSTOMIZE) ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall-crds: generate-manifests $(KUSTOMIZE) ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy-controller: generate-manifests $(KUSTOMIZE) ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}:${TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

.PHONY: tilt-up
tilt-up: $(ENVSUBST) $(KUSTOMIZE) cluster  ## Start a mgt-cluster & Tilt. Installs the CRDs and deploys the controllers
	EXP_CLUSTER_RESOURCE_SET=true tilt up

install-essentials: ## This gets the secret and installs a CNI and the CCM. Usage: MAKE install-essentials NAME=<cluster-name>
	export CAPH_WORKER_CLUSTER_KUBECONFIG=/tmp/workload-kubeconfig
	$(MAKE) wait-and-get-secret CLUSTER_NAME=$(NAME)
	$(MAKE) install-manifests

wait-and-get-secret:
	# Wait for the kubeconfig to become available.
	${TIMEOUT} 5m bash -c "while ! kubectl get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	kubectl get secrets $(CLUSTER_NAME)-kubeconfig -o json | jq -r .data.value | base64 --decode > $(CAPH_WORKER_CLUSTER_KUBECONFIG)
	${TIMEOUT} 15m bash -c "while ! kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) get nodes | grep master; do sleep 1; done"

install-manifests:
	# Deploy cilium
	helm repo add cilium https://helm.cilium.io/
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install cilium cilium/cilium --version 1.11.1 \
  	--namespace kube-system \
	-f templates/cilium/cilium.yaml

	# Deploy HCloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.9 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=$(PRIVATE_NETWORK)

	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

.PHONY: create-workload-cluster
create-workload-cluster-with-network-packer: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) 
	cat templates/cluster-template-packer-hcloud-network.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests PRIVATE_NETWORK=true

create-workload-cluster-with-network: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) 
	cat templates/cluster-template-hcloud-network.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests PRIVATE_NETWORK=true

create-workload-cluster-packer: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) 
	cat templates/cluster-template-packer.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests PRIVATE_NETWORK=false

create-workload-cluster: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) 
	cat templates/cluster-template.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests PRIVATE_NETWORK=false

move-to-workload-cluster:
	clusterctl init --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
	kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) -n cluster-api-provider-hetzner-system wait deploy/caph-controller-manager --for condition=available && sleep 15s
	clusterctl move --to-kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG)

create-talos-workload-cluster-packer: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) || true
	cat templates/cluster-template-packer-talos.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests

.PHONY: delete-workload-cluster
delete-workload-cluster: ## Deletes the example workload Kubernetes cluster
	@echo 'Your Hetzner resources will now be deleted, this can take up to 20 minutes'
	kubectl patch cluster $(CLUSTER_NAME) --type=merge -p '{"spec":{"paused": "false"}}' || true
	kubectl delete cluster $(CLUSTER_NAME)
	${TIMEOUT} 15m bash -c "while kubectl get cluster | grep $(NAME); do sleep 1; done"
	@echo 'Cluster deleted'

##@ Management Cluster

create-mgt-cluster: cluster ## Start a mgt-cluster with the latest version of all capi components and the hetzner provider. Usage: MAKE create-mgt-cluster HCLOUD=<hcloud-token>
	clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) 
	kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'

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