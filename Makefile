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

INFRA_SHORT = caph
IMAGE_PREFIX ?= ghcr.io/syself
INFRA_PROVIDER = hetzner

STAGING_IMAGE = $(INFRA_SHORT)-staging
BUILDER_IMAGE = $(IMAGE_PREFIX)/$(INFRA_SHORT)-builder
BUILDER_IMAGE_VERSION = $(shell cat .builder-image-version.txt)

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec
.DEFAULT_GOAL:=help
GOTEST ?= go test

# https://github.com/syself/hetzner-cloud-controller-manager#networks-support
PRIVATE_NETWORK ?= false

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

# Boiler plate for building Docker containers.
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
export GOBIN := $(abspath $(TOOLS_BIN_DIR))

# Files
WORKER_CLUSTER_KUBECONFIG ?= ".workload-cluster-kubeconfig.yaml"
MGT_CLUSTER_KUBECONFIG ?= ".mgt-cluster-kubeconfig.yaml"

# Kubebuilder.
# go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
# The command `setup-envtest list` shows the available versions.
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.33.6

##@ Binaries
############
# Binaries #
############

KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/kustomize)
kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize
$(KUSTOMIZE): # Build kustomize from tools folder.
	go install sigs.k8s.io/kustomize/kustomize/v5@v5.8.0

TILT := $(abspath $(TOOLS_BIN_DIR)/tilt)
tilt: $(TILT) ## Build a local copy of tilt
$(TILT):
	@mkdir -p $(TOOLS_BIN_DIR)
	MINIMUM_TILT_VERSION=0.35.2 hack/ensure-tilt.sh

SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/setup-envtest)
setup-envtest: $(SETUP_ENVTEST) ## Build a local copy of setup-envtest
$(SETUP_ENVTEST): # Build setup-envtest from tools folder.
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20251126220622-4b46eb04d57f

CTLPTL := $(abspath $(TOOLS_BIN_DIR)/ctlptl)
ctlptl: $(CTLPTL) ## Build a local copy of ctlptl
$(CTLPTL):
	go install github.com/tilt-dev/ctlptl/cmd/ctlptl@v0.8.43

CLUSTERCTL := $(abspath $(TOOLS_BIN_DIR)/clusterctl)
clusterctl: $(CLUSTERCTL) ## Build a local copy of clusterctl
$(CLUSTERCTL):
	go install sigs.k8s.io/cluster-api/cmd/clusterctl@v1.11.3

HCLOUD := $(abspath $(TOOLS_BIN_DIR)/hcloud)
hcloud: $(HCLOUD) ## Build a local copy of hcloud
$(HCLOUD):
	curl -sSL https://github.com/hetznercloud/cli/releases/download/v1.57.0/hcloud-$$(go env GOOS)-$$(go env GOARCH).tar.gz | tar xz -C $(TOOLS_BIN_DIR) hcloud
	chmod a+rx $(HCLOUD)

KIND := $(abspath $(TOOLS_BIN_DIR)/kind)
kind: $(KIND) ## Build a local copy of kind
$(KIND):
	go install sigs.k8s.io/kind@v0.30.0

KUBECTL := $(abspath $(TOOLS_BIN_DIR)/kubectl)
kubectl: $(TOOLS_BIN_DIR) $(KUBECTL) ## Build a local copy of kubectl
$(KUBECTL):
	mkdir -p $(TOOLS_BIN_DIR)
	curl -fsSL "https://dl.k8s.io/release/v1.33.6/bin/$$(go env GOOS)/$$(go env GOARCH)/kubectl" -o $(KUBECTL)
	chmod a+rx $(KUBECTL)

go-binsize-treemap := $(abspath $(TOOLS_BIN_DIR)/go-binsize-treemap)
go-binsize-treemap: $(go-binsize-treemap) # Build go-binsize-treemap from tools folder.
$(go-binsize-treemap):
	go install github.com/nikolaydubina/go-binsize-treemap@v0.2.3

go-cover-treemap := $(abspath $(TOOLS_BIN_DIR)/go-cover-treemap)
go-cover-treemap: $(go-cover-treemap) # Build go-cover-treemap from tools folder.
$(go-cover-treemap):
	go install github.com/nikolaydubina/go-cover-treemap@v1.5.0

