package client

import (
	"encoding/json"
	"fmt"

	"github.com/syself/hrobot-go/models"
)

func (c *Client) FailoverGetList() ([]models.Failover, error) {
	url := c.baseURL + "/failover"
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var failoverList []models.FailoverResponse
	err = json.Unmarshal(bytes, &failoverList)
	if err != nil {
		return nil, err
	}

	var data []models.Failover
	for _, failover := range failoverList {
		data = append(data, failover.Failover)
	}

	return data, nil
}

func (c *Client) FailoverGet(ip string) (*models.Failover, error) {
	url := fmt.Sprintf(c.baseURL+"/failover/%s", ip)
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var failoverResp models.FailoverResponse
	err = json.Unmarshal(bytes, &failoverResp)
	if err != nil {
		return nil, err
	}

	return &failoverResp.Failover, nil
}
