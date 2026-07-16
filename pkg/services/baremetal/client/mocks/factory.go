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
	"sync"

	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// SSHFactory is a test SSH client factory whose mock clients can be hot-swapped between
// tests via SetClients without replacing the factory pointer on the reconciler. A
// sync.RWMutex ensures that concurrent NewClient calls (from in-flight reconcile
// goroutines) and SetClients calls (from the test reset path) never race.
type SSHFactory struct {
	mu                        sync.RWMutex
	rescueClient              *sshmock.Client
	osClientAfterInstallImage *sshmock.Client
	osClientAfterCloudInit    *sshmock.Client
}

var _ sshclient.Factory = &SSHFactory{}

// NewSSHFactory creates a new SSHFactory primed with the given clients.
func NewSSHFactory(
	rescueClient *sshmock.Client,
	osClientAfterInstallImage *sshmock.Client,
	osClientAfterCloudInit *sshmock.Client,
) *SSHFactory {
	f := &SSHFactory{}
	f.SetClients(rescueClient, osClientAfterInstallImage, osClientAfterCloudInit)
	return f
}

// SetClients atomically replaces the mock clients returned by NewClient (defined below). The
// finish() func returned by ResetAndInitNamespace calls this after all On() expectations are
// registered, so any goroutine that calls NewClient after this returns sees a fully-configured
// mock. f.mu guards only the SSHFactory fields against data races between SetClients and
// NewClient; the ReconcileGate in the reconcilers is what prevents Reconcile from running
// while mocks are being swapped between tests.
func (f *SSHFactory) SetClients(
	rescue *sshmock.Client,
	osAfterInstallImage *sshmock.Client,
	osAfterCloudInit *sshmock.Client,
) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rescueClient = rescue
	f.osClientAfterInstallImage = osAfterInstallImage
	f.osClientAfterCloudInit = osAfterCloudInit
}

// NewClient implements sshclient.Factory. f.mu.RLock allows multiple goroutines to call
// NewClient concurrently while guarding the client fields against a concurrent SetClients write.
func (f *SSHFactory) NewClient(in sshclient.Input) sshclient.Client {
	f.mu.RLock()
	defer f.mu.RUnlock()
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

// Evict implements sshclient.Factory. There is no connection pool to evict from in tests.
func (f *SSHFactory) Evict(_ string) {}

type robotFactory struct {
	client *robotmock.Client
}

// NewRobotFactory creates a new factory for Robot clients.
func NewRobotFactory(client *robotmock.Client) robotclient.Factory {
	return &robotFactory{client: client}
}

var _ = robotclient.Factory(&robotFactory{})

// NewClient implements the NewClient function of the RobotFactory interface.
func (f *robotFactory) NewClient(_ robotclient.Credentials) robotclient.Client {
	return f.client
}
