#!/bin/bash

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

function print_heading(){
    blue='\033[0;34m'
    nc='\033[0m' # No Color
    echo -e "${blue}${1}${nc}"
}

print_heading Hetzner

kubectl get clusters -A

print_heading machines:

kubectl get machines -A

print_heading hcloudmachine:

kubectl get hcloudmachine -A

print_heading hetznerbaremetalmachine:

kubectl get hetznerbaremetalmachine -A

print_heading events:

kubectl get events -A --sort-by=metadata.creationTimestamp | tail -8

print_heading logs:

./hack/tail-caph-controller-logs.sh

echo

ip=$(kubectl get machine -l cluster.x-k8s.io/control-plane  -o  jsonpath='{.items[0].status.addresses[?(@.type=="ExternalIP")].address}' |  grep -oP '[0-9.]{8,}')
if [ -z "$ip" ]; then
    echo "❌ Could not get IP of control-plane"
    exit 1
fi


SSH_PORT=22
if netcat -w 2 -z "$ip" $SSH_PORT; then
    echo "👌 $ip ssh port $SSH_PORT is reachable"
else
    echo "❌ ssh port $SSH_PORT for $ip is not reachable"
    exit
fi

echo

./hack/get-kubeconfig-of-workload-cluster.sh

kubeconfig=".workload-cluster-kubeconfig.yaml"


echo "KUBECONFIG=$kubeconfig kubectl cluster-info"
if KUBECONFIG=$kubeconfig kubectl cluster-info >/dev/null 2>&1; then
    echo "👌 cluster is reachable"
else
    echo "❌ cluster is not reachable"
    exit
fi

echo

deployment=$(KUBECONFIG=$kubeconfig kubectl get -n kube-system deployment | grep -P 'ccm-(hetzner|hcloud)' | cut -d' ' -f1)
if [ -z "$deployment" ]; then
    echo "❌ ccm not installed?"
else
    echo  "👌 ccm installed:"
    KUBECONFIG=$kubeconfig kubectl get -n kube-system deployment $deployment
fi