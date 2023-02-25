package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	klog "k8s.io/klog/v2"
)

type ddProtocol int

const (
	// Pre DD 4.12
	ddProtocolV1 ddProtocol = iota

	// Post DD 4.12
	ddProtocolV2 = 1
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Uses the DockerDesktop GUI+Backend protocols to control DockerDesktop.
//
// There isn't an off-the-shelf library or documented protocol we can use
// for this, so we do the best we can.
type DockerDesktopClient struct {
	backendNativeClient HTTPClient
	backendClient       HTTPClient
}

func NewDockerDesktopClient() (DockerDesktopClient, error) {
	backendNativeSocketPaths, err := dockerDesktopBackendNativeSocketPaths()
	if err != nil {
		return DockerDesktopClient{}, err
	}

	backendNativeClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var lastErr error

				// Different versions of docker use different socket paths,
				// so return all of them and connect to the first one that
				// accepts a TCP dial.
				for _, socketPath := range backendNativeSocketPaths {
					conn, err := dialDockerDesktop(socketPath)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, lastErr
			},
		},
	}
	backendClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialDockerBackend()
			},
		},
	}
	return DockerDesktopClient{
		backendNativeClient: backendNativeClient,
		backendClient:       backendClient,
	}, nil
}

func (c DockerDesktopClient) Open(ctx context.Context) error {
	var err error
	switch runtime.GOOS {

	case "windows":
		return fmt.Errorf("Cannot auto-start Docker Desktop on Windows")

	case "darwin":
		_, err = os.Stat("/Applications/Docker.app")
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Please install Docker for Desktop: https://www.docker.com/products/docker-desktop")
			}
			return err
		}
		cmd := exec.Command("open", "/Applications/Docker.app")
		err = cmd.Run()

	case "linux":
		cmd := exec.Command("systemctl", "--user", "start", "docker-desktop")
		err = cmd.Run()
	}

	if err != nil {
		return errors.Wrap(err, "starting Docker")
	}
	return nil
}

func (c DockerDesktopClient) Quit(ctx context.Context) error {
	var err error
	switch runtime.GOOS {
	case "windows":
		return fmt.Errorf("Cannot quit Docker Desktop on Windows")

	case "darwin":
		cmd := exec.Command("osascript", "-e", `quit app "Docker"`)
		err = cmd.Run()

	case "linux":
		cmd := exec.Command("systemctl", "--user", "stop", "docker-desktop")
		err = cmd.Run()
	}

	if err != nil {
		return errors.Wrap(err, "quitting Docker")
	}
	return nil
}

func (c DockerDesktopClient) ResetCluster(ctx context.Context) error {
	resp, err := c.tryRequests("reset docker-desktop kubernetes", []clientRequest{
		{
			client:  c.backendClient,
			method:  "POST",
			url:     "http://localhost/kubernetes/reset",
			headers: map[string]string{"Content-Type": "application/json"},
		},
		{
			client:  c.backendNativeClient,
			method:  "POST",
			url:     "http://localhost/kubernetes/reset",
			headers: map[string]string{"Content-Type": "application/json"},
		},
	})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c DockerDesktopClient) SettingsValues(ctx context.Context) (interface{}, error) {
	s, err := c.settings(ctx)
	if err != nil {
		return nil, err
	}
	return c.settingsForWrite(s, ddProtocolV1), nil
}

func (c DockerDesktopClient) SetSettingValue(ctx context.Context, key, newValue string) error {
	settings, err := c.settings(ctx)
	if err != nil {
		return err
	}

	changed, err := c.applySet(settings, key, newValue)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return c.writeSettings(ctx, settings)
}

