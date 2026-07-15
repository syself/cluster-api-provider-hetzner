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
	"net/http"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	hrobot "github.com/syself/hrobot-go"
	"github.com/syself/hrobot-go/models"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
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

// LoggingTransport is a struct for creating new logger for robot API.
type LoggingTransport struct {
	roundTripper http.RoundTripper
	log          logr.Logger
}

var replaceHex = regexp.MustCompile(`0x[0123456789abcdef]+`)

// replaceDigits collapses numeric path segments (server IDs, action IDs, ...) so the
// endpoint label on robotRequestsTotal/robotRequestDuration stays low-cardinality
// (e.g. "/server/1234" becomes "/server/N").
var replaceDigits = regexp.MustCompile(`\d+`)

func normalizeRobotPath(path string) string {
	return replaceDigits.ReplaceAllString(path, "N")
}

// robotInFlightRequests, robotRequestsTotal and robotRequestDuration mirror the generic
// per-endpoint instrumentation hcloud-go provides out of the box for the HCloud API (see
// hcloud.WithInstrumentation), so the Robot API gets the same always-on visibility.
var robotInFlightRequests = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "caph_robot_in_flight_requests",
	Help: "A gauge of in-flight requests to the Hetzner Robot API.",
})

var robotRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "caph_robot_requests_total",
	Help: "A counter for requests to the Hetzner Robot API, labeled by status code, method and endpoint.",
}, []string{"code", "method", "endpoint"})

var robotRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "caph_robot_request_duration_seconds",
	Help:    "A histogram of request latencies to the Hetzner Robot API.",
	Buckets: prometheus.DefBuckets,
}, []string{"method"})

func init() {
	metrics.Registry.MustRegister(robotInFlightRequests, robotRequestsTotal, robotRequestDuration)
	metrics.Registry.MustRegister(getBMServerCallsTotal, getBMServerCallsByStateTotal)
	metrics.Registry.MustRegister(apiCallsByCallerTotal)
}

// RoundTrip is used for logging api calls to robot API.
func (lt *LoggingTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	stack := replaceHex.ReplaceAllString(string(debug.Stack()), "0xX")

	robotInFlightRequests.Inc()
	start := time.Now()
	resp, err = lt.roundTripper.RoundTrip(req)
	robotRequestDuration.WithLabelValues(strings.ToLower(req.Method)).Observe(time.Since(start).Seconds())
	robotInFlightRequests.Dec()

	if err != nil {
		lt.log.V(1).Info("hetzner robot API. Error.", "err", err, "method", req.Method, "url", req.URL, "stack", stack)
		return resp, err
	}

	robotRequestsTotal.WithLabelValues(strconv.Itoa(resp.StatusCode), strings.ToLower(req.Method), normalizeRobotPath(req.URL.Path)).Inc()
	lt.log.V(1).Info("hetzner robot API called.", "statusCode", resp.StatusCode, "method", req.Method, "url", req.URL, "stack", stack)
	return resp, nil
}

// MetricPerServerID adds a server_id label to Robot API call metrics if true. Mirrors
// hcloudclient.MetricPerServerID; both are wired to the same --metric-per-server-id flag.
// This adds one Prometheus time series per distinct server ID, so it must not be enabled
// permanently on a long-lived production manager.
var MetricPerServerID bool

// getBMServerCallsTotal counts calls to GetBMServer, labeled by server ID. Only incremented
// when MetricPerServerID is true. Used to measure API call volume during e2e runs (see issue #2163).
var getBMServerCallsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "caph_robot_getbmserver_calls_total",
	Help: "Number of GetBMServer calls to the Hetzner Robot API, labeled by server ID. Only populated when --metric-per-server-id is set.",
}, []string{"server_id"})

// getBMServerCallsByStateTotal counts GetBMServer calls labeled by the host's ProvisioningState.
// Unlike getBMServerCallsTotal, this is always on: ProvisioningState has a small, fixed set of
// values, so cardinality stays bounded regardless of fleet size. Used to identify which
// reconcile phase drives the most GetBMServer calls (see issue #2163).
var getBMServerCallsByStateTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "caph_robot_getbmserver_calls_by_state_total",
	Help: "Number of GetBMServer calls to the Hetzner Robot API, labeled by the host's ProvisioningState.",
}, []string{"provisioning_state"})

// RecordGetBMServerCallByState increments the per-ProvisioningState GetBMServer call counter.
// Callers that know which ProvisioningState triggered a GetBMServer call should call this
// alongside the GetBMServer call itself.
func RecordGetBMServerCallByState(state string) {
	getBMServerCallsByStateTotal.WithLabelValues(state).Inc()
}

