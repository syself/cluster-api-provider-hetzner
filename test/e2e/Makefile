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

# Use GOPROXY environment variable if set

.DEFAULT_GOAL:=help

GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

REPO_ROOT := $(shell git rev-parse --show-toplevel)

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Binaries
## --------------------------------------

TOOLS_DIR := $(REPO_ROOT)/hack/tools
BIN_DIR := bin
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
GINKGO := $(TOOLS_BIN_DIR)/ginkgo
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize

$(GINKGO): # Build ginkgo from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/ginkgo github.com/onsi/ginkgo/ginkgo

$(KUSTOMIZE): # Build kustomize from tools folder.
	KUSTOMIZE_VERSION=4.5.1	$(REPO_ROOT)/hack/ensure-kustomize.sh

## --------------------------------------
## Templates
## --------------------------------------

HETZNER_TEMPLATES := $(REPO_ROOT)/test/e2e/data/infrastructure-hetzner

e2e-cilium-templates:
	helm repo add cilium https://helm.cilium.io/
	helm repo update cilium
	helm template cilium cilium/cilium --version 1.11.5 \
  	--namespace kube-system \
	-f $(REPO_ROOT)/templates/cilium/cilium.yaml > $(REPO_ROOT)/test/e2e/data/cni/cilium/cilium.yaml

e2e-ccm-templates:
	helm repo add syself https://charts.syself.com
	helm template ccm syself/ccm-hcloud --version 1.0.10 \
	--namespace kube-system \
	--set pdb.enabled=false \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=false > $(REPO_ROOT)/test/e2e/data/ccm/hcloud-ccm.yaml
	helm template ccm syself/ccm-hcloud --version 1.0.10 \
	--namespace kube-system \
	--set pdb.enabled=false \
	--set secret.name=hetzner \
	--set secret.tokenKeyName=hcloud \
	--set privateNetwork.enabled=true > $(REPO_ROOT)/test/e2e/data/ccm/hcloud-ccm-network.yaml
	helm template ccm syself/ccm-hetzner --version 1.1.2 \
	--namespace kube-system \
	--set pdb.enabled=false \
	--set image.tag=latest \
	--set privateNetwork.enabled=false > $(REPO_ROOT)/test/e2e/data/ccm/hcloud-ccm-hetzner.yaml

.PHONY: cluster-templates
cluster-templates: $(KUSTOMIZE) cluster-templates-v1beta1 ## Generate cluster templates for all versions

cluster-templates-v1beta1: $(KUSTOMIZE) ## Generate cluster templates for v1beta1
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-k8s-upgrade --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-k8s-upgrade.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-k8s-upgrade-kcp-scale-in --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-k8s-upgrade-kcp-scale-in.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-packer --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-packer.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-csr-off --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-csr-off.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-loadbalancer-off --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-loadbalancer-off.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-load-balancer-extra-services --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-load-balancer-extra-services.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-placement-groups --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-placement-groups.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-talos --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hcloud-feature-talos.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-network --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-network.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-kcp-remediation --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-kcp-remediation.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-md-remediation --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-md-remediation.yaml
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-node-drain --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-node-drain.yaml
	HETZNER_SSH_PUB=$(HETZNER_SSH_PUB) HETZNER_SSH_PRIV=$(HETZNER_SSH_PRIV) ../../hack/ci-hetzner-secret.sh 
	$(KUSTOMIZE) build $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hetzner-baremetal --load-restrictor LoadRestrictionsNone | sed 's/hcloud_secret_placeholder/$(shell echo -n "$(HCLOUD_TOKEN)" | base64 | tr -d '\n')/' | sed 's/robot-user_secret_placeholder/$(shell echo -n "$(HETZNER_ROBOT_USER)" | base64 | tr -d '\n')/' | sed 's/robot-password_secret_placeholder/$(shell echo -n "$(HETZNER_ROBOT_PASSWORD)" | base64 | tr -d '\n')/' > $(HETZNER_TEMPLATES)/v1beta1/cluster-template-hetzner-baremetal.yaml


## --------------------------------------
## Testing
## --------------------------------------

GINKGO_NODES ?= 1
GINKGO_NOCOLOR ?= false
GINKGO_FOCUS ?= ""
GINKGO_SKIP ?= ""
GINKGO_FLAKE_ATTEMPTS ?= 2
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
SKIP_CLEANUP ?= false
SKIP_CREATE_MGMT_CLUSTER ?= false
E2E_DIR ?= $(REPO_ROOT)/test/e2e
E2E_CONF_FILE ?= $(E2E_DIR)/config/hetzner-ci-envsubst.yaml
KUBETEST_CONF_PATH ?= $(abspath $(E2E_DIR)/data/kubetest/conformance.yaml)

# # to set multiple ginkgo skip flags, if any
# ifneq ($(strip $(GINKGO_SKIP)),)
# _SKIP_ARGS := $(foreach arg,$(strip $(GINKGO_SKIP)),-skip="$(arg)")
# endif

.PHONY: run
run: $(GINKGO)  cluster-templates  ## Run the end-to-end tests
	time $(GINKGO) -v -trace -progress -v -tags=e2e \
		--randomizeAllSpecs -race $(GINKGO_ADDITIONAL_ARGS) \
		-focus=$(GINKGO_FOCUS) -skip=$(GINKGO_SKIP) \
		-nodes=$(GINKGO_NODES) --noColor=$(GINKGO_NOCOLOR) \
		--flakeAttempts=$(GINKGO_FLAKE_ATTEMPTS) ./ -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) \
		-e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER)