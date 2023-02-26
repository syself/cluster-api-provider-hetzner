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


SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec
.DEFAULT_GOAL:=help
GOTEST ?= go test

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

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

#############
# Variables #
#############

# Certain aspects of the build are done in containers for consistency (e.g. protobuf generation)
# If you have the correct tools installed and you want to speed up development you can run
# make BUILD_IN_CONTAINER=false target
# or you can override this with an environment variable
BUILD_IN_CONTAINER ?= true

# ensure you run `make ci` after changing this
BUILDER_IMAGE_VERSION := 1.0.1

# Boiler plate for building Docker containers.
IMAGE_PREFIX ?= ghcr.io/syself
TAG ?= dev
ARCH ?= amd64
# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always
# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

TIMEOUT := $(shell command -v timeout || command -v gtimeout)

# Directories
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
EXP_DIR := exp
TEST_DIR := test
BIN_DIR := bin
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)
# Default path for Kubeconfig File.
CAPH_WORKER_CLUSTER_KUBECONFIG ?= "/tmp/workload-kubeconfig"

INTEGRATION_CONF_FILE ?= "$(abspath test/integration/integration-dev.yaml)"
E2E_TEMPLATE_DIR := "$(abspath test/e2e/data/infrastructure-hetzner/)"
ARTIFACTS_PATH := $(ROOT_DIR)/_artifacts
CI_KIND ?= true

# Docker
RM := --rm
TTY := -t

# Kubebuilder.
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.25.0
export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT ?= 60s
export KUBEBUILDER_CONTROLPLANE_STOP_TIMEOUT ?= 60s

##@ Binaries
############
# Binaries #
############
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)
controller-gen: $(CONTROLLER_GEN) ## Build a local copy of controller-gen
$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR); go build -mod=vendor -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/kustomize)
kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize
$(KUSTOMIZE): # Build kustomize from tools folder.
	cd $(TOOLS_DIR) && go build -mod=vendor -tags=tools -o $(KUSTOMIZE) sigs.k8s.io/kustomize/kustomize/v4

GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/golangci-lint)
golangci-lint: $(GOLANGCI_LINT) ## Build a local copy of golangci-lint. After running this command do: BUILD_IN_CONTAINER=false make lint
$(GOLANGCI_LINT): images/builder/Dockerfile # Download golanci-lint using hack script into tools folder.
	hack/ensure-golangci-lint.sh \
		-b $(TOOLS_DIR)/$(BIN_DIR) \
		$(shell cat images/builder/Dockerfile | grep "GOLANGCI_VERSION=" | sed 's/.*GOLANGCI_VERSION=//' | sed 's/\s.*$//')

TILT := $(abspath $(TOOLS_BIN_DIR)/tilt)
tilt: $(TILT) ## Build a local copy of tilt
$(TILT):  
	@mkdir $(TOOLS_BIN_DIR) || true
	MINIMUM_TILT_VERSION=0.31.2 hack/ensure-tilt.sh

ENVSUBST := $(abspath $(TOOLS_BIN_DIR)/envsubst)
envsubst: $(ENVSUBST) ## Build a local copy of envsubst
$(ENVSUBST): $(TOOLS_DIR)/go.mod # Build envsubst from tools folder.
	cd $(TOOLS_DIR) && go build -mod=vendor -tags=tools -o $(ENVSUBST) github.com/drone/envsubst/v2/cmd/envsubst

SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/setup-envtest)
setup-envtest: $(SETUP_ENVTEST) ## Build a local copy of setup-envtest
$(SETUP_ENVTEST): $(TOOLS_DIR)/go.mod # Build setup-envtest from tools folder.
	cd $(TOOLS_DIR); go build -mod=vendor -tags=tools -o $(BIN_DIR)/setup-envtest sigs.k8s.io/controller-runtime/tools/setup-envtest

