package client

import (
	"encoding/json"

	"github.com/syself/hrobot-go/models"
)

func (c *Client) IPGetList() ([]models.IP, error) {
	url := c.baseURL + "/ip"
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var ips []models.IPResponse
	err = json.Unmarshal(bytes, &ips)
	if err != nil {
		return nil, err
	}

	var data []models.IP
	for _, ip := range ips {
		data = append(data, ip.IP)
	}

	return data, nil
}