// modulePrefix is trimmed from runtime function names so caller labels stay short, e.g.
// "pkg/services/baremetal/host.(*Service).actionPreparing" instead of the fully qualified
// import path. Mirrors hcloudclient.modulePrefix.
const modulePrefix = "github.com/syself/cluster-api-provider-hetzner/"

// apiCallsByCallerTotal counts every Robot API client method call, labeled by the calling caph Go
// function (caller) and the client method name (method). Cardinality is bounded by the number of
// call sites in the caph source code, not by fleet size, so it is safe to run permanently. Use
// this to find which caph code triggers a given Robot API call (see docs/caph/04-developers/07-third-party-api-metrics.md).
var apiCallsByCallerTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "caph_robot_api_calls_by_caller_total",
	Help: "Number of Robot API client method calls, labeled by the calling caph Go function (caller) and the robot client method name (method).",
}, []string{"caller", "method"})

// recordAPICallByCaller records that method was called, labeled by the caph Go function that
// called it. It must be called directly from the realHetznerRobotClient method it instruments:
// skip depth 2 in runtime.Caller means 0 is recordAPICallByCaller itself, 1 is the
// realHetznerRobotClient method, and 2 is the caph code that called that method.
func recordAPICallByCaller(method string) {
	caller := "unknown"
	if pc, _, _, ok := runtime.Caller(2); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			caller = strings.TrimPrefix(fn.Name(), modulePrefix)
		}
	}
	apiCallsByCallerTotal.WithLabelValues(caller, method).Inc()
}

// NewClient creates new robot clients.
func (f *factory) NewClient(creds Credentials) Client {
	client := &http.Client{
		Transport: &LoggingTransport{
			roundTripper: http.DefaultTransport,
			log:          ctrl.Log.WithName("robot-api"),
		},
	}
	return &realHetznerRobotClient{
		client: hrobot.NewBasicAuthClientWithCustomHttpClient(creds.Username, creds.Password, client),
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
	recordAPICallByCaller("ValidateCredentials")
	return c.client.ValidateCredentials()
}

func (c *realHetznerRobotClient) RebootBMServer(id int, rebootType infrav1.RebootType) (*models.ResetPost, error) {
	recordAPICallByCaller("RebootBMServer")
	return c.client.ResetSet(id, &models.ResetSetInput{Type: string(rebootType)})
}

func (c *realHetznerRobotClient) ListBMServers() ([]models.Server, error) {
	recordAPICallByCaller("ListBMServers")
	return c.client.ServerGetList()
}

func (c *realHetznerRobotClient) ListBMKeys() ([]models.Key, error) {
	recordAPICallByCaller("ListBMKeys")
	return c.client.KeyGetList()
}

func (c *realHetznerRobotClient) SetBMServerName(id int, name string) (*models.Server, error) {
	recordAPICallByCaller("SetBMServerName")
	return c.client.ServerSetName(id, &models.ServerSetNameInput{Name: name})
}

func (c *realHetznerRobotClient) GetBMServer(id int) (*models.Server, error) {
	recordAPICallByCaller("GetBMServer")
	if MetricPerServerID {
		getBMServerCallsTotal.WithLabelValues(strconv.Itoa(id)).Inc()
	}
	return c.client.ServerGet(id)
}

func (c *realHetznerRobotClient) ListSSHKeys() ([]models.Key, error) {
	recordAPICallByCaller("ListSSHKeys")
	return c.client.KeyGetList()
}

func (c *realHetznerRobotClient) SetSSHKey(name, publicKey string) (*models.Key, error) {
	recordAPICallByCaller("SetSSHKey")
	return c.client.KeySet(&models.KeySetInput{Name: name, Data: publicKey})
}

func (c *realHetznerRobotClient) SetBootRescue(id int, fingerprint string) (*models.Rescue, error) {
	recordAPICallByCaller("SetBootRescue")
	return c.client.BootRescueSet(id, &models.RescueSetInput{OS: "linux", AuthorizedKey: fingerprint})
}

func (c *realHetznerRobotClient) GetBootRescue(id int) (*models.Rescue, error) {
	recordAPICallByCaller("GetBootRescue")
	return c.client.BootRescueGet(id)
}

func (c *realHetznerRobotClient) DeleteBootRescue(id int) (*models.Rescue, error) {
	recordAPICallByCaller("DeleteBootRescue")
	return c.client.BootRescueDelete(id)
}

func (c *realHetznerRobotClient) GetReboot(id int) (*models.Reset, error) {
	recordAPICallByCaller("GetReboot")
	return c.client.ResetGet(id)
}