CTLPTL := $(abspath $(TOOLS_BIN_DIR)/ctlptl)
ctlptl: $(CTLPTL) ## Build a local copy of ctlptl
$(CTLPTL): 
	cd $(TOOLS_DIR) && go build -mod=vendor -tags=tools -o $(CTLPTL) github.com/tilt-dev/ctlptl/cmd/ctlptl

CLUSTERCTL := $(abspath $(TOOLS_BIN_DIR)/clusterctl)
clusterctl: $(CLUSTERCTL) ## Build a local copy of clusterctl
$(CLUSTERCTL): $(TOOLS_DIR)/go.mod 
	cd $(TOOLS_DIR) && go build -mod=vendor -tags=tools -o $(CLUSTERCTL) sigs.k8s.io/cluster-api/cmd/clusterctl

KIND := $(abspath $(TOOLS_BIN_DIR)/kind)
kind: $(KIND) ## Build a local copy of kind
$(KIND): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -mod=vendor -tags=tools -o $(KIND) sigs.k8s.io/kind

go-binsize-treemap := $(abspath $(TOOLS_BIN_DIR)/go-binsize-treemap)
go-binsize-treemap: $(go-binsize-treemap) # Build go-binsize-treemap from tools folder.
$(go-binsize-treemap): 
	cd $(TOOLS_DIR); go build -mod=vendor -tags=tools -o $(BIN_DIR)/go-binsize-treemap github.com/nikolaydubina/go-binsize-treemap

go-cover-treemap := $(abspath $(TOOLS_BIN_DIR)/go-cover-treemap)
go-cover-treemap: $(go-cover-treemap) # Build go-cover-treemap from tools folder.
$(go-cover-treemap): 
	cd $(TOOLS_DIR); go build -mod=vendor -tags=tools -o $(BIN_DIR)/go-cover-treemap github.com/nikolaydubina/go-cover-treemap

GOTESTSUM := $(abspath $(TOOLS_BIN_DIR)/gotestsum)
gotestsum: $(GOTESTSUM) # Build gotestsum from tools folder.
$(GOTESTSUM): 
	cd $(TOOLS_DIR); go build -mod=vendor -tags=tools -o $(BIN_DIR)/gotestsum gotest.tools/gotestsum

