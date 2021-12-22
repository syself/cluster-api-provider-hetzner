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

// Package robotclient contains the interface to speak to Hetzner robot API.
package robotclient

import (
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	hrobot "github.com/syself/hrobot-go"
	"github.com/syself/hrobot-go/models"
)

// Client collects all methods used by the controller in the robot API.
type Client interface {
	ValidateCredentials() error

	RebootBMServer(int, infrav1.RebootType) (*models.ResetPost, error)
	ListBMServers() ([]models.Server, error)
	SetBMServerName(int, string) (*models.Server, error)
	GetBMServer(int) (*models.Server, error)
	ListSSHKeys() ([]models.Key, error)
	SetSSHKey(name, publickey string) (*models.Key, error)
	SetBootRescue(id int, fingerprint string) (*models.Rescue, error)
	GetBootRescue(id int) (*models.Rescue, error)
	DeleteBootRescue(id int) (*models.Rescue, error)
	GetReboot(int) (*models.Reset, error)
}

// Factory is the interface for creating new Client objects.
type Factory interface {
	NewClient(Credentials) Client
}

// NewClient creates new HCloud clients.
func (f *factory) NewClient(creds Credentials) Client {
	return &realHetznerRobotClient{
		client: hrobot.NewBasicAuthClient(creds.Username, creds.Password),
	}
}

type factory struct{}

var _ = Factory(&factory{})

// NewFactory creates a new factory for HCloud clients.
func NewFactory() Factory {
	return &factory{}
}

var _ = Client(&realHetznerRobotClient{})

type realHetznerRobotClient struct {
	client   hrobot.RobotClient
	userName string
	password string
}

func (c *realHetznerRobotClient) UserName() string {
	return c.userName
}

func (c *realHetznerRobotClient) Password() string {
	return c.password
}

func (c *realHetznerRobotClient) ValidateCredentials() error {
	return c.client.ValidateCredentials()
}

func (c *realHetznerRobotClient) RebootBMServer(id int, rebootType infrav1.RebootType) (*models.ResetPost, error) {
	return c.client.ResetSet(id, &models.ResetSetInput{Type: string(rebootType)})
}

func (c *realHetznerRobotClient) ListBMServers() ([]models.Server, error) {
	return c.client.ServerGetList()
}

func (c *realHetznerRobotClient) ListBMKeys() ([]models.Key, error) {
	return c.client.KeyGetList()
}

func (c *realHetznerRobotClient) SetBMServerName(id int, name string) (*models.Server, error) {
	return c.client.ServerSetName(id, &models.ServerSetNameInput{Name: name})
}

func (c *realHetznerRobotClient) GetBMServer(id int) (*models.Server, error) {
	return c.client.ServerGet(id)
}

func (c *realHetznerRobotClient) ListSSHKeys() ([]models.Key, error) {
	return c.client.KeyGetList()
}

func (c *realHetznerRobotClient) SetSSHKey(name, publicKey string) (*models.Key, error) {
	return c.client.KeySet(&models.KeySetInput{Name: name, Data: publicKey})
}

func (c *realHetznerRobotClient) SetBootRescue(id int, fingerprint string) (*models.Rescue, error) {
	return c.client.BootRescueSet(id, &models.RescueSetInput{OS: "linux", AuthorizedKey: fingerprint})
}

func (c *realHetznerRobotClient) GetBootRescue(id int) (*models.Rescue, error) {
	return c.client.BootRescueGet(id)
}

func (c *realHetznerRobotClient) DeleteBootRescue(id int) (*models.Rescue, error) {
	return c.client.BootRescueDelete(id)
}

func (c *realHetznerRobotClient) GetReboot(id int) (*models.Reset, error) {
	return c.client.ResetGet(id)
}