// Returns true if the value changed, false if the value is unchanged.
// Returns an error if not able to set.
func (c DockerDesktopClient) applySet(settings map[string]interface{}, key, newValue string) (bool, error) {
	parts := strings.Split(key, ".")
	if len(parts) <= 1 {
		return false, fmt.Errorf("key cannot be set: %s", key)
	}

	parentKey := strings.Join(parts[:len(parts)-1], ".")
	childKey := parts[len(parts)-1]
	parentSpec, err := c.lookupMapAt(settings, parentKey)
	if err != nil {
		return false, err
	}

	// In Docker Desktop, a boolean setting can be stored in one of two formats:
	//
	// {"kubernetes": {"enabled": true}}
	// {"kubernetes": {"enabled": {"value": true}}}
	//
	// To resolve this problem, we create some intermediate variables:
	// v - the value that we're replacing
	// vParent - the map owning the value we're replacing
	// vParentKey - the key where v lives in vParent
	v, ok := parentSpec[childKey]
	if !ok {
		return false, fmt.Errorf("nothing found at DockerDesktop setting %q", key)
	}

	vParent := parentSpec
	vParentKey := childKey
	childMap, isMap := v.(map[string]interface{})
	if isMap {
		v = childMap["value"]
		vParent = childMap
		vParentKey = "value"
	}

	switch v := v.(type) {
	case bool:
		if newValue == "true" {
			vParent[vParentKey] = true
			return !v, nil
		} else if newValue == "false" {
			vParent[vParentKey] = false
			return v, nil
		}

		return false, fmt.Errorf("expected bool for setting %q, got: %s", key, newValue)

	case float64:
		newValFloat, err := strconv.ParseFloat(newValue, 64)
		if err != nil {
			return false, fmt.Errorf("expected number for setting %q, got: %s. Error: %v", key, newValue, err)
		}

		max, ok := vParent["max"].(float64)
		if ok && newValFloat > max {
			return false, fmt.Errorf("setting value %q: %s greater than max allowed (%f)", key, newValue, max)
		}
		min, ok := vParent["min"].(float64)
		if ok && newValFloat < min {
			return false, fmt.Errorf("setting value %q: %s less than min allowed (%f)", key, newValue, min)
		}

		if newValFloat != v {
			vParent[vParentKey] = newValFloat
			return true, nil
		}
		return false, nil
	case string:
		if newValue != v {
			vParent[vParentKey] = newValue
			return true, nil
		}
		return false, nil
	default:
		if key == "vm.fileSharing" {
			pathSpec := []map[string]interface{}{}
			paths := strings.Split(newValue, ",")
			for _, path := range paths {
				pathSpec = append(pathSpec, map[string]interface{}{"path": path, "cached": false})
			}

			vParent[vParentKey] = pathSpec

			// Don't bother trying to optimize this.
			return true, nil
		}
	}

	return false, fmt.Errorf("Cannot set key: %q", key)
}

