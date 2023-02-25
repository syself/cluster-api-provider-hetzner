package client

import (
	"encoding/json"
	"fmt"
	neturl "net/url"
	"strconv"

	"github.com/syself/hrobot-go/models"
)

func (c *Client) BootRescueGet(id int) (*models.Rescue, error) {
	url := fmt.Sprintf(c.baseURL+"/boot/%v/rescue", id)
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var rescueResp models.RescueResponse
	err = json.Unmarshal(bytes, &rescueResp)
	if err != nil {
		return nil, err
	}

	return &rescueResp.Rescue, nil
}

func (c *Client) BootRescueSet(id int, input *models.RescueSetInput) (*models.Rescue, error) {
	url := fmt.Sprintf(c.baseURL+"/boot/%v/rescue", id)

	formData := neturl.Values{}
	formData.Set("os", input.OS)
	if input.Arch > 0 {
		formData.Set("arch", strconv.Itoa(input.Arch))
	}
	if len(input.AuthorizedKey) > 0 {
		formData.Set("authorized_key", input.AuthorizedKey)
	}

	bytes, err := c.doPostFormRequest(url, formData)
	if err != nil {
		return nil, err
	}

	var rescueResp models.RescueResponse
	err = json.Unmarshal(bytes, &rescueResp)
	if err != nil {
		return nil, err
	}

	return &rescueResp.Rescue, nil
}

func (c *Client) BootRescueDelete(id int) (*models.Rescue, error) {
	url := fmt.Sprintf(c.baseURL+"/boot/%v/rescue", id)
	bytes, err := c.doDeleteRequest(url)
	if err != nil {
		return nil, err
	}

	var rescueResp models.RescueResponse
	err = json.Unmarshal(bytes, &rescueResp)
	if err != nil {
		return nil, err
	}

	return &rescueResp.Rescue, nil
}

func (c *Client) BootLinuxGet(id int) (*models.Linux, error) {
	url := fmt.Sprintf(c.baseURL+"/boot/%v/linux", id)
	bytes, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}

	var linuxResp models.LinuxResponse
	err = json.Unmarshal(bytes, &linuxResp)
	if err != nil {
		return nil, err
	}

	return &linuxResp.Linux, nil
}

func (c *Client) BootLinuxSet(id int, input *models.LinuxSetInput) (*models.Linux, error) {
	url := fmt.Sprintf(c.baseURL+"/boot/%v/linux", id)

	formData := neturl.Values{}
	formData.Set("dist", input.Dist)
	if input.Arch > 0 {
		formData.Set("arch", strconv.Itoa(input.Arch))
	}
	if len(input.Lang) > 0 {
		formData.Set("lang", input.Lang)
	}
	if len(input.AuthorizedKey) > 0 {
		formData.Set("authorized_key", input.AuthorizedKey)
	}

	bytes, err := c.doPostFormRequest(url, formData)
	if err != nil {
		return nil, err
	}

	var linuxResp models.LinuxResponse
	err = json.Unmarshal(bytes, &linuxResp)
	if err != nil {
		return nil, err
	}

	return &linuxResp.Linux, nil
}

func (c *Client) BootLinuxDelete(id int) (*models.Linux, error) {
	url := fmt.Sprintf(c.baseURL+"/boot/%v/linux", id)
	bytes, err := c.doDeleteRequest(url)
	if err != nil {
		return nil, err
	}

	var linuxResp models.LinuxResponse
	err = json.Unmarshal(bytes, &linuxResp)
	if err != nil {
		return nil, err
	}
	return &linuxResp.Linux, nil
}
