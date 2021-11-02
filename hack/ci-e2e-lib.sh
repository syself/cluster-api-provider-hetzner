#!/usr/bin/env bash

# Creates a kind cluster with the ctlptl tool https://github.com/tilt-dev/ctlptl
ctlptl_kind-cluster-with-registry () {
  
  local CLUSTER_NAME=$1
  local CLUSTER_VERSION=$2

  cat <<EOF | ctlptl apply -f -
apiVersion: ctlptl.dev/v1alpha1
kind: Registry
name: ${CLUSTER_NAME}-registry
port: 5000
---
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: kind
registry: ${CLUSTER_NAME}-registry
kindV1Alpha4Cluster:
  name: ${CLUSTER_NAME}
  nodes:
  - role: control-plane
    image: kindest/node:${CLUSTER_VERSION}
  networking:
    podSubnet: "10.244.0.0/16"
    serviceSubnet: "10.96.0.0/12"
EOF
}