GOTESTSUM := $(abspath $(TOOLS_BIN_DIR)/gotestsum)
gotestsum: $(GOTESTSUM) # Build gotestsum from tools folder.
$(GOTESTSUM):
	go install gotest.tools/gotestsum@v1.13.0

all-tools: $(GOTESTSUM) $(go-cover-treemap) $(go-binsize-treemap) $(KIND) $(KUBECTL) $(CLUSTERCTL) $(CTLPTL) $(SETUP_ENVTEST) $(KUSTOMIZE) ## Install all tools required for development
	echo 'done'

##@ Development
###############
# Development #
###############
install-crds: generate-manifests $(KUSTOMIZE) ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -


uninstall-crds: generate-manifests $(KUSTOMIZE) ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete -f -

undeploy-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete -f -

install-essentials: ## This gets the secret and installs a CNI and the CCM. Usage: MAKE install-essentials CLUSTER_NAME=<cluster-name>
	$(MAKE) wait-and-get-secret CLUSTER_NAME=$(CLUSTER_NAME)
	$(MAKE) install-cilium-in-wl-cluster
	$(MAKE) install-ccm-in-wl-cluster

wait-and-get-secret: $(KUBECTL)
	./hack/ensure-env-variables.sh CLUSTER_NAME
	# Wait for the kubeconfig to become available.
	rm -f $(WORKER_CLUSTER_KUBECONFIG)
	${TIMEOUT} --foreground 5m bash -c "while ! $(KUBECTL) get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	./hack/get-kubeconfig-of-workload-cluster.sh
	${TIMEOUT} --foreground 15m bash -c "while ! $(KUBECTL) --kubeconfig=$(WORKER_CLUSTER_KUBECONFIG) get nodes | grep control-plane; do sleep 1; done"

install-cilium-in-wl-cluster:
	# Deploy cilium
	helm repo add cilium https://helm.cilium.io/
	helm repo update cilium
	KUBECONFIG=$(WORKER_CLUSTER_KUBECONFIG) helm upgrade --install cilium cilium/cilium \
  		--namespace kube-system \
		-f templates/cilium/cilium.yaml


install-ccm-in-wl-cluster:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	helm repo add syself https://charts.syself.com
	helm repo update syself
	KUBECONFIG=$(WORKER_CLUSTER_KUBECONFIG) helm upgrade --install ccm syself/ccm-hetzner --version 2.0.1 \
	--namespace kube-system \
	--set privateNetwork.enabled=$(PRIVATE_NETWORK)
	@echo 'run "kubectl --kubeconfig=$(WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'
endif

add-ssh-pub-key:
	./hack/ensure-env-variables.sh HCLOUD_TOKEN SSH_KEY SSH_KEY_NAME
	SSH_KEY_CONTENT=$$(cat $(SSH_KEY)) ; \
	curl -sS \
		-X POST \
		-H "Authorization: Bearer $${HCLOUD_TOKEN}" \
		-H "Content-Type: application/json" \
		-d '{"labels":{},"name":"${SSH_KEY_NAME}","public_key":"'"$${SSH_KEY_CONTENT}"'"}' \
		'https://api.hetzner.cloud/v1/ssh_keys'

env-vars-for-wl-cluster:
	@./hack/ensure-env-variables.sh CLUSTER_NAME CONTROL_PLANE_MACHINE_COUNT HCLOUD_CONTROL_PLANE_MACHINE_TYPE \
	HCLOUD_REGION SSH_KEY_NAME HCLOUD_WORKER_MACHINE_TYPE KUBERNETES_VERSION WORKER_MACHINE_COUNT

create-workload-cluster-hcloud: env-vars-for-wl-cluster $(KUSTOMIZE) install-crds ## Creates a workload-cluster.
	# Create workload Cluster.
	./hack/ensure-env-variables.sh HCLOUD_TOKEN
	$(KUBECTL) create secret generic $(INFRA_PROVIDER) --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud.yaml
	cat templates/cluster-templates/cluster-template-hcloud.yaml | $(CLUSTERCTL) generate yaml | $(KUBECTL) apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-cilium-in-wl-cluster
	$(MAKE) install-ccm-in-wl-cluster

