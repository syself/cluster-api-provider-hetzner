/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta2

import (
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// LoadBalancerAlgorithmType defines the Algorithm type.
// +kubebuilder:validation:Enum=round_robin;least_connections
type LoadBalancerAlgorithmType string

const (

	// LoadBalancerAlgorithmTypeRoundRobin sends requests to the targets in turn. It is the default for the
	// control plane load balancer.
	LoadBalancerAlgorithmTypeRoundRobin = LoadBalancerAlgorithmType("round_robin")

	// LoadBalancerAlgorithmTypeLeastConnections sends requests to the target with the fewest open connections.
	LoadBalancerAlgorithmTypeLeastConnections = LoadBalancerAlgorithmType("least_connections")
)

// LoadBalancerTargetType defines the target type.
// +kubebuilder:validation:Enum=server;ip
type LoadBalancerTargetType string

const (

	// LoadBalancerTargetTypeServer adds an HCloud server as a load balancer target.
	LoadBalancerTargetTypeServer = LoadBalancerTargetType("server")

	// LoadBalancerTargetTypeIP adds an IP address as a load balancer target. CAPH uses it for bare metal servers.
	LoadBalancerTargetTypeIP = LoadBalancerTargetType("ip")
)

// LoadBalancerTargetAddressFamily defines which addresses of a bare metal server are
// attached as targets of a load balancer.
// +kubebuilder:validation:Enum=ipv4;ipv6;dualstack
type LoadBalancerTargetAddressFamily string

const (
	// LoadBalancerTargetAddressFamilyIPv4 attaches only the IPv4 address of a bare metal
	// server.
	LoadBalancerTargetAddressFamilyIPv4 = LoadBalancerTargetAddressFamily("ipv4")

	// LoadBalancerTargetAddressFamilyIPv6 attaches only the IPv6 address of a bare metal
	// server.
	LoadBalancerTargetAddressFamilyIPv6 = LoadBalancerTargetAddressFamily("ipv6")

	// LoadBalancerTargetAddressFamilyDualStack attaches both the IPv4 and the IPv6 address
	// of a bare metal server, as two separate targets. This is the default, see
	// LoadBalancerSpec.TargetAddressFamily.
	LoadBalancerTargetAddressFamilyDualStack = LoadBalancerTargetAddressFamily("dualstack")
)

// HCloudAlgorithmType converts LoadBalancerAlgorithmType to hcloud type.
func (algorithmType *LoadBalancerAlgorithmType) HCloudAlgorithmType() hcloud.LoadBalancerAlgorithmType {
	switch *algorithmType {
	case LoadBalancerAlgorithmTypeLeastConnections:
		return hcloud.LoadBalancerAlgorithmTypeLeastConnections
	case LoadBalancerAlgorithmTypeRoundRobin:
		return hcloud.LoadBalancerAlgorithmTypeRoundRobin
	}
	return hcloud.LoadBalancerAlgorithmType("")
}

// HetznerSSHKeys defines the global cluster-wide SSHKeys for HetznerCluster. It serves as the default for machines as well.
// +kubebuilder:validation:MinProperties=1
type HetznerSSHKeys struct {
	// hcloud defines the SSH keys used for hcloud.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	HCloud []SSHKey `json:"hcloud,omitempty"`
	// rescueSecretRef defines the reference to the secret where the SSH key for the rescue system is stored.
	// When it is not set, no secret with a rescue SSH key is configured.
	// +optional
	RescueSecretRef *SSHSecretRef `json:"rescueSecretRef,omitempty"`
}

// SSHKey defines the SSHKey for HCloud.
type SSHKey struct {
	// name defines the name of the SSH key.
	// The limit is the largest name limit that Hetzner documents, because Hetzner does not document a limit for this name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name,omitempty"`
	// fingerprint defines the fingerprint of the SSH key - added by the controller.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Fingerprint string `json:"fingerprint,omitempty"`
}

// IsDefined returns true if the SSHKey is set.
func (k *SSHKey) IsDefined() bool {
	if k == nil {
		return false
	}
	return k.Name != "" || k.Fingerprint != ""
}

// HCloudMachineType defines the HCloud Machine type.
// +kubebuilder:validation:MaxLength=64
type HCloudMachineType string

// ResourceLifecycle configures the lifecycle of a resource.
type ResourceLifecycle string

// HCloudPlacementGroupSpec defines a PlacementGroup.
type HCloudPlacementGroupSpec struct {
	// name is the name of the placement group. The placement group in HCloud gets the name
	// "<HetznerCluster name>-<name>".
	// The limit is the largest name limit that Hetzner documents, because Hetzner does not document a limit for this name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name,omitempty"`

	// type is the type of the placement group. The only allowed value is spread.
	// +optional
	// +kubebuilder:validation:Enum=spread
	// +kubebuilder:default=spread
	Type string `json:"type,omitempty"`
}

// HCloudPlacementGroupStatus returns the status of a Placementgroup.
type HCloudPlacementGroupStatus struct {
	// id is the ID of the placement group in HCloud.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ID int64 `json:"id,omitempty"`

	// servers are the IDs of the HCloud servers in the placement group.
	// The limit is more than twice the Hetzner limit of 10 servers in a placement group. A higher Hetzner limit then does not break status updates.
	// +optional
	// +listType=set
	// +kubebuilder:validation:MaxItems=32
	Server []int64 `json:"servers,omitempty"`

	// name is the placement group name. It is required because status.hcloudPlacementGroups is a
	// list-map keyed on name, and a list-map key must be a required (or defaulted) property.
	// The limit is the largest name limit that Hetzner documents, because Hetzner does not document a limit for this name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name,omitempty"`

	// type is the type of the placement group in HCloud, for example spread.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Type string `json:"type,omitempty"`
}

// HetznerSecretRef defines all the names of the secret and the relevant keys needed to access Hetzner API.
type HetznerSecretRef struct {
	// name defines the name of the secret. The controller reads the credential from the
	// management-cluster secret with this name and creates a workload-cluster secret with the same
	// name. The upstream hcloud-ccm helm chart expects the secret name "hcloud", while the Syself
	// ccm fork defaults to "hetzner".
	//
	// To facilitate migrations to the upstream hcloud-ccm, it is ensured that the secret it expects
	// is created in the proper way. This means that CAPH ensures that in the secret called "hcloud"
	// there is the hcloud token stored under the key "token". This might lead to the same value
	// being stored in two different keys of the same secret, depending on the configuration used
	// here.
	//
	// We recommend to use the value "hcloud". Set `spec.skipCreatingHetznerSecretInWorkloadCluster`
	// if you don't want workload-cluster secrets to be created.
	//
	// +optional
	// +kubebuilder:default=hetzner
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name,omitempty"`

	// key defines the keys that are used in the secret.
	// +required
	Key HetznerSecretKeyRef `json:"key,omitempty"`
}

// HetznerSecretKeyRef defines the key name of the HetznerSecret.
type HetznerSecretKeyRef struct {
	// hcloudToken defines the name of the key where the token for the Hetzner Cloud API is stored.
	// The controller reads the token from the management-cluster secret using this key and writes
	// it to workload-cluster secrets using the same key. Upstream hcloud-ccm expects the key
	// "token", while the legacy Syself ccm fork uses "hcloud". The configured workload-cluster
	// secret remains untouched. CAPH only adds the compatibility key "token" inside the
	// workload-cluster secret named "hcloud", and only if this field is not already "token". If
	// the configured secret name is already "hcloud", the same token value is stored under both
	// keys in that single secret. It defaults to "token".
	//
	// +optional
	// +kubebuilder:default=token
	// +kubebuilder:validation:MaxLength=253
	HCloudToken string `json:"hcloudToken"`

	// hetznerRobotUser defines the name of the key where the username for the Hetzner Robot API is
	// stored. It gets used for reading the credential in the mgt-cluster, and it gets used for
	// creating a secret in the wl-cluster. We recommend to use "robot-user", because this is the
	// default of upstream hcloud-ccm.
	//
	// +optional
	// +kubebuilder:default=hetzner-robot-user
	// +kubebuilder:validation:MaxLength=253
	HetznerRobotUser string `json:"hetznerRobotUser"`

	// hetznerRobotPassword defines the name of the key where the password for the Hetzner Robot API
	// is stored.  It gets used for reading the credential in the mgt-cluster, and it gets used for
	// creating a secret in the wl-cluster. We recommend to use "robot-password", because this is
	// the default of upstream hcloud-ccm.
	//
	// +optional
	// +kubebuilder:default=hetzner-robot-password
	// +kubebuilder:validation:MaxLength=253
	HetznerRobotPassword string `json:"hetznerRobotPassword"`

	// sshKey defines the name of the ssh key. It is only used in the mgt-cluster. It is not synced
	// to the wl-cluster.
	//
	// +optional
	// +kubebuilder:default=hcloud-ssh-key-name
	// +kubebuilder:validation:MaxLength=253
	SSHKey string `json:"sshKey"`
}

// PublicNetworkSpec contains specs about the public network spec of an HCloud server.
type PublicNetworkSpec struct {
	// enableIPv4 defines whether server has IPv4 address enabled.
	// As Hetzner load balancers require an IPv4 address, this setting will be ignored and set to true if there is no private net.
	// +optional
	// +kubebuilder:default=true
	EnableIPv4 bool `json:"enableIPv4"`
	// enableIPv6 defines whether server has IPv6 addresses enabled.
	// +optional
	// +kubebuilder:default=true
	EnableIPv6 bool `json:"enableIPv6"`
}

// LoadBalancerSpec defines the desired state of the Control Plane load balancer.
// +kubebuilder:validation:MinProperties=1
type LoadBalancerSpec struct {
	// enabled specifies if a load balancer should be created.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// name defines the name of the load balancer. It can be specified in order to use an existing load balancer.
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Name *string `json:"name,omitempty"`

	// algorithm defines the type of load balancer algorithm. It could be round_robin or least_connections. The default value is "round_robin".
	// +optional
	// +kubebuilder:default=round_robin
	Algorithm LoadBalancerAlgorithmType `json:"algorithm,omitempty"`

	// type defines the type of load balancer. It could be one of lb11, lb21, or lb31.
	// +optional
	// +kubebuilder:validation:Enum=lb11;lb21;lb31
	// +kubebuilder:default=lb11
	Type string `json:"type,omitempty"`

	// port defines the API Server port. It must be a valid port range (1-65535). If omitted, the default value is 6443.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=6443
	Port int32 `json:"port,omitempty"`

	// extraServices defines how traffic will be routed from the load balancer to your target server.
	// The limit is more than twice the Hetzner limit of 30 services on a load balancer. A higher Hetzner limit then still works.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=64
	ExtraServices []LoadBalancerServiceSpec `json:"extraServices,omitempty"`

	// region contains the name of the HCloud location where the load balancer is running.
	// +optional
	Region Region `json:"region,omitempty"`

	// enableProxyProtocol enables proxy protocol on the kube-apiserver load balancer service.
	// Only the kube-apiserver service is affected; extra services are never given proxy protocol.
	//
	// For new clusters the LB service is created with proxy protocol enabled immediately — no
	// annotation check is performed because there are no existing backends that could receive
	// unexpected PROXY-protocol headers.
	//
	// For existing clusters that want to enable proxy protocol after the fact, CAPH waits until
	// every control-plane machine carries the annotation
	// capi.syself.com/proxy-protocol-for-controlplane-loadbalancer: "true" before switching the LB
	// service to proxy protocol in place. The annotation is set on the control-plane machine
	// template, so a machine from an earlier template does not carry it and the check stays false
	// until the rollout completes. This prevents backends still expecting plain TCP from receiving
	// malformed PROXY-protocol headers.
	//
	// Enabling proxy protocol is a one-way operation — it is never turned back off.
	// +optional
	EnableProxyProtocol bool `json:"enableProxyProtocol,omitempty"`

	// targetAddressFamily selects which addresses of a bare metal control plane server are
	// attached as targets of the load balancer.
	//
	// This only affects bare metal servers. An HCloud server is a resource of the HCloud
	// API, so the load balancer references it by its server ID ("server" target type) and
	// resolves the address on its own. A bare metal server is a Robot resource that the
	// load balancer cannot reference that way, so it is attached by address instead ("ip"
	// target type), and such a target holds exactly one address. Reaching one server over
	// both protocols therefore takes two targets, and this field decides which of them
	// CAPH creates.
	//
	// An empty value means dualstack, so by default both the IPv4 and the IPv6 address are
	// attached. Set ipv4 on a cluster whose servers only use IPv4. Hetzner routes an IPv6
	// subnet to a bare metal server, so an IPv6 address for it usually exists in the Robot
	// API, but that says nothing about whether the installed OS configured it. An image that
	// sets up IPv4 only is common, and a target for an address the server does not answer on
	// never passes its health check. The load balancer then reports an unhealthy target for
	// as long as the machine exists, which buries a genuinely unhealthy control plane in
	// noise, and one target slot is spent on a target that cannot serve traffic. Set ipv6 for
	// a single-stack IPv6 setup.
	//
	// The value can be changed at any time. CAPH attaches the addresses of the selected
	// family that are missing and removes the targets of the addresses it no longer
	// selects, so a switch converges without manual cleanup in the HCloud API.
	// +optional
	TargetAddressFamily LoadBalancerTargetAddressFamily `json:"targetAddressFamily,omitempty"`

	// healthCheck configures the health check the load balancer uses to determine whether the
	// kube-apiserver service on a control-plane node is healthy. If omitted, the load balancer's
	// default behavior (a plain TCP check, with Hetzner's default interval, timeout and retries)
	// is unchanged.
	//
	// Switching protocol to http or https lets the load balancer check the kube-apiserver's actual
	// readiness (e.g. path "/readyz") instead of just whether the port accepts connections. Only
	// the kube-apiserver service is affected; extra services are never given a custom health
	// check. Doing so requires the kube-apiserver to serve that path without authentication,
	// since the load balancer's health check request is unauthenticated. This is not configured
	// by CAPH; the cluster operator must allow anonymous access to the configured path on the
	// apiserver.
	//
	// On an existing cluster, switching from tcp to an http or https check waits until every
	// control-plane infra machine carries the annotation
	// capi.syself.com/http-health-check-for-controlplane-loadbalancer: "true", set on the
	// control-plane infra machine template. This is checked on every reconcile, so it applies to
	// every switch away from tcp, not only the first one. Until the annotation is there the tcp
	// check stays in place, so the switch never marks a backend that does not yet answer the path
	// unhealthy. This mirrors the proxy-protocol migration in enableProxyProtocol. Any other change
	// (e.g. a path or timeout change while already on http/https) is applied immediately, without
	// this gate; nothing checks that the targets already pass the new check, so change these only
	// once they do.
	//
	// Leaving healthCheck out means CAPH does not manage the check, so a load balancer that
	// already has one keeps it. Deleting the field after an http check was applied therefore does
	// not undo that check.
	// +optional
	HealthCheck *LoadBalancerHealthCheckSpec `json:"healthCheck,omitempty"`
}

// TargetAddressFamilyOrDefault returns the address family to use for the "ip" targets of
// bare metal servers, resolving an empty value to the default.
//
// Defaulting happens here and not through a +kubebuilder:default marker on the field, so
// that an object which never set the field stays free of it instead of having the value
// written into it. A switch with a default branch rather than a comparison against the
// empty string is deliberate: the enum validation only guards writes through the API
// server, so an unexpected value can still reach this code, for example from an object
// stored before the field existed, and it has to resolve to something sane.
func (spec LoadBalancerSpec) TargetAddressFamilyOrDefault() LoadBalancerTargetAddressFamily {
	switch spec.TargetAddressFamily {
	case LoadBalancerTargetAddressFamilyIPv4, LoadBalancerTargetAddressFamilyIPv6:
		return spec.TargetAddressFamily
	default:
		return LoadBalancerTargetAddressFamilyDualStack
	}
}

// WantsIPv4 reports whether the configured address family selects the IPv4 address of a
// bare metal server for attachment.
func (spec LoadBalancerSpec) WantsIPv4() bool {
	family := spec.TargetAddressFamilyOrDefault()
	return family == LoadBalancerTargetAddressFamilyIPv4 || family == LoadBalancerTargetAddressFamilyDualStack
}

// WantsIPv6 reports whether the configured address family selects the IPv6 address of a
// bare metal server for attachment.
func (spec LoadBalancerSpec) WantsIPv6() bool {
	family := spec.TargetAddressFamilyOrDefault()
	return family == LoadBalancerTargetAddressFamilyIPv6 || family == LoadBalancerTargetAddressFamilyDualStack
}

// LoadBalancerHealthCheckSpec configures the health check a load balancer service uses. Field
// names mirror the Hetzner Cloud API's load balancer health_check object; see
// https://docs.hetzner.cloud/reference/cloud#tag/load-balancer-actions/add_load_balancer_service
// for the current defaults and limits.
type LoadBalancerHealthCheckSpec struct {
	// protocol is the protocol used for the health check. If omitted, "tcp" is used, matching the
	// load balancer's own default behavior.
	// +optional
	// +kubebuilder:validation:Enum=tcp;http;https
	// +kubebuilder:default=tcp
	Protocol string `json:"protocol,omitempty"`

	// port is the target port the check runs against. It defaults to the service's destination
	// port. Set it when the health-check endpoint is served on a different port than the API
	// server itself.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// intervalSeconds is the time in seconds between two consecutive health checks. If omitted,
	// Hetzner's own default is used (see the API reference on LoadBalancerHealthCheckSpec above).
	// +optional
	// +kubebuilder:validation:Minimum=3
	// +kubebuilder:validation:Maximum=60
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`

	// timeoutSeconds is the time in seconds to wait for a health check attempt to succeed. If
	// omitted, Hetzner's own default is used (see the API reference on LoadBalancerHealthCheckSpec
	// above).
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=60
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// retries is the number of consecutive failed health checks before a target is considered
	// unhealthy. The same number of successful checks makes it healthy again. If omitted,
	// Hetzner's own default is used (see the API reference on LoadBalancerHealthCheckSpec above).
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	Retries int32 `json:"retries,omitempty"`

	// path is the HTTP(S) path requested for the health check, e.g. "/readyz". Only valid when
	// protocol is http or https.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	Path *string `json:"path,omitempty"`

	// domain sets the Host header sent with the HTTP(S) health check request. Only valid when
	// protocol is http or https.
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Domain *string `json:"domain,omitempty"`

	// response is a string that must be contained in the HTTP(S) response for the check to pass.
	// Only valid when protocol is http or https.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	Response *string `json:"response,omitempty"`

	// statusCodes are the HTTP response status codes counted as healthy, for example ["200"].
	// Single codes ("200") and wildcards ("2??") are both allowed. If empty, Hetzner's own default
	// is used (see the API reference on LoadBalancerHealthCheckSpec above). Only valid when
	// protocol is http or https.
	// The limit of 16 characters for an item is far above the 3 characters of a status code.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=20
	// +kubebuilder:validation:items:MaxLength=16
	StatusCodes []string `json:"statusCodes,omitempty"`
}

// LoadBalancerServiceSpec defines a load balancer Target.
type LoadBalancerServiceSpec struct {
	// protocol specifies the supported load balancer Protocol. It could be one of the https, http, or tcp.
	// +required
	// +kubebuilder:validation:Enum=http;https;tcp
	Protocol string `json:"protocol,omitempty"`

	// listenPort defines the incoming port (source port) open on the load balancer. It must be a valid port range (1-65535).
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenPort int32 `json:"listenPort,omitempty"`

	// destinationPort defines the port on the server. It must be a valid port range (1-65535).
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	DestinationPort int32 `json:"destinationPort,omitempty"`
}

// LoadBalancerStatus defines the observed state of the control plane load balancer.
type LoadBalancerStatus struct {
	// id is the ID of the load balancer in HCloud.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ID int64 `json:"id,omitempty"`

	// ipv4 is the public IPv4 address of the load balancer.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=45
	IPv4 string `json:"ipv4,omitempty"`

	// ipv6 is the public IPv6 address of the load balancer.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=45
	IPv6 string `json:"ipv6,omitempty"`

	// internalIP is the IP address of the load balancer in the private network of the cluster.
	// It is empty when the load balancer is not attached to that network.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=45
	InternalIP string `json:"internalIP,omitempty"`

	// targets is the list of servers and IP addresses that the load balancer sends traffic to.
	// The limit is more than twice the Hetzner limit of 150 targets. A higher Hetzner limit then does not break status updates.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=512
	Target []LoadBalancerTarget `json:"targets,omitempty"`

	// protected is true when the load balancer has delete protection in HCloud.
	// +optional
	Protected bool `json:"protected,omitempty"`

	// proxyProtocolEnabled reflects whether the kube-apiserver load balancer service currently
	// has proxy protocol enabled, as observed on the actual HCloud load balancer. This can lag
	// behind spec.controlPlaneLoadBalancer.enableProxyProtocol while the migration to proxy
	// protocol is in progress (see the field's docs for details).
	// +optional
	ProxyProtocolEnabled bool `json:"proxyProtocolEnabled,omitempty"`
}

// LoadBalancerTarget defines the target of a load balancer.
type LoadBalancerTarget struct {
	// type is the type of the target, server or ip.
	// +required
	Type LoadBalancerTargetType `json:"type,omitempty"`

	// serverID is the ID of the HCloud server. It is only set when type is server.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ServerID int64 `json:"serverID,omitempty"`

	// ip is the IP address of the target. It is only set when type is ip.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=45
	IP string `json:"ip,omitempty"`
}

// HCloudNetworkSpec defines the desired state of the HCloud Private Network.
// +kubebuilder:validation:MinProperties=1
type HCloudNetworkSpec struct {
	// enabled defines whether the network should be enabled or not.
	// +optional
	Enabled bool `json:"enabled"`

	// cidrBlock defines the cidrBlock of the HCloud Network. If omitted, default "10.0.0.0/16" will be used.
	// +kubebuilder:default="10.0.0.0/16"
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=49
	CIDRBlock string `json:"cidrBlock,omitempty"`

	// subnetCidrBlock defines the cidrBlock for the subnet of the HCloud Network.
	// Note: A subnet is required.
	// +kubebuilder:default="10.0.0.0/24"
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=49
	SubnetCIDRBlock string `json:"subnetCidrBlock,omitempty"`

	// networkZone specifies the HCloud network zone of the private network.
	// The zones must be one of eu-central, us-east, us-west or ap-southeast. The default is eu-central.
	// +kubebuilder:validation:Enum=eu-central;us-east;us-west;ap-southeast
	// +kubebuilder:default=eu-central
	// +optional
	NetworkZone HCloudNetworkZone `json:"networkZone,omitempty"`
}

// NetworkStatus defines the observed state of the HCloud Private Network.
type NetworkStatus struct {
	// id is the ID of the network in HCloud.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ID int64 `json:"id,omitempty"`

	// attachedServers are the IDs of the HCloud servers that are attached to the network.
	// The limit is more than twice the Hetzner limit of 100 resources in a network. A higher Hetzner limit then does not break status updates.
	// +optional
	// +listType=set
	// +kubebuilder:validation:MaxItems=256
	AttachedServers []int64 `json:"attachedServers,omitempty"`
}

// Region is a Hetzner Location.
// +kubebuilder:validation:Enum=fsn1;hel1;nbg1;ash;hil;sin
type Region string

// HCloudNetworkZone describes the Network zone.
type HCloudNetworkZone string

// HCloudBootState defines the boot state of an HCloud server.
// A new constant must also be added to the Enum marker. TestStateEnumsMatchConstants checks this.
// +kubebuilder:validation:Enum=Initializing;EnablingRescue;BootingToRescue;RunningImageCommand;BootingToRealOS;OperatingSystemRunning;ProvisioningFailed
type HCloudBootState string

const (
	// HCloudBootStateUnset is the initial state when the boot state has not been set yet.
	HCloudBootStateUnset HCloudBootState = ""

	// HCloudBootStateInitializing indicates the controller waits for the server create action to
	// finish. The server stays powered off; once it is provisioned, the rescue system gets enabled.
	HCloudBootStateInitializing HCloudBootState = "Initializing"

	// HCloudBootStateEnablingRescue indicates that the controller waits for the rescue system to be enabled. Then the server gets booted into the rescue system.
	HCloudBootStateEnablingRescue HCloudBootState = "EnablingRescue"

	// HCloudBootStateBootingToRescue indicates that the controller
	// waits for the rescue system to be reachable. Then it starts the custom provisioner command.
	HCloudBootStateBootingToRescue HCloudBootState = "BootingToRescue"

	// HCloudBootStateRunningImageCommand indicates the controller waits for the
	// custom provisioner command, and then switches BootState to BootingToRealOS (no additional reboot gets
	// done).
	HCloudBootStateRunningImageCommand HCloudBootState = "RunningImageCommand"

	// HCloudBootStateBootingToRealOS indicates that the server is booting the operating system.
	HCloudBootStateBootingToRealOS HCloudBootState = "BootingToRealOS"

	// HCloudBootStateOperatingSystemRunning indicates that the server is successfully running.
	HCloudBootStateOperatingSystemRunning HCloudBootState = "OperatingSystemRunning"

	// HCloudBootStateProvisioningFailed indicates that provisioning failed. The capi machine, and
	// the hcloud machine will get deleted.
	HCloudBootStateProvisioningFailed HCloudBootState = "ProvisioningFailed"
)
