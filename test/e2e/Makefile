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
	-f $(REPO_ROOT)/templates/cilium/cilium.yaml \
	--set-string hubble.relay.tls.server.ca.crt="LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURGRENDQWZ5Z0F3SUJBZ0lSQUxWa0wxN1NYdVcyQXNVaUVUY2dsdjB3RFFZSktvWklodmNOQVFFTEJRQXcKRkRFU01CQUdBMVVFQXhNSlEybHNhWFZ0SUVOQk1CNFhEVEkxTURreE1qRTNOVEV3TmxvWERUSTRNRGt4TVRFMwpOVEV3Tmxvd0ZERVNNQkFHQTFVRUF4TUpRMmxzYVhWdElFTkJNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DCkFROEFNSUlCQ2dLQ0FRRUF6ai9ZYW0xQWNnOWlFdmorTG9KUVBpOUVBZ2JXRC9zblYvZUc1SGlvR0syTlpkNzcKaUcrWHZGYkJ1TThYbU5zWmdlQTJqRVJjemNFVHMwd0hhTUlSRGdwRXFWK0t5Q1lnUC93ODc1aXFJVmV2Y28zaQo3bkVXSjRnbE15a2lzNGVKWkJIOG9yZ1VVb2JEUFhUeDc3N2dEeU5wVVN0RXlzUXlTUWRndDdPSUg4TzVoUWdLClE0Y09SY0p3d0Q1NGZsRXR5bndmM2JwaE85ejF4eE9LRC90d0tHdnlCOWdtSWIzR01Tbnczck9id1hhZ2VHVmkKamF1c0hMK3Jxemw2aDFyUWxnSTZtMjNUTEt2VjV0Wm15Z1JwS0pCZ3lBVFJzOVE4aXMyZXFyTFJpOWsrYk5QMApaeVZHK3NSdCtvUkJCdXpVYUkzM0tja0RCNDVOeWFvdFpSLzl6UUlEQVFBQm8yRXdYekFPQmdOVkhROEJBZjhFCkJBTUNBcVF3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdFR0NDc0dBUVVGQndNQ01BOEdBMVVkRXdFQi93UUYKTUFNQkFmOHdIUVlEVlIwT0JCWUVGR2JudXlxMkdubm1VdXZCVjBHSE9BdDl0Z2dKTUEwR0NTcUdTSWIzRFFFQgpDd1VBQTRJQkFRQlpGSzdxWEFhQTA1RUtqRi8xZTFxRXhDbFltV0NMSGNGQzZRUnRiTDZmbmhIU2pIYXgyVVNrCmFOVnZFSis4MXVvL2pkbDgxQWdRV0xQQTdXbUNZbk9iUzduWUhRNUhWeXhGV0tKSENoR2pCYzhOd0ErVmlBaS8KTmoxeWtjeDVYZUk3d3IzYklmYjN1eXRGazNKcVJwZkVYTC8rOHZQQ1BtNGQ0UEszbHVHZldhS2JCR3RSb1ZxdQpJb0hzWFByeFA5NE1KNUpzZTBWM0NDbTJ5YVZvTWV2c3lnZlBoMG90WHczaC9wcEVBNUFQM2g5MkJXbUVJVStQCm1mMVZRM2VOUmo1RU93QlhocStQdUduR1luem1HcW56U094TVhoMEd5M0l0Nkg1MTFUdVVOckJtUDQvdTBUR1IKVFNyK2psVm02dTkzMXBSY0piMXFhU1gvclZVci9paTAKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=" \
	--set-string hubble.relay.tls.server.crt="STATIC_SERVER_CRT_VALUE" \
	--set-string hubble.relay.tls.server.key="LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBemovWWFtMUFjZzlpRXZqK0xvSlFQaTlFQWdiV0Qvc25WL2VHNUhpb0dLMk5aZDc3CmlHK1h2RmJCdU04WG1Oc1pnZUEyakVSY3pjRVRzMHdIYU1JUkRncEVxVitLeUNZZ1Avdzg3NWlxSVZldmNvM2kKN25FV0o0Z2xNeWtpczRlSlpCSDhvcmdVVW9iRFBYVHg3NzdnRHlOcFVTdEV5c1F5U1FkZ3Q3T0lIOE81aFFnSwpRNGNPUmNKd3dENTRmbEV0eW53ZjNicGhPOXoxeHhPS0QvdHdLR3Z5QjlnbUliM0dNU253M3JPYndYYWdlR1ZpCmphdXNITCtycXpsNmgxclFsZ0k2bTIzVExLdlY1dFpteWdScEtKQmd5QVRSczlROGlzMmVxckxSaTlrK2JOUDAKWnlWRytzUnQrb1JCQnV6VWFJMzNLY2tEQjQ1Tnlhb3RaUi85elFJREFRQUJBb0lCQVFDWWYxcDc3b1hORUJTNgo1SUhFVDdRWEZvbVpXZ2FpMEZmWjBmcGFLZUJZUnRLYXRSR1VZcloxbWt3b1ltOFhKVUVwNTJkTnRTcE5NK3BtCkV0d1RtQkpXS0Q5MllGOVQ5K1MyaXNGZmdxaWdlSjRQL0ZzRFpIZzJDSk5BemJTTHU3WlpXSGRNdFFESnVEeloKendNbitpRGU4SmJwWGkzQ0l1anZXRllwM2NQVGYyWjcrd1BORGJjK2lVdDhKQzdxS3I4UWNMdGlETkZpQ3lvTQpPcm01TTZ5R0dxT0R2TTljUjV4TzRRdHdVd21ZNjBXZXRsOXlpOHNUYk1ubzROUW9MQ2FTSkZhVTZMYUdZYkFHCklkaDFRU2t1NjZjVFpxN3JpQi9VUjBtUDQvUFlZMWFyR2tsOHpkcE9uRWNOUEZiMU5JR1ByMmhsdEljbnEvOWkKaFVaN25oNzlBb0dCQU5UWkJpUU9JbGJzZVlTSmluTkR1RHNlWVlUOWNhVTNIOVc4SUxQb0l5UTNVRWJnK25DMQpqc1B2WUMvdDNhWXFNY09hRWdaM25TN3RjQkhuYjZPbFVDaXk2R0c1eUJKeTlUVkNuV2FSOEUxWlFpeGhwU2x1Cno0YWJLMzdKS09NcCsyamw0WnVXZ29SZGo1azcyR2VLU2xoMjhwRnNuYnh5dk1FaDRtdml3cGNUQW9HQkFQZ1EKWEg1dS9kcWNsOWlKa0diam8vZVZvdUc5RlFyK3paTjRQc1VVellFK1dGNExQL3BXM0loQ01XcHJKcjkzYUY5YQpyWDk4K3JvbnVFMm5aTHplaXVuVUVqc0RNczY4VFFzK1Z3d3g5bnFqeTVlZGNLUDc0cGRJSWxMUVZhTEZDMEhmCjR3U0tIdzhSVmI4QitFU0RVaEVwZmJvNjI2MU50L3F3RE05M3MxT2ZBb0dCQU1id0NTTk42cDZmNjN3TEtkSk4KNnpXbVViT0JSdVVLdnNDQVUzaVRjYlQ1MWhoSlV3cGhSNHl1Z1pLS21tNGlaZHpTZGlLQlRmSjhqaERONWJSbwpZS0dKaGxWQTFWVVpDVUs4Zm0xTjNmSU5pU0grS2VhdDRWMklUb0R6OXpsTEZlYVhaZVNNS25VWkZZdzlPU2lkClFwdW9vVk4yMTM4NUpsN0ZFdUdQVWh6dEFvR0FBdzM0aFpvS1NESGJjTkFvMWd1QmpsL3FWSXRHZ2c4bkt1UzEKS1NvYmFiY0IwWTZqNkFvL3h2SEJ1QVMvd21YUEJEV1NvMW8vb0NFekZNbTE1Tmw4cm1wdEd3cnNDajJwV1RVWQpUMDhpTU5wcGZHRjA3S0FDWjBoOUc3WHpDQUVUSldtRFd4LzBSL1Ezc1JWdUlESFp4ZHBUOFFJZWlHREhuaStpCnlMODVseE1DZ1lFQXY5Q3VWaERibzluY3ZQcFpYUlFoVEdyNER0UVdiL3owd0dpbXBERDdpUTJJQ0NqOGZkTlcKRTdUWU92RWw1NUNyL0lHWlRkTURkNEpIZy9GY0dmOEFKVGxsVWpvVW1MUi80Q1lOaUhia3V4VkM4NjJWRXZmegpHc0l1OGh6cXBsQ3JqV3ZPN0JqbmduY1JmdjFVR1B6Y2RJYmdCY255ZlY0SDJzUXJpTmM4MWMwPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=" \
	> $(REPO_ROOT)/test/e2e/data/cni/cilium/cilium.yaml
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
