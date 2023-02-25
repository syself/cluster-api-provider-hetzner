package client

import (
	"encoding/json"
	"fmt"

	"github.com/syself/hrobot-go/models"
)

func (c *Client) RDnsGetList() ([]models.Rdns, error) {
	url := c.baseURL + "/rdns"
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var rdnsList []models.RdnsResponse
	err = json.Unmarshal(bytes, &rdnsList)
	if err != nil {
		return nil, err
	}

	var data []models.Rdns
	for _, rdns := range rdnsList {
		data = append(data, rdns.Rdns)
	}

	return data, nil
}

func (c *Client) RDnsGet(ip string) (*models.Rdns, error) {
	url := fmt.Sprintf(c.baseURL+"/rdns/%s", ip)
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var rDnsResp models.RdnsResponse
	err = json.Unmarshal(bytes, &rDnsResp)
	if err != nil {
		return nil, err
	}

	return &rDnsResp.Rdns, nil
}