create-workload-cluster-hcloud-network: env-vars-for-wl-cluster $(KUSTOMIZE) ## Creates a workload-cluster.
	# Create workload Cluster.
	./hack/ensure-env-variables.sh HCLOUD_TOKEN
	$(KUBECTL) create secret generic $(INFRA_PROVIDER) --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-network --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-network.yaml
	cat templates/cluster-templates/cluster-template-hcloud-network.yaml | $(CLUSTERCTL) generate yaml | $(KUBECTL) apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-cilium-in-wl-cluster
	$(MAKE) install-ccm-in-wl-cluster PRIVATE_NETWORK=true

# Use that, if you want to test hcloud control-planes, hcloud worker and bm worker.
create-workload-cluster-hetzner-hcloud-control-plane: env-vars-for-wl-cluster $(KUSTOMIZE) ## Creates a workload-cluster.
	# Create workload Cluster.
	./hack/ensure-env-variables.sh HCLOUD_TOKEN HETZNER_ROBOT_USER HETZNER_ROBOT_PASSWORD HETZNER_SSH_PRIV_PATH HETZNER_SSH_PUB_PATH SSH_KEY_NAME
	$(KUBECTL) create secret generic $(INFRA_PROVIDER) --from-literal=hcloud=$(HCLOUD_TOKEN) --from-literal=robot-user=$(HETZNER_ROBOT_USER) --from-literal=robot-password=$(HETZNER_ROBOT_PASSWORD) --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic robot-ssh --from-literal=sshkey-name=$(SSH_KEY_NAME) --from-file=ssh-privatekey=${HETZNER_SSH_PRIV_PATH} --from-file=ssh-publickey=${HETZNER_SSH_PUB_PATH} --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/$(INFRA_PROVIDER)-hcloud-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-$(INFRA_PROVIDER)-hcloud-control-planes.yaml
	cat templates/cluster-templates/cluster-template-$(INFRA_PROVIDER)-hcloud-control-planes.yaml | $(CLUSTERCTL) generate yaml | $(KUBECTL) apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-cilium-in-wl-cluster
	$(MAKE) install-ccm-in-wl-cluster

create-workload-cluster-hetzner-baremetal-control-plane: env-vars-for-wl-cluster $(KUSTOMIZE) ## Creates a workload-cluster.
	# Create workload Cluster.
	./hack/ensure-env-variables.sh HCLOUD_TOKEN HETZNER_ROBOT_USER HETZNER_ROBOT_PASSWORD HETZNER_SSH_PRIV_PATH HETZNER_SSH_PUB_PATH SSH_KEY_NAME
	$(KUBECTL) create secret generic $(INFRA_PROVIDER) --from-literal=hcloud=$(HCLOUD_TOKEN) --from-literal=robot-user=$(HETZNER_ROBOT_USER) --from-literal=robot-password=$(HETZNER_ROBOT_PASSWORD) --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic robot-ssh --from-literal=sshkey-name=$(SSH_KEY_NAME) --from-file=ssh-privatekey=${HETZNER_SSH_PRIV_PATH} --from-file=ssh-publickey=${HETZNER_SSH_PUB_PATH} --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/$(INFRA_PROVIDER)-baremetal-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-$(INFRA_PROVIDER)-baremetal-control-planes.yaml
	cat templates/cluster-templates/cluster-template-$(INFRA_PROVIDER)-baremetal-control-planes.yaml | $(CLUSTERCTL) generate yaml | $(KUBECTL) apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-cilium-in-wl-cluster
	$(MAKE) install-ccm-in-wl-cluster