##@ Development
###############
# Development #
###############
install-crds: generate-manifests $(KUSTOMIZE) ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall-crds: generate-manifests $(KUSTOMIZE) ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy-controller: generate-manifests $(KUSTOMIZE) ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMAGE_PREFIX}/caph-staging:${TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

install-essentials: ## This gets the secret and installs a CNI and the CCM. Usage: MAKE install-essentials NAME=<cluster-name>
	export CAPH_WORKER_CLUSTER_KUBECONFIG=/tmp/workload-kubeconfig
	$(MAKE) wait-and-get-secret CLUSTER_NAME=$(NAME)
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hetzner

wait-and-get-secret:
	# Wait for the kubeconfig to become available.
	${TIMEOUT} 5m bash -c "while ! kubectl get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	kubectl get secrets $(CLUSTER_NAME)-kubeconfig -o json | jq -r .data.value | base64 --decode > $(CAPH_WORKER_CLUSTER_KUBECONFIG)
	${TIMEOUT} 15m bash -c "while ! kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) get nodes | grep control-plane; do sleep 1; done"

install-manifests-cilium:
	# Deploy cilium
	helm repo add cilium https://helm.cilium.io/
	helm repo update cilium
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install cilium cilium/cilium --version 1.12.2 \
  	--namespace kube-system \
	-f templates/cilium/cilium.yaml

install-manifests-ccm-hetzner:
	# Deploy Hetzner Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	helm repo update syself
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hetzner --version 1.1.4 \
	--namespace kube-system \
	--set image.tag=latest \
	--set privateNetwork.enabled=$(PRIVATE_NETWORK)
	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

install-manifests-ccm-hcloud:
	# Deploy Hcloud Cloud Controller Manager
	helm repo add syself https://charts.syself.com
	helm repo update syself
	KUBECONFIG=$(CAPH_WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hcloud --version 1.0.11 \
	--namespace kube-system \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=$(PRIVATE_NETWORK)
	@echo 'run "kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

create-workload-cluster-hcloud: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud.yaml
	cat templates/cluster-templates/cluster-template-hcloud.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hcloud PRIVATE_NETWORK=false

create-workload-cluster-hcloud-packer: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-packer --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-packer.yaml
	cat templates/cluster-templates/cluster-template-hcloud-packer.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hcloud PRIVATE_NETWORK=false

create-workload-cluster-hcloud-talos-packer: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-talos-packer --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-talos-packer.yaml
	cat templates/cluster-templates/cluster-template-hcloud-talos-packer.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hcloud PRIVATE_NETWORK=false

create-workload-cluster-hcloud-network: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-network --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-network.yaml
	cat templates/cluster-templates/cluster-template-hcloud-network.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hcloud PRIVATE_NETWORK=true

create-workload-cluster-hcloud-network-packer: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-network-packer --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-network-packer.yaml
	cat templates/cluster-templates/cluster-template-hcloud-network-packer.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hcloud PRIVATE_NETWORK=true

create-workload-cluster-hetzner-hcloud-control-plane: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --from-literal=robot-user=$(HETZNER_ROBOT_USER) --from-literal=robot-password=$(HETZNER_ROBOT_PASSWORD) --save-config --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic robot-ssh --from-literal=sshkey-name=cluster --from-file=ssh-privatekey=${HETZNER_SSH_PRIV_PATH} --from-file=ssh-publickey=${HETZNER_SSH_PUB_PATH} --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-hcloud-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-hcloud-control-planes.yaml
	cat templates/cluster-templates/cluster-template-hetzner-hcloud-control-planes.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hetzner PRIVATE_NETWORK=false

create-workload-cluster-hetzner-baremetal-control-plane: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --from-literal=robot-user=$(HETZNER_ROBOT_USER) --from-literal=robot-password=$(HETZNER_ROBOT_PASSWORD) --save-config --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic robot-ssh --from-literal=sshkey-name=cluster --from-file=ssh-privatekey=${HETZNER_SSH_PRIV_PATH} --from-file=ssh-publickey=${HETZNER_SSH_PUB_PATH} --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-baremetal-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes.yaml
	cat templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hetzner PRIVATE_NETWORK=false

create-workload-cluster-hetzner-baremetal-control-plane-remediation: $(KUSTOMIZE) $(ENVSUBST) ## Creates a workload-cluster. ENV Variables need to be exported or defined in the tilt-settings.json
	# Create workload Cluster.
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) --from-literal=robot-user=$(HETZNER_ROBOT_USER) --from-literal=robot-password=$(HETZNER_ROBOT_PASSWORD) --save-config --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic robot-ssh --from-literal=sshkey-name=cluster --from-file=ssh-privatekey=${HETZNER_SSH_PRIV_PATH} --from-file=ssh-publickey=${HETZNER_SSH_PUB_PATH} --save-config --dry-run=client -o yaml | kubectl apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-baremetal-control-planes-remediation --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes-remediation.yaml
	cat templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes-remediation.yaml | $(ENVSUBST) - | kubectl apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-manifests-cilium
	$(MAKE) install-manifests-ccm-hetzner PRIVATE_NETWORK=false

move-to-workload-cluster: $(CLUSTERCTL)
	$(CLUSTERCTL) init --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
	kubectl --kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG) -n cluster-api-provider-hetzner-system wait deploy/caph-controller-manager --for condition=available && sleep 15s
	$(CLUSTERCTL) move --to-kubeconfig=$(CAPH_WORKER_CLUSTER_KUBECONFIG)

.PHONY: delete-workload-cluster
delete-workload-cluster: ## Deletes the example workload Kubernetes cluster
	@echo 'Your Hetzner resources will now be deleted, this can take up to 20 minutes'
	kubectl patch cluster $(CLUSTER_NAME) --type=merge -p '{"spec":{"paused": "false"}}' || true
	kubectl delete cluster $(CLUSTER_NAME)
	${TIMEOUT} 15m bash -c "while kubectl get cluster | grep $(NAME); do sleep 1; done"
	@echo 'Cluster deleted'

