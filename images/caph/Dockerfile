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
FROM --platform=${BUILDPLATFORM} docker.io/library/golang:1.23.7-bullseye@sha256:15c182db16ffc2d7abc4db2b09f468129b9adec2f7c2ddaa0c909d8cb06f13b5 \
    AS build
ARG TARGETOS TARGETARCH

COPY . /src/cluster-api-provider-hetzner
WORKDIR /src/cluster-api-provider-hetzner
RUN --mount=type=cache,target=/root/.cache --mount=type=cache,target=/go/pkg \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -mod=vendor -trimpath -ldflags "-s -w ${LDFLAGS} -extldflags '-static'" \
    -o manager main.go

FROM --platform=${BUILDPLATFORM} gcr.io/distroless/static:nonroot
WORKDIR /
COPY data/hetzner-installimage-v1.0.7.tgz /installimage.tgz
COPY --from=build /src/cluster-api-provider-hetzner/manager .
# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/manager"]
