#!/bin/bash

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

	echo -n $HETZNER_SSH_PUB > tmp_ssh_pub
	echo -n $HETZNER_SSH_PRIV > tmp_ssh_priv
	base64 -d tmp_ssh_priv > tmp_ssh_priv_enc
	base64 -d tmp_ssh_pub > tmp_ssh_pub_enc
	kubectl create secret generic robot-ssh --from-literal=sshkey-name=ci --from-file=ssh-privatekey=tmp_ssh_priv_enc --from-file=ssh-publickey=tmp_ssh_pub_enc --dry-run=client -o yaml > data/infrastructure-hetzner/v1beta1/cluster-template-hetzner-secret.yaml