create-workload-cluster-hetzner-baremetal-control-plane-remediation: env-vars-for-wl-cluster $(KUSTOMIZE) ## Creates a workload-cluster.
	# Create workload Cluster.
	./hack/ensure-env-variables.sh HCLOUD_TOKEN HETZNER_ROBOT_USER HETZNER_ROBOT_PASSWORD HETZNER_SSH_PRIV_PATH HETZNER_SSH_PUB_PATH SSH_KEY_NAME
	$(KUBECTL) create secret generic $(INFRA_PROVIDER) --from-literal=hcloud=$(HCLOUD_TOKEN) --from-literal=robot-user=$(HETZNER_ROBOT_USER) --from-literal=robot-password=$(HETZNER_ROBOT_PASSWORD) --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic robot-ssh --from-literal=sshkey-name=$(SSH_KEY_NAME) --from-file=ssh-privatekey=${HETZNER_SSH_PRIV_PATH} --from-file=ssh-publickey=${HETZNER_SSH_PUB_PATH} --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build templates/cluster-templates/$(INFRA_PROVIDER)-baremetal-control-planes-remediation --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-$(INFRA_PROVIDER)-baremetal-control-planes-remediation.yaml
	cat templates/cluster-templates/cluster-template-$(INFRA_PROVIDER)-baremetal-control-planes-remediation.yaml | $(CLUSTERCTL) generate yaml | $(KUBECTL) apply -f -
	$(MAKE) wait-and-get-secret
	$(MAKE) install-cilium-in-wl-cluster
	$(MAKE) install-ccm-in-wl-cluster

move-to-workload-cluster: $(CLUSTERCTL)
	$(CLUSTERCTL) init --kubeconfig=$(WORKER_CLUSTER_KUBECONFIG) --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure $(INFRA_PROVIDER)
	$(KUBECTL) --kubeconfig=$(WORKER_CLUSTER_KUBECONFIG) -n $(INFRA_SHORT)-system wait deploy/$(INFRA_SHORT)-controller-manager --for condition=available && sleep 15s
	$(CLUSTERCTL) move --to-kubeconfig=$(WORKER_CLUSTER_KUBECONFIG)

.PHONY: delete-workload-cluster
delete-workload-cluster: ## Deletes the example workload Kubernetes cluster
	./hack/ensure-env-variables.sh CLUSTER_NAME
	@echo 'Your workload cluster will now be deleted, this can take up to 20 minutes'
	$(KUBECTL) patch cluster $(CLUSTER_NAME) --type=merge -p '{"spec":{"paused": false}}'
	$(KUBECTL) delete cluster $(CLUSTER_NAME)
	${TIMEOUT} --foreground 15m bash -c "while $(KUBECTL) get cluster | grep $(NAME); do sleep 1; done"
	@echo 'Cluster deleted'

create-mgt-cluster: $(CLUSTERCTL) $(KUBECTL) cluster ## Start a mgt-cluster with the latest version of all capi components and the infra provider.
	$(CLUSTERCTL) init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure $(INFRA_PROVIDER)
	$(KUBECTL) create secret generic $(INFRA_PROVIDER) --from-literal=hcloud=$(HCLOUD_TOKEN) --save-config --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) patch secret $(INFRA_PROVIDER) -p '{"metadata":{"labels":{"clusterctl.cluster.x-k8s.io/move":""}}}'

.PHONY: cluster
cluster: $(CTLPTL) $(KUBECTL) ## Creates kind-dev Cluster
	@# Fail early: Test if HCLOUD_TOKEN is valid. Background: After Tilt started, changing .envrc has no effect for processes
	@# started via Tilt. That's why this should fail early.
	@curl -fsS -H "Authorization: Bearer $$HCLOUD_TOKEN" 'https://api.hetzner.cloud/v1/ssh_keys' > /dev/null || (echo "HCLOUD_TOKEN is invalid (might help: ./hack/ci-e2e-get-token.sh and update .envrc)"; exit 1)
	./hack/kind-dev.sh

.PHONY: delete-mgt-cluster
delete-mgt-cluster: $(CTLPTL) ## Deletes Kind-dev Cluster (default)
	$(CTLPTL) delete cluster kind-$(INFRA_SHORT)

.PHONY: delete-registry
delete-registry: $(CTLPTL) ## Deletes Kind-dev Cluster and the local registry
	$(CTLPTL) delete registry $(INFRA_SHORT)-registry

.PHONY: delete-mgt-cluster-registry
delete-mgt-cluster-registry: $(CTLPTL) ## Deletes Kind-dev Cluster and the local registry
	$(CTLPTL) delete cluster kind-$(INFRA_SHORT)
	$(CTLPTL) delete registry $(INFRA_SHORT)-registry

generate-hcloud-token:
	@if [ -n "$${TTS_TOKEN}" ]; then \
		echo "Error: TTS_TOKEN is set. Please remove the deprecated variable (.envrc ?)."; \
		exit 1; \
	fi
	./hack/ensure-env-variables.sh TPS_TOKEN
	./hack/ci-e2e-get-token.sh