func (c DockerDesktopClient) settingsForWriteJSON(settings map[string]interface{}, v ddProtocol) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := json.NewEncoder(buf).Encode(c.settingsForWrite(settings, v))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c DockerDesktopClient) writeSettings(ctx context.Context, settings map[string]interface{}) error {
	v2Body, err := c.settingsForWriteJSON(settings, ddProtocolV2)
	if err != nil {
		return errors.Wrap(err, "writing docker-desktop settings")
	}
	v1Body, err := c.settingsForWriteJSON(settings, ddProtocolV1)
	if err != nil {
		return errors.Wrap(err, "writing docker-desktop settings")
	}
	resp, err := c.tryRequests("writing docker-desktop settings", []clientRequest{
		{
			client:  c.backendClient,
			method:  "POST",
			url:     "http://localhost/app/settings",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    v2Body,
		},
		{
			client:  c.backendNativeClient,
			method:  "POST",
			url:     "http://localhost/settings",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    v1Body,
		},
	})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c DockerDesktopClient) settings(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.tryRequests("reading docker-desktop settings", []clientRequest{
		{
			client: c.backendClient,
			method: "GET",
			url:    "http://localhost/app/settings",
		},
		{
			client: c.backendNativeClient,
			method: "GET",
			url:    "http://localhost/settings",
		},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	settings := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&settings)
	if err != nil {
		return nil, errors.Wrap(err, "reading docker-desktop settings")
	}
	klog.V(8).Infof("Response body: %+v\n", settings)
	return settings, nil
}

func (c DockerDesktopClient) lookupMapAt(settings map[string]interface{}, key string) (map[string]interface{}, error) {
	parts := strings.Split(key, ".")
	current := settings
	for i, part := range parts {
		var ok bool
		val := current[part]
		current, ok = val.(map[string]interface{})
		if !ok {
			if val == nil {
				return nil, fmt.Errorf("nothing found at DockerDesktop setting %q",
					strings.Join(parts[:i+1], "."))
			}
			return nil, fmt.Errorf("expected map at DockerDesktop setting %q, got: %T",
				strings.Join(parts[:i+1], "."), val)
		}
	}
	return current, nil
}

func (c DockerDesktopClient) setK8sEnabled(settings map[string]interface{}, newVal bool) (changed bool, err error) {
	return c.applySet(settings, "vm.kubernetes.enabled", fmt.Sprintf("%v", newVal))
}

func (c DockerDesktopClient) ensureMinCPU(settings map[string]interface{}, desired int) (changed bool, err error) {
	cpusSetting, err := c.lookupMapAt(settings, "vm.resources.cpus")
	if err != nil {
		return false, err
	}

	value, ok := cpusSetting["value"].(float64)
	if !ok {
		return false, fmt.Errorf("expected number at DockerDesktop setting vm.resources.cpus.value, got: %T",
			cpusSetting["value"])
	}
	max, ok := cpusSetting["max"].(float64)
	if !ok {
		return false, fmt.Errorf("expected number at DockerDesktop setting vm.resources.cpus.max, got: %T",
			cpusSetting["max"])
	}

	if desired > int(max) {
		return false, fmt.Errorf("desired cpus (%d) greater than max allowed (%d)", desired, int(max))
	}

	if desired <= int(value) {
		return false, nil
	}

	cpusSetting["value"] = desired
	return true, nil
}

func (c DockerDesktopClient) settingsForWrite(settings interface{}, v ddProtocol) interface{} {
	settingsMap, ok := settings.(map[string]interface{})
	if !ok {
		return settings
	}

	_, hasLocked := settingsMap["locked"]
	value, hasValue := settingsMap["value"]
	if hasLocked && hasValue {
		// In the old protocol, we only sent the value back. In the new protocol,
		// we send the whole struct.
		if v == ddProtocolV1 {
			return value
		} else {
			return settingsMap
		}
	}

	if hasLocked && len(settingsMap) == 1 {
		return nil
	}

	_, hasLocks := settingsMap["locks"]
	json, hasJSON := settingsMap["json"]
	if hasLocks && hasJSON {
		return json
	}

	for key, value := range settingsMap {
		newVal := c.settingsForWrite(value, v)
		if newVal != nil {
			settingsMap[key] = newVal
		} else {
			delete(settingsMap, key)
		}
	}

	return settings
}

type clientRequest struct {
	client  HTTPClient
	method  string
	url     string
	headers map[string]string
	body    []byte
}

func status2xx(resp *http.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode <= 204
}

type withStatusCode struct {
	error
	statusCode int
}

func (w withStatusCode) Cause() error { return w.error }

// tryRequest either returns a 2xx response or an error, but not both.
// If a response is returned, the caller must close its body.
func (c DockerDesktopClient) tryRequest(label string, creq clientRequest) (*http.Response, error) {
	klog.V(7).Infof("%s %s\n", creq.method, creq.url)

	body := []byte{}
	if creq.body != nil {
		body = creq.body
		klog.V(8).Infof("Request body: %s\n", string(body))
	}
	req, err := http.NewRequest(creq.method, creq.url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, label)
	}

	for k, v := range creq.headers {
		req.Header.Add(k, v)
	}

	resp, err := creq.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, label)
	}
	if !status2xx(resp) {
		resp.Body.Close()
		return nil, withStatusCode{errors.Errorf("%s: status code %d", label, resp.StatusCode), resp.StatusCode}
	}

	return resp, nil
}

func errorPriority(err error) int {
	switch e := err.(type) {
	case withStatusCode:
		return e.statusCode / 100
	default: // give actual errors higher priority than non-2xx status codes
		return 10
	}
}

func chooseWorstError(errs []error) error {
	err := errs[0]
	prio := errorPriority(err)
	for _, e := range errs[1:] {
		if p := errorPriority(e); p > prio {
			err = e
			prio = p
		}
	}
	return err
}

// tryRequests returns the first 2xx response for the given requests, in order,
// or the "highest priority" error (based on errorPriority) from response
// errors. If a response is returned, the caller must close its body.
func (c DockerDesktopClient) tryRequests(label string, requests []clientRequest) (*http.Response, error) {
	if len(requests) == 0 {
		panic(fmt.Sprintf("%s: no requests provided", label))
	}

	errs := []error{}
	for _, creq := range requests {
		resp, err := c.tryRequest(label, creq)
		if err == nil {
			return resp, nil
		}
		errs = append(errs, err)
	}
	return nil, chooseWorstError(errs)
}
