package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/syself/hrobot-go/models"
)

const (
	baseURL   string = "https://robot-ws.your-server.de"
	version          = "0.2.6"
	userAgent        = "hrobot-client/" + version
)

type Client struct {
	Username   string
	Password   string
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

func NewBasicAuthClient(username, password string) RobotClient {
	return &Client{
		Username:   username,
		Password:   password,
		baseURL:    baseURL,
		userAgent:  userAgent,
		httpClient: &http.Client{},
	}
}

func NewBasicAuthClientWithCustomHttpClient(username, password string, httpClient *http.Client) RobotClient {
	return &Client{
		Username:   username,
		Password:   password,
		baseURL:    baseURL,
		userAgent:  userAgent,
		httpClient: httpClient,
	}
}

func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

func (c *Client) SetUserAgent(userAgent string) {
	c.userAgent = userAgent
}

func (c *Client) GetVersion() string {
	return version
}

func (c *Client) ValidateCredentials() error {
	if _, err := c.doGetRequest(c.baseURL); err != nil {
		return err
	}
	return nil
}

func (c *Client) SetCredentials(username, password string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	c.Username = username
	c.Password = password
	return nil
}

func (c *Client) doGetRequest(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	bytes, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (c *Client) doDeleteRequest(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}

	bytes, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (c *Client) doPostFormRequest(url string, formData url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	bytes, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", c.userAgent)
	req.SetBasicAuth(c.Username, c.Password)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		return nil, errorFromResponse(resp, body)
	}
	return body, nil
}

func errorFromResponse(resp *http.Response, body []byte) (reterr error) {
	var errorResponse models.ErrorResponse
	reterr = fmt.Errorf("server responded with status code %v", resp.StatusCode)
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return
	}
	if errorResponse.Error.Code == "" && errorResponse.Error.Message == "" {
		return
	}
	return errorResponse.Error
}
