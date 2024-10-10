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

# Install Lychee
FROM docker.io/library/alpine:3.20.3@sha256:beefdbd8a1da6d2915566fde36db9db0b524eb737fc57cd1367effd16dc0d06d AS lychee
# update: datasource=github-tags depName=lycheeverse/lychee versioning=semver
ENV LYCHEE_VERSION="v0.15.1"
# hadolint ignore=DL3018
RUN apk add --no-cache curl && \
    curl -L -o /tmp/lychee-${LYCHEE_VERSION}.tgz https://github.com/lycheeverse/lychee/releases/download/${LYCHEE_VERSION}/lychee-${LYCHEE_VERSION}-x86_64-unknown-linux-gnu.tar.gz && \
    tar -xz -C /tmp -f /tmp/lychee-${LYCHEE_VERSION}.tgz && \
    mv /tmp/lychee /usr/bin/lychee && \
    rm -rf /tmp/linux-amd64 /tmp/lychee-${LYCHEE_VERSION}.tgz

# Install Golang CI Lint
FROM docker.io/library/alpine:3.20.3@sha256:beefdbd8a1da6d2915566fde36db9db0b524eb737fc57cd1367effd16dc0d06d AS golangci
# update: datasource=github-tags depName=golangci/golangci-lint versioning=semver
ENV GOLANGCI_VERSION="v1.61.0"
WORKDIR /
# hadolint ignore=DL3018,DL4006
RUN apk add --no-cache curl && \
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s ${GOLANGCI_VERSION}

# Install Hadolint
FROM docker.io/hadolint/hadolint:v2.12.0-alpine@sha256:3c206a451cec6d486367e758645269fd7d696c5ccb6ff59d8b03b0e45268a199 AS hadolint

# Install Trivy
FROM docker.io/aquasec/trivy:0.56.2@sha256:26245f364b6f5d223003dc344ec1eb5eb8439052bfecb31d79aeba0c74344b3a AS trivy

############################
# Caph Build Image Base #
############################
FROM docker.io/library/golang:1.22.6-bullseye@sha256:3bc1984c6725fdc2ac075004a2aa52131cccc95d2d69fd7b934d343b669e0aa7

# update: datasource=repology depName=debian_11/skopeo versioning=loose
ENV SKOPEO_VERSION="1.2.2+dfsg1-1+b6"
# update: datasource=github-tags depName=adrienverge/yamllint versioning=semver
ENV YAMLLINT_VERSION="v1.35.1"
# update: datasource=github-tags depName=opt-nc/yamlfixer versioning=semver
ENV YAMLFIXER_VERSION="0.9.15"

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get install -qy --no-install-recommends \
    gnupg python3 python3-pip \
    file zip unzip jq gettext \
    skopeo=${SKOPEO_VERSION} \
    protobuf-compiler libprotobuf-dev \
    libsystemd-dev jq && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    pip install --no-cache-dir \
    yamllint==${YAMLLINT_VERSION} \
    yamlfixer-opt-nc==${YAMLFIXER_VERSION}

COPY --from=lychee /usr/bin/lychee /usr/bin/lychee
COPY --from=golangci /bin/golangci-lint /usr/local/bin
COPY --from=hadolint /bin/hadolint /usr/bin/hadolint
COPY --from=trivy /usr/local/bin/trivy /usr/bin/trivy

ENV GOCACHE=/go/cache

COPY build.sh /
RUN chmod +x /build.sh
ENTRYPOINT ["/build.sh"]