##@ Clean
#########
# Clean #
#########
.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin
	rm -rf test/e2e/data/infrastructure-hetzner/*/cluster-template*.yaml

.PHONY: clean-bin
clean-bin: ## Remove all generated helper binaries
	rm -rf $(BIN_DIR)
	rm -rf $(TOOLS_BIN_DIR)

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_config_patch.yaml ./*manager_pull_policy.yaml

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
	@# CAPH_CONTAINER_TAG: caph container image tag. For PRs this is pr-NNNN
	./hack/ensure-env-variables.sh CAPH_CONTAINER_TAG
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/$(STAGING_IMAGE) MANIFEST_TAG=$(CAPH_CONTAINER_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests

.PHONY: release-manifests
release-manifests: generate-manifests generate-go-deepcopy $(KUSTOMIZE) $(RELEASE_DIR) cluster-templates ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	## Build $(INFRA_SHORT)-components (aggregate of all of the above).
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp templates/cluster-templates/cluster-template* $(RELEASE_DIR)/
	cp templates/cluster-templates/cluster-class* $(RELEASE_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/$(INFRA_SHORT) MANIFEST_TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	## Build the manifests
	$(MAKE) release-manifests clean-release-git
	./hack/check-release-manifests.sh


.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

##@ Images
##########
# Images #
##########

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_config_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml

builder-image-promote-latest:
	./hack/ensure-env-variables.sh USERNAME PASSWORD
	skopeo copy --src-creds=$(USERNAME):$(PASSWORD) --dest-creds=$(USERNAME):$(PASSWORD) \
		docker://$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) \
		docker://$(BUILDER_IMAGE):latest

##@ Binary
##########
# Binary #
##########
$(INFRA_SHORT): ## Build controller binary.
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


$(MGT_CLUSTER_KUBECONFIG):
	./hack/get-kubeconfig-of-management-cluster.sh

$(WORKER_CLUSTER_KUBECONFIG):
	./hack/get-kubeconfig-of-workload-cluster.sh

.PHONY: k9s-workload-cluster
k9s-workload-cluster: $(WORKER_CLUSTER_KUBECONFIG)
	KUBECONFIG=$(WORKER_CLUSTER_KUBECONFIG) k9s

.PHONY: bash-with-kubeconfig-set-to-workload-cluster
bash-with-kubeconfig-set-to-workload-cluster: $(WORKER_CLUSTER_KUBECONFIG)
	KUBECONFIG=$(WORKER_CLUSTER_KUBECONFIG) bash

.PHONY: tail-controller-logs
tail-controller-logs: ## Show the last lines of the controller logs
	@hack/tail-controller-logs.sh

.PHONY: ssh-first-control-plane
ssh-first-control-plane: ## ssh into the first control-plane
	@hack/ssh-first-control-plane.sh


E2E_DIR ?= $(ROOT_DIR)/test/e2e
E2E_CONF_FILE_SOURCE ?= $(E2E_DIR)/config/$(INFRA_PROVIDER).yaml
E2E_CONF_FILE ?= $(E2E_DIR)/config/$(INFRA_PROVIDER).tmp.yaml


.PHONY: test-unit
test-unit: $(SETUP_ENVTEST) $(GOTESTSUM) ## Run unit and integration tests
	echo  $(SETUP_ENVTEST) $(GOTESTSUM)
	./hack/test-unit.sh

.PHONY: e2e-image
e2e-image: ## Build the e2e manager image
	./hack/ensure-env-variables.sh CAPH_CONTAINER_TAG
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" -t $(IMAGE_PREFIX)/$(STAGING_IMAGE):$(CAPH_CONTAINER_TAG) -f images/$(INFRA_SHORT)/Dockerfile .

.PHONY: e2e-conf-file
e2e-conf-file: $(E2E_CONF_FILE)
$(E2E_CONF_FILE): $(E2E_CONF_FILE_SOURCE) ./hack/create-e2e-conf-file.sh
	CAPH_LATEST_VERSION=$(CAPH_LATEST_VERSION) \
	E2E_CONF_FILE_SOURCE=$(E2E_CONF_FILE_SOURCE) \
	E2E_CONF_FILE=$(E2E_CONF_FILE) \
	CLUSTERCTL=$(CLUSTERCTL) \
	./hack/create-e2e-conf-file.sh

.PHONY: test-e2e
test-e2e: test-e2e-hcloud

.PHONY: test-e2e-hcloud
test-e2e-hcloud: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	rm -f $(WORKER_CLUSTER_KUBECONFIG)
	HETZNER_SSH_PUB= HETZNER_SSH_PRIV= \
	HETZNER_SSH_PUB_PATH= HETZNER_SSH_PRIV_PATH= \
	HETZNER_ROBOT_PASSWORD= HETZNER_ROBOT_USER= \
	GINKGO_FOKUS="'\[Basic\]'" GINKGO_NODES=2 \
	./hack/ci-e2e-capi.sh

.PHONY: test-e2e-feature
test-e2e-feature: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Feature\]'" GINKGO_NODES=3 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-lifecycle
test-e2e-lifecycle: $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Lifecycle\]'" GINKGO_NODES=3 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-upgrade-$(INFRA_SHORT)
test-e2e-upgrade-$(INFRA_SHORT): $(E2E_CONF_FILE) $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Upgrade CAPH\]'" GINKGO_NODES=2 ./hack/ci-e2e-capi.sh

.PHONY: test-e2e-upgrade-kubernetes
test-e2e-upgrade-kubernetes: $(if $(SKIP_IMAGE_BUILD),,e2e-image) $(ARTIFACTS)
	GINKGO_FOKUS="'\[Upgrade Kubernetes\]'" GINKGO_NODES=2 ./hack/ci-e2e-capi.sh

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
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 1024 -h 256 > .reports/$(INFRA_SHORT)-binsize-treemap-sm.svg
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 1024 -h 1024 > .reports/$(INFRA_SHORT)-binsize-treemap.svg
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 2048 -h 2048 > .reports/$(INFRA_SHORT)-binsize-treemap-lg.svg

report-binsize-treemap-all: $(go-binsize-treemap) report-binsize-treemap
	@mkdir -p $(shell pwd)/.reports
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 4096 -h 4096 > .reports/$(INFRA_SHORT)-binsize-treemap-xl.svg
	go tool nm -size bin/manager | $(go-binsize-treemap) -w 8192 -h 8192 > .reports/$(INFRA_SHORT)-binsize-treemap-xxl.svg

report-cover-treemap: $(go-cover-treemap) ## Creates a treemap of the coverage
	@mkdir -p $(shell pwd)/.reports
	$(go-cover-treemap) -w 1080 -h 360 -coverprofile .coverage/cover.out > .reports/$(INFRA_SHORT)-cover-treemap-sm.svg
	$(go-cover-treemap) -w 2048 -h 1280 -coverprofile .coverage/cover.out > .reports/$(INFRA_SHORT)-cover-treemap-lg.svg
	$(go-cover-treemap) --only-folders -coverprofile .coverage/cover.out > .reports/$(INFRA_SHORT)-cover-treemap-folders.svg

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

.PHONY: verify-manifests ## Verify Manifests
verify-manifests:
	./hack/verify-manifests.sh

.PHONY: verify-container-images
verify-container-images: ## Verify container images
	trivy image -q --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL $(IMAGE_PREFIX)/$(INFRA_SHORT):latest

.PHONY: verify-generated-files
verify-generated-files: ## Verify geneated files in git repo
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	./hack/verify-generated-files.sh
endif

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
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	./hack/golang-modules-update.sh
endif

$(HOME)/.ssh/$(INFRA_PROVIDER).pub:
	echo "Creating SSH key-pair to access the nodes which get created by $(INFRA_PROVIDER)"
	ssh-keygen -f ~/.ssh/$(INFRA_PROVIDER)

generate-modules-ci: generate-modules
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

generate-manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	# Ensure that these old binaries are not longer used. We use
	# these from the builder-image now.
	rm -f ./hack/tools/bin/controller-gen ./hack/tools/bin/helm
	controller-gen \
			paths=./api/... \
			paths=./controllers/... \
			crd:crdVersions=v1 \
			rbac:roleName=manager-role \
			output:crd:dir=./config/crd/bases \
			output:webhook:dir=./config/webhook \
			webhook
endif

generate-go-deepcopy: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	controller-gen \
		object:headerFile="./hack/boilerplate/boilerplate.generatego.txt" \
		paths="./api/..."
endif

generate-api-ci: generate-manifests generate-go-deepcopy
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

cluster-templates: $(KUSTOMIZE)
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-network --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-network.yaml
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
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run -v --fix
endif

.PHONY: format-starlark
format-starlark: ## Format the Starlark codebase
	./hack/verify-starlark.sh fix

.PHONY: format-yaml
format-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
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
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run -v
endif

.PHONY: lint-golang-ci
lint-golang-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run --out-format=github-actions
endif

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml --strict .
endif

.PHONY: lint-yaml-ci
lint-yaml-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml . --format github
endif

DOCKERFILES=$(shell find . -not \( -path ./hack -prune \) -not \( -path ./vendor -prune \) -type f -regex ".*Dockerfile.*"  | tr '\n' ' ')
.PHONY: lint-dockerfile
lint-dockerfile: ## Lint Dockerfiles
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	hadolint --version
	hadolint -t error $(DOCKERFILES)
endif

lint-links: ## Link Checker
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run --rm \
		-e GITHUB_TOKEN \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	@lychee --version
	@if [ -z "$${GITHUB_TOKEN}" ]; then echo "GITHUB_TOKEN is not set"; exit 1; fi
	lychee --verbose --config .lychee.toml --cache ./*.md  ./docs/**/*.md 2>&1 | grep -vP '\[(200|EXCLUDED)\]'
