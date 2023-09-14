package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/syself/hrobot-go/models"
)

const baseURL string = "https://robot-ws.your-server.de"
const version = "0.2.4"
const userAgent = "hrobot-client/" + version

type Client struct {
	Username  string
	Password  string
	baseURL   string
	userAgent string
}

func NewBasicAuthClient(username, password string) RobotClient {
	return &Client{
		Username:  username,
		Password:  password,
		baseURL:   baseURL,
		userAgent: userAgent,
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

// NewLoggingClient returns a new http.Client that logs all requests.
func NewLoggingClient() *http.Client {
	return &http.Client{
		Transport: &loggingTransport{
			transport: http.DefaultTransport,
		},
	}
}

// loggingTransport is a custom http.RoundTripper that logs requests.
type loggingTransport struct {
	transport http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Dump the request for logging purposes
	fmt.Printf("http callllll %s %s %s\n", req.Method, req.RequestURI, req.URL.String())
	// Call the wrapped transport's RoundTrip method to actually send the request
	return t.transport.RoundTrip(req)
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", c.userAgent)
	req.SetBasicAuth(c.Username, c.Password)
	client := NewLoggingClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
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
