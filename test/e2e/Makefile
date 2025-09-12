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


# If you update this file, please follow:
# https://suva.sh/posts/well-documented-makefiles/

.DEFAULT_GOAL:=help

SHELL = /usr/bin/env bash -o pipefail

REPO_ROOT := $(shell git rev-parse --show-toplevel)

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Binaries
## --------------------------------------

TOOLS_DIR := $(REPO_ROOT)/hack/tools
BIN_DIR := bin
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
GINKGO := $(abspath $(TOOLS_BIN_DIR)/ginkgo)
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize

export GOBIN := $(abspath $(TOOLS_BIN_DIR))

ginkgo: $(GINKGO)
$(GINKGO): # Build ginkgo from tools folder.
	go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo

$(KUSTOMIZE): # Build kustomize from tools folder.
	KUSTOMIZE_VERSION=4.5.7	$(REPO_ROOT)/hack/ensure-kustomize.sh

## --------------------------------------
## Templates
## --------------------------------------

HETZNER_TEMPLATES := $(REPO_ROOT)/test/e2e/data/infrastructure-hetzner

e2e-cilium-templates:
	# The `sed` call is needed to avoid:
	#   yamlprocessor.errMissingVariables
	#   value for variables [BIN_PATH] is not set.
	#   Please set the value using os environment variables or the clusterctl config file
	#   Missing: ["BIN_PATH"]
	helm repo add cilium https://helm.cilium.io/
	helm repo update cilium
	helm template cilium cilium/cilium --version 1.17.2 \
  	--namespace kube-system \
	-f $(REPO_ROOT)/templates/cilium/cilium.yaml > $(REPO_ROOT)/test/e2e/data/cni/cilium/cilium.yaml
	sed -i 's/$${BIN_PATH}/$$BIN_PATH/' $(REPO_ROOT)/test/e2e/data/cni/cilium/cilium.yaml
	sed -i '1s/^/# Created by `make e2e-cilium-templates`\n/' $(REPO_ROOT)/test/e2e/data/cni/cilium/cilium.yaml

e2e-ccm-templates:
	helm repo add syself https://charts.syself.com
	helm template ccm syself/ccm-hcloud --version 1.0.11 \
	--namespace kube-system \
	--set pdb.enabled=false \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=false > $(REPO_ROOT)/test/e2e/data/ccm/hcloud-ccm.yaml
	helm template ccm syself/ccm-hcloud --version 1.0.11 \
	--namespace kube-system \
	--set pdb.enabled=false \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=true > $(REPO_ROOT)/test/e2e/data/ccm/hcloud-ccm-network.yaml
	helm template ccm syself/ccm-hetzner --version 2.0.1 \
	--namespace kube-system \
	--set pdb.enabled=false \
	--set privateNetwork.enabled=false > $(REPO_ROOT)/test/e2e/data/ccm/hcloud-ccm-hetzner.yaml

.PHONY: cluster-templates
cluster-templates: $(KUSTOMIZE) cluster-templates-v1beta1 ## Generate cluster templates for all versions

cluster-templates-v1beta1: $(KUSTOMIZE) ## Generate cluster templates for v1beta1
	HETZNER_TEMPLATES=$(HETZNER_TEMPLATES) ./cluster-templates-v1beta1.sh

## --------------------------------------
## Testing
## --------------------------------------

GINKGO_NODES ?= 1
GINKGO_NOCOLOR ?= false
GINKGO_FOCUS ?= ""
GINKGO_SKIP ?= ""
GINKGO_FLAKE_ATTEMPTS ?= 0
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
SKIP_CLEANUP ?= false
SKIP_CREATE_MGMT_CLUSTER ?= false
E2E_DIR ?= $(REPO_ROOT)/test/e2e
E2E_CONF_FILE ?= $(E2E_DIR)/config/hetzner.tmp.yaml
KUBETEST_CONF_PATH ?= $(abspath $(E2E_DIR)/data/kubetest/conformance.yaml)

# # to set multiple ginkgo skip flags, if any
# ifneq ($(strip $(GINKGO_SKIP)),)
# _SKIP_ARGS := $(foreach arg,$(strip $(GINKGO_SKIP)),-skip="$(arg)")
# endif

.PHONY: run
run: $(GINKGO)  cluster-templates  ## Run the end-to-end tests
	time $(GINKGO) -v --trace --show-node-events --tags=e2e \
		--randomize-all -race $(GINKGO_ADDITIONAL_ARGS) \
		--output-dir="$(ARTIFACTS)" \
		--junit-report="junit.e2e_suite.1.xml" \
		--focus=$(GINKGO_FOCUS) --skip=$(GINKGO_SKIP) \
		--no-color=$(GINKGO_NOCOLOR) \
		--flake-attempts=$(GINKGO_FLAKE_ATTEMPTS) ./ -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) \
		-e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER)