endif

##@ Main Targets
################
# Main Targets #
################
.PHONY: lint
lint: lint-golang lint-yaml lint-dockerfile lint-links ## Lint Codebase

.PHONY: format
format: format-starlark format-golang format-yaml ## Format Codebase

.PHONY: generate-mocks
generate-mocks: ## Generate Mocks
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go run github.com/vektra/mockery/v2@v2.53.4
endif

.PHONY: generate
generate: generate-manifests generate-go-deepcopy generate-boilerplate generate-modules generate-mocks ## Generate Files

ALL_VERIFY_CHECKS = boilerplate shellcheck starlark manifests
.PHONY: verify
verify: generate lint $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Verify all

.PHONY: modules
modules: generate-modules ## Update go.mod & go.sum

.PHONY: boilerplate
boilerplate: generate-boilerplate ## Ensure that your files have a boilerplate header

.PHONY: builder-image-push
builder-image-push: ## Build $(INFRA_SHORT)-builder to a new version. For more information see README.
	BUILDER_IMAGE=$(BUILDER_IMAGE) ./hack/upgrade-builder-image.sh

.PHONY: test
test: test-unit ## Runs all unit and integration tests.

.PHONY: tilt-up
tilt-up: env-vars-for-wl-cluster $(KUBECTL) $(KUSTOMIZE) $(TILT) cluster  ## Start a mgt-cluster & Tilt. Installs the CRDs and deploys the controllers
	@mkdir -p .tiltbuild
	EXP_CLUSTER_RESOURCE_SET=true $(TILT) up --port=10351

.PHONY: watch
watch: ## Watch CRDs cluster, machines and Events.
	watch -c -n 2 hack/output-for-watch.sh

.PHONY: create-hetzner-installimage-tgz
create-hetzner-installimage-tgz:
	rm -rf data/hetzner-installimage*
	cd data; \
	  installimageurl=$$(curl -sL https://api.github.com/repos/syself/hetzner-installimage/releases/latest | jq -r .assets[].browser_download_url); \
	  echo $$installimageurl; \
	  curl -sSLO $$installimageurl
	@if [ $$(tar -tzf data/hetzner-installimage*tgz| cut -d/ -f1| sort | uniq) != "hetzner-installimage" ]; then \
	   echo "tgz must contain only one directory. And it must be 'hetzner-installimage'."; \
	   exit 1; \
	fi
	@echo
	@echo "============= ↓↓↓↓↓ Now update the version number here ↓↓↓↓↓ ============="
	@git ls-files | xargs grep -P 'hetzner-installimage.*v\d+\.\d+' || true
	@echo "↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑"

builder-image-shell: ## Start an interactive shell in the builder image.
	docker run --rm -t -i \
		--entrypoint bash \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION)
