# Copyright 2019 The Kubernetes Authors.
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

# Build the manager binary
FROM --platform=${BUILDPLATFORM} docker.io/library/golang:1.20.3@sha256:6a09d7e431f3a2e263c6e7f14f26db634f2f707b8f3efb7255a54d9ff2c6ee3a as build
ARG TARGETOS TARGETARCH

COPY . /src/cluster-api-provider-hetzner
WORKDIR /src/cluster-api-provider-hetzner
RUN --mount=type=cache,target=/root/.cache --mount=type=cache,target=/go/pkg \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -mod=vendor -ldflags "${LDFLAGS} -extldflags '-static'" \
    -o manager main.go

FROM --platform=${BUILDPLATFORM} gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /src/cluster-api-provider-hetzner/manager .
# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/manager"]