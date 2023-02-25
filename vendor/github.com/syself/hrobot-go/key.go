package client

import (
	"encoding/json"
	"fmt"
	neturl "net/url"

	"github.com/syself/hrobot-go/models"
)

func (c *Client) KeyGetList() ([]models.Key, error) {
	url := c.baseURL + "/key"
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var keys []models.KeyResponse
	err = json.Unmarshal(bytes, &keys)
	if err != nil {
		return nil, err
	}

	var data []models.Key
	for _, key := range keys {
		data = append(data, key.Key)
	}

	return data, nil
}

func (c *Client) KeySet(input *models.KeySetInput) (*models.Key, error) {
	url := fmt.Sprintf(c.baseURL + "/key")

	formData := neturl.Values{}
	formData.Set("name", input.Name)
	formData.Set("data", input.Data)

	bytes, err := c.doPostFormRequest(url, formData)
	if err != nil {
		return nil, err
	}

	var keyResp models.KeyResponse
	err = json.Unmarshal(bytes, &keyResp)
	if err != nil {
		return nil, err
	}

	return &keyResp.Key, nil
}
