# Copyright 2023 The Kubernetes Authors.
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

# This is the Dockerfile for the Builder Image that is used by the CI
# pipelines.
# If you make changes to this Dockerfile run `make builder-image-push`.


FROM cgr.dev/chainguard/wolfi-base:latest AS wolfi
WORKDIR /

# update: datasource=repology depName=wolfi/controller-gen versioning=apk
ARG CONTROLLER_GEN_VERSION="0.20.0-r1"

# update: datasource=repology depName=wolfi/golangci-lint versioning=apk
ARG GOLANGCI_VERSION="2.8.0-r2"

# update: datasource=repology depName=wolfi/helm versioning=apk
ARG HELM_VERSION="4.1.0-r1"

# update: datasource=repology depName=wolfi/skopeo versioning=apk
ARG SKOPEO_VERSION="1.21.0-r3"

# update: datasource=repology depName=wolfi/trivy versioning=apk
ARG TRIVY_VERSION="0.69.0-r1"

RUN apk add --no-cache \
    controller-gen=$CONTROLLER_GEN_VERSION \
    curl \
    golangci-lint=$GOLANGCI_VERSION \
    helm=$HELM_VERSION \
    skopeo=$SKOPEO_VERSION \
    trivy=$TRIVY_VERSION


# --- end of Wolfi

# Install Lychee
FROM docker.io/lycheeverse/lychee:0.14.3 AS lychee

# Install Hadolint
FROM docker.io/hadolint/hadolint:v2.12.0-alpine@sha256:3c206a451cec6d486367e758645269fd7d696c5ccb6ff59d8b03b0e45268a199 AS hadolint

############################
# Caph Build Image Base #
############################
FROM docker.io/library/golang:1.25-bookworm@sha256:38342f3e7a504bf1efad858c18e771f84b66dc0b363add7a57c9a0bbb6cf7b12

# Start of "we should move that to a previous Layer"
# We should keep the final layer clean. I guess this could all be done in the wolfi layer.

# update: datasource=github-tags depName=adrienverge/yamllint versioning=semver
ENV YAMLLINT_VERSION="v1.36.2"
# update: datasource=github-tags depName=opt-nc/yamlfixer versioning=semver
ENV YAMLFIXER_VERSION="0.9.15"

# PIPX_HOME is the directory where virtual environments for the installed tools will be created.
ENV PIPX_HOME=/opt/pipx

# PIPX_BIN_DIR is the directory where the tools installed via pipx will exist.
ENV PIPX_BIN_DIR=/usr/local/bin

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get install -qy --no-install-recommends \
    gnupg python3 python3-pip pipx \
    file zip unzip jq gettext \
    protobuf-compiler libprotobuf-dev \
    libsystemd-dev jq && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    pipx install yamllint==${YAMLLINT_VERSION} && \
    pipx install yamlfixer-opt-nc==${YAMLFIXER_VERSION} && \
    pipx ensurepath
# End of "we should move that to a previous Layer"

COPY --from=wolfi /usr/bin/controller-gen /usr/bin/controller-gen
COPY --from=wolfi /bin/golangci-lint /usr/bin/golangci-lint
COPY --from=wolfi /usr/bin/helm /usr/bin/helm
COPY --from=wolfi /usr/bin/skopeo /usr/bin/skopeo
COPY --from=wolfi /usr/bin/trivy /usr/bin/trivy

COPY --from=lychee /usr/local/bin/lychee /usr/bin/lychee
COPY --from=hadolint /bin/hadolint /usr/bin/hadolint


COPY build.sh /
RUN chmod +x /build.sh

# Fail, if one of these directories exists.
RUN set -eu; \
    for var in GOCACHE GOMODCACHE; do \
    dir="$(go env "${var}")"; \
    if [ -e "${dir}" ]; then \
    echo "Refusing to build: ${var} already exists at ${dir}. This would cause permission issues with the on-the-fly created Linux user in the container (see build.sh)." >&2; \
    exit 1; \
    fi; \
    done

#RUN rm /usr/local/go/bin/go

ENV RUNNING_IN_CONTAINER=true

# We are already in the container. Do not enter it again.
ENV BUILD_IN_CONTAINER=false

# build.sh automatically switches to the UID/GID of the host.
ENTRYPOINT ["/build.sh"]
