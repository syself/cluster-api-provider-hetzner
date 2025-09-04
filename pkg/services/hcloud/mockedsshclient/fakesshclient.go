/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mockedsshclient implements functions to create mocked ssh clients for hcloud testing
package mockedsshclient

import (
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

type mockedSSHClientFactory struct {
	sshMockClient *sshmock.Client
}

func (f *mockedSSHClientFactory) NewClient(_ sshclient.Input) sshclient.Client {
	return f.sshMockClient
}

// NewSSHFactory creates a new factory for SSH clients.
func NewSSHFactory(sshMockClient *sshmock.Client) sshclient.Factory {
	return &mockedSSHClientFactory{
		sshMockClient: sshMockClient,
	}
}