create-mgt-cluster: $(CLUSTERCTL) cluster ## Start a mgt-cluster with the latest version of all capi components and the hetzner provider. Usage: MAKE create-mgt-cluster HCLOUD=<hcloud-token>
	$(CLUSTERCTL) init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
	kubectl create secret generic hetzner --from-literal=hcloud=$(HCLOUD_TOKEN) 
	kubectl patch secret hetzner -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'

.PHONY: cluster
cluster: $(CTLPTL) ## Creates kind-dev Cluster
	./hack/kind-dev.sh

.PHONY: delete-cluster
delete-cluster: $(CTLPTL) ## Deletes Kind-dev Cluster (default)
	$(CTLPTL) delete cluster kind-caph

.PHONY: delete-registry
delete-registry: $(CTLPTL) ## Deletes Kind-dev Cluster and the local registry
	$(CTLPTL) delete registry caph-registry

.PHONY: delete-cluster-registry
delete-cluster-registry: $(CTLPTL) ## Deletes Kind-dev Cluster and the local registry
	$(CTLPTL) delete cluster kind-caph
	$(CTLPTL) delete registry caph-registry

##@ Clean
#########
# Clean #
#########
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

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_image_patch.yaml ./*manager_pull_policy.yaml

##@ Releasing
#############
# Releasing #
#############
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
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/caph-staging MANIFEST_TAG=$(TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests

.PHONY: release-manifests
release-manifests: generate-manifests generate-go-deepcopy $(KUSTOMIZE) $(RELEASE_DIR) cluster-templates ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	## Build caph-components (aggregate of all of the above).
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp templates/cluster-templates/cluster-template* $(RELEASE_DIR)/
	cp templates/cluster-templates/cluster-class* $(RELEASE_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/caph MANIFEST_TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	## Build the manifests
	$(MAKE) release-manifests clean-release-git

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

##@ Images
##########
# Images #
##########
caph-image: ## Build caph image
	$(SUDO) docker build -t $(IMAGE_PREFIX)/caph-staging:$(TAG) -f images/caph/Dockerfile .
caph-image-cross: ## Build caph image all arch image
	$(SUDO) DOCKER_BUILDKIT=1 docker build -t $(IMAGE_PREFIX)/caph-staging:$(TAG) -f images/caph/Dockerfile.cross .

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_image_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml

builder-image-promote-latest:
	skopeo copy --src-creds=$(USERNAME):$(PASSWORD) --dest-creds=$(USERNAME):$(PASSWORD) docker://ghcr.io/syself/caph-builder:$(BUILDER_IMAGE_VERSION) docker://ghcr.io/syself/caph-builder:latest

##@ Binary
##########
# Binary #
##########
caph: ## Build Caph binary.
	go build -mod=vendor -o bin/manager main.go 

run: ## Run a controller from your host.
	go run ./main.go

##@ Testing
###########
# Testing #
###########
ARTIFACTS ?= _artifacts
$(ARTIFACTS):
	mkdir -p $(ARTIFACTS)/

KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env --bin-dir $(abspath $(TOOLS_BIN_DIR)) -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))

E2E_DIR ?= $(ROOT_DIR)/test/e2e
E2E_CONF_FILE_SOURCE ?= $(E2E_DIR)/config/hetzner.yaml
E2E_CONF_FILE ?= $(E2E_DIR)/config/hetzner-ci-envsubst.yaml

.PHONY: test-unit
test-unit: $(SETUP_ENVTEST) $(GOTESTSUM) ## Run unit and integration tests
	@mkdir -p $(shell pwd)/.coverage
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GOTESTSUM) --junitfile=.coverage/junit.xml --format testname -- -mod=vendor -covermode=atomic -coverprofile=.coverage/cover.out -p=4 ./controllers/... ./pkg/...

.PHONY: e2e-image
e2e-image: ## Build the e2e manager image
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" -t $(IMAGE_PREFIX)/caph-staging:e2e -f images/caph/Dockerfile .

.PHONY: $(E2E_CONF_FILE)
e2e-conf-file: $(E2E_CONF_FILE)
$(E2E_CONF_FILE): $(ENVSUBST) $(E2E_CONF_FILE_SOURCE)
	mkdir -p $(shell dirname $(E2E_CONF_FILE))
	$(ENVSUBST) < $(E2E_CONF_FILE_SOURCE) > $(E2E_CONF_FILE)

.PHONY: test-e2e
test-e2e: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Basic\]'" GINKGO_NODES=2 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-feature
test-e2e-feature: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Feature\]'" GINKGO_NODES=3 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-feature-packer
test-e2e-feature-packer: $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Feature Packer\]'" GINKGO_NODES=1 PACKER_IMAGE_NAME=templates/node-image/1.25.2-ubuntu-22-04-containerd ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-feature-talos
test-e2e-feature-talos: $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Feature Talos\]'" GINKGO_NODES=1 PACKER_TALOS=templates/node-image/talos-image ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-lifecycle
test-e2e-lifecycle: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Lifecycle\]'" GINKGO_NODES=3 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-upgrade-caph
test-e2e-upgrade-caph: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Upgrade CAPH\]'" GINKGO_NODES=2 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-upgrade-kubernetes
test-e2e-upgrade-kubernetes: $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Upgrade Kubernetes\]'" GINKGO_NODES=2 PACKER_KUBERNETES_UPGRADE_FROM=templates/node-image/1.24.1-ubuntu-20-04-containerd PACKER_KUBERNETES_UPGRADE_TO=templates/node-image/1.25.2-ubuntu-22-04-containerd ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-conformance
test-e2e-conformance: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Conformance\]'" GINKGO_NODES=1 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-baremetal
test-e2e-baremetal: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Baremetal\]'" GINKGO_NODES=1 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-baremetal-feature
test-e2e-baremetal-feature: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Baremetal Feature\]'" GINKGO_NODES=1 ./hack/ci-e2e-capi.sh

##@ Report
##########
# Report #
##########
report-cover-html: ## Create a html report
	@mkdir -p $(shell pwd)/.reports
	go tool cover -html .coverage/cover.out -o .reports/coverage.html

report-binsize-treemap: $(go-binsize-treemap) ## Creates a treemap of the binary
	@mkdir -p $(shell pwd)/.reports
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 1024 -h 256 > .reports/caph-binsize-treemap-sm.svg
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 1024 -h 1024 > .reports/caph-binsize-treemap.svg
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 2048 -h 2048 > .reports/caph-binsize-treemap-lg.svg

report-binsize-treemap-all: $(go-binsize-treemap) report-binsize-treemap
	@mkdir -p $(shell pwd)/.reports
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 4096 -h 4096 > .reports/caph-binsize-treemap-xl.svg
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 8192 -h 8192 > .reports/caph-binsize-treemap-xxl.svg

report-cover-treemap: $(go-cover-treemap) ## Creates a treemap of the coverage
	@mkdir -p $(shell pwd)/.reports
	$(go-cover-treemap) -w 1080 -h 360 -coverprofile .coverage/cover.out > .reports/caph-cover-treemap-sm.svg
	$(go-cover-treemap) -w 2048 -h 1280 -coverprofile .coverage/cover.out > .reports/caph-cover-treemap-lg.svg
	$(go-cover-treemap) --only-folders -coverprofile .coverage/cover.out > .reports/caph-cover-treemap-folders.svg

##@ Verify
##########
# Verify #
##########
.PHONY: verify-boilerplate
verify-boilerplate: ## Verify boilerplate text exists in each file
	./hack/verify-boilerplate.sh

.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files
	./hack/verify-shellcheck.sh

.PHONY: verify-starlark
verify-starlark: ## Verify Starlark Code
	./hack/verify-starlark.sh

.PHONY: verify-container-images
verify-container-images: ## Verify container images
	trivy image -q --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL ghcr.io/syself/caph:latest

##@ Generate
############
# Generate #
############
.PHONY: generate-boilerplate
generate-boilerplate: ## Generates missing boilerplates
	./hack/ensure-boilerplate.sh

# support go modules
generate-modules: ## Generates missing go modules
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	./hack/golang-modules-update.sh
endif

generate-modules-ci: generate-modules
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

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

generate-api-ci: generate-manifests generate-go-deepcopy
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

cluster-templates: $(KUSTOMIZE)
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-packer --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-packer.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-talos-packer --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-talos-packer.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-network --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-network.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-network-packer --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-network-packer.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-hcloud-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-hcloud-control-planes.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-baremetal-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-baremetal-control-planes-remediation --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes-remediation.yaml

##@ Format
##########
# Format #
##########
.PHONY: format-golang
format-golang: ## Format the Go codebase and run auto-fixers if supported by the linter.
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	GO111MODULE=on golangci-lint run -v --fix
endif

.PHONY: format-starlark
format-starlark: ## Format the Starlark codebase
	./hack/verify-starlark.sh fix

.PHONY: format-yaml
format-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	yamlfixer --version
	yamlfixer -c .yamllint.yaml .
endif

##@ Lint
########
# Lint #
########
.PHONY: lint-golang
lint-golang: ## Lint Golang codebase
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	GO111MODULE=on golangci-lint run -v 
endif

.PHONY: lint-golang-ci
lint-golang-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	GO111MODULE=on golangci-lint run -v --out-format=github-actions
endif

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml --strict .
endif

.PHONY: lint-yaml-ci
lint-yaml-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml . --format github
endif

DOCKERFILES=$(shell find . -not \( -path ./hack -prune \) -not \( -path ./vendor -prune \) -type f -regex ".*Dockerfile.*"  | tr '\n' ' ')
.PHONY: lint-dockerfile
lint-dockerfile: ## Lint Dockerfiles
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run  $(RM) $(TTY) -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	hadolint --version
	hadolint -t error $(DOCKERFILES)
endif

lint-links: ## Link Checker
ifeq ($(BUILD_IN_CONTAINER),true)
	$(SUDO) docker run $(RM) $(TTY) -i \
		-v $(shell pwd):/src/cluster-api-provider-hetzner$(MOUNT_FLAGS) \
		$(IMAGE_PREFIX)/caph-builder:$(BUILDER_IMAGE_VERSION) $@;
else
	lychee --verbose --config .lychee.toml ./*.md  ./docs/**/*.md  ./cmd/**/*.md
