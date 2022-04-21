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

// Package mocks defines factories that allow the usage of generated mocks in unit tests.
package mocks

import (
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

type sshFactory struct {
	rescueClient              *sshmock.Client
	osClientAfterInstallImage *sshmock.Client
	osClientAfterCloudInit    *sshmock.Client
}

// NewSSHFactory creates a new factory for SSH clients.
func NewSSHFactory(
	rescueClient *sshmock.Client,
	osClientAfterInstallImage *sshmock.Client,
	osClientAfterCloudInit *sshmock.Client,
) sshclient.Factory {
	return &sshFactory{
		rescueClient:              rescueClient,
		osClientAfterInstallImage: osClientAfterInstallImage,
		osClientAfterCloudInit:    osClientAfterCloudInit,
	}
}

var _ = sshclient.Factory(&sshFactory{})

// NewClient implements the NewClient function of the SSHFactory interface.
func (f *sshFactory) NewClient(in sshclient.Input) sshclient.Client {
	// return rescueClient when private key rescue-private-key is given. Otherwise give the os private key.
	if in.Port == 0 {
		panic("no port specified for ssh client")
	}
	if in.PrivateKey == "rescue-ssh-secret-private-key" {
		return f.rescueClient
	}
	if in.Port == 24 {
		return f.osClientAfterCloudInit
	}
	return f.osClientAfterInstallImage
}

type robotFactory struct {
	client *robotmock.Client
}

// NewRobotFactory creates a new factory for Robot clients.
func NewRobotFactory(client *robotmock.Client) robotclient.Factory {
	return &robotFactory{client: client}
}

var _ = robotclient.Factory(&robotFactory{})

// NewClient implements the NewClient function of the RobotFactory interface.
func (f *robotFactory) NewClient(creds robotclient.Credentials) robotclient.Client {
	return f.client
}
