package scope

import (
	"context"

	hrobot "github.com/syself/hrobot-go"
	"github.com/syself/hrobot-go/models"
)

// HetznerRobotClient collects all methods used by the controller in the hrobot cloud API
type HetznerRobotClient interface {
	UserName() string
	Password() string
	ResetBMServer(int, string) (*models.ResetPost, error)
	ListBMServers() ([]models.Server, error)
	ActivateRescue(int, string) (*models.Rescue, error)
	ListBMKeys() ([]models.Key, error)
	SetBMServerName(int, string) (*models.Server, error)
	GetBMServer(int) (*models.Server, error)
}

type HetznerRobotClientFactory func(context.Context) (HetznerRobotClient, error)

var _ HetznerRobotClient = &realHetznerRobotClient{}

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

func (c *realHetznerRobotClient) ResetBMServer(id int, resetType string) (*models.ResetPost, error) {
	return c.client.ResetSet(id, &models.ResetSetInput{Type: resetType})
}

func (c *realHetznerRobotClient) ListBMServers() ([]models.Server, error) {
	return c.client.ServerGetList()
}

func (c *realHetznerRobotClient) ActivateRescue(id int, key string) (*models.Rescue, error) {
	return c.client.BootRescueSet(id, &models.RescueSetInput{OS: "linux", Arch: 64, AuthorizedKey: key})
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