endif

##@ Main Targets
################
# Main Targets #
################
.PHONY: lint
lint: lint-golang lint-yaml lint-dockerfile lint-links ## Lint Codebase

.PHONY: format
format: format-starlark format-golang format-yaml ## Format Codebase

.PHONY: generate
generate: generate-manifests generate-go-deepcopy generate-boilerplate generate-modules ## Generate Files

ALL_VERIFY_CHECKS = boilerplate shellcheck starlark
.PHONY: verify
verify: generate lint $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Verify all
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		echo "Please check the generated files and stage or commit the changes to fix this error."; \
		echo "If you are actively developing these files you can ignore this error"; \
		echo "(Don't forget to check in the generated files when finished)\n"; \
		exit 1; \
	fi

.PHONY: modules
modules: generate-modules ## Update go.mod & go.sum 

.PHONY: boilerplate
boilerplate: generate-boilerplate ## Ensure that your files have a boilerplate header 

.PHONY: builder-image-push
builder-image-push: ## Build caph-builder to a new version. For more information see README.
	./hack/upgrade-builder-image.sh

.PHONY: test
test: test-unit ## Runs all unit and integration tests.

.PHONY: tilt-up
tilt-up: $(ENVSUBST) $(KUSTOMIZE) $(TILT) cluster  ## Start a mgt-cluster & Tilt. Installs the CRDs and deploys the controllers
	EXP_CLUSTER_RESOURCE_SET=true $(TILT) up