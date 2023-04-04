package api

import (
	"github.com/tilt-dev/ctlptl/pkg/api/k3dv1alpha4"
	"github.com/tilt-dev/localregistry-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

// TypeMeta partially copies apimachinery/pkg/apis/meta/v1.TypeMeta
// No need for a direct dependence; the fields are stable.
type TypeMeta struct {
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
}

// Cluster contains cluster configuration.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Cluster struct {
	TypeMeta `yaml:",inline"`

	// The cluster name. Pulled from .kube/config.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// The name of the tool used to create this cluster.
	Product string `json:"product,omitempty" yaml:"product,omitempty"`

	// Make sure that the cluster has access to at least this many
	// CPUs. This is mostly helpful for ensuring that your Docker Desktop
	// VM has enough CPU. If ctlptl can't guarantee this many
	// CPU, it will return an error.
	MinCPUs int `json:"minCPUs,omitempty" yaml:"minCPUs,omitempty"`

	// The name of a registry.
	//
	// If the registry doesn't exist, ctlptl will create one with this name.
	//
	// The registry can be configured by creating a `kind: Registry` config file.
	//
	// Not supported on all cluster products.
	Registry string `json:"registry,omitempty" yaml:"registry,omitempty"`

	// The desired version of Kubernetes to run.
	//
	// Examples:
	// v1.19.1
	// v1.14.0
	// Must start with 'v' and contain a major, minor, and patch version.
	//
	// Not all cluster products allow you to customize this.
	KubernetesVersion string `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`

	// The Kind cluster config. Only applicable for clusters with product: kind.
	//
	// Full documentation at:
	// https://pkg.go.dev/sigs.k8s.io/kind/pkg/apis/config/v1alpha4#Cluster
	//
	// Properties of this config may be overridden by properties of the ctlptl
	// Cluster config. For example, the name field of the top-level Cluster object
	// wins over one specified in the Kind config.
	KindV1Alpha4Cluster *v1alpha4.Cluster `json:"kindV1Alpha4Cluster,omitempty" yaml:"kindV1Alpha4Cluster,omitempty"`

	// The Minikube cluster config. Only applicable for clusters with product: minikube.
	Minikube *MinikubeCluster `json:"minikube,omitempty" yaml:"minikube,omitempty"`

	// The K3D cluster config. Only applicable for clusters with product: k3d.
	K3D *K3DCluster `json:"k3d,omitempty" yaml:"k3d,omitempty"`

	// Most recently observed status of the cluster.
	// Populated by the system.
	// Read-only.
	Status ClusterStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type ClusterStatus struct {
	// When the cluster was first created.
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty" yaml:"creationTimestamp,omitempty"`

	// Local registry status documented on the cluster itself.
	LocalRegistryHosting *localregistry.LocalRegistryHostingV1 `json:"localRegistryHosting,omitempty" yaml:"localRegistryHosting,omitempty"`

	// The number of CPU. Only applicable to local clusters.
	CPUs int `json:"cpus,omitempty" yaml:"cpus,omitempty"`

	// Whether this is the current cluster in `kubectl`
	Current bool `json:"current,omitempty" yaml:"current,omitempty"`

	// The version of Kubernetes currently running.
	//
	// Reported by the Kubernetes API. May contain a build tag.
	//
	// Examples:
	// v1.19.1
	// v1.18.10-gke.601
	// v1.19.3-34+fa32ff1c160058
	KubernetesVersion string `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`

	// Populated when we encounter an error reading the cluster status.
	Error string `json:"error,omitempty"`
}

// MinikubeCluster describes minikube-specific options for starting a cluster.
//
// Options in this struct, when possible, should match the flags
// to `minikube start`.
//
// Prefer setting features on the ClusterSpec rather than on the MinikubeCluster
// object when possible. For example, this object doesn't have a `kubernetesVersion`
// field, because it's supported by ClusterSpec.
//
// ctlptl's logic for diffing clusters and applying changes is less robust
// for cluster-specific config flags.
type MinikubeCluster struct {
	// The container runtime of the cluster. Defaults to containerd.
	ContainerRuntime string `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`

	// Extra config options passed directly to Minikube's --extra-config flags.
	// When not set, we will default to starting minikube with these configs:
	//
	// kubelet.max-pods=500
	ExtraConfigs []string `json:"extraConfigs,omitempty" yaml:"extraConfigs,omitempty"`

	// Unstructured flags to pass to minikube on `minikube start`.
	// These flags will be passed before all tilt-determined flags.
	StartFlags []string `json:"startFlags,omitempty" yaml:"startFlags,omitempty"`
}

// K3DCluster describes k3d-specific options for starting a cluster.
//
// Prefer setting features on the ClusterSpec rather than on the K3dCluster
// object when possible.
//
// ctlptl's logic for diffing clusters and applying changes is less robust
// for cluster-specific configs.
type K3DCluster struct {
	// K3D's own cluster config format.
	//
	// Documentation: https://k3d.io/v5.4.6/usage/configfile/
	//
	// Uses this schema: https://github.com/k3d-io/k3d/blob/v5.4.6/pkg/config/v1alpha4/types.go
	V1Alpha4Simple *k3dv1alpha4.SimpleConfig `json:"v1alpha4Simple,omitempty" yaml:"v1alpha4Simple,omitempty"`
}

// ClusterList is a list of Clusters.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	TypeMeta `json:",inline"`

	// List of clusters.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md
	Items []Cluster `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// Cluster contains registry configuration.
//
// Currently designed for local registries on the host machine, but
// may eventually expand to support remote registries.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Registry struct {
	TypeMeta `yaml:",inline"`

	// The registry name. Get/set from the Docker container name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// The host IPv4 address to bind the container to.
	ListenAddress string `json:"listenAddress,omitempty" yaml:"listenAddress,omitempty"`

	// The desired host port. Set to 0 to choose a random port,
	// or to preserve the existing port.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// Labels that must be attached to the running registry.
	//
	// If you change the set of labels, the registry must be stopped and
	// restarted.
	//
	// Important for K3d, which will only connect to registries
	// that are tagged "app: k3d".
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Environment vars to use for registry container (optional).
	//
	// Can be used to change some parameters likes REGISTRY_HTTP_ADDR, REGISTRY_PROXY_REMOTEURL
	Env []string `json:"env,omitempty" yaml:"env,omitempty"`

	// Image to use for registry container (optional).
	//
	// Can be used to provide an alternate image or use a different registry
	// than Docker Hub.
	//
	// Defaults to `docker.io/library/registry:2`.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// Most recently observed status of the registry.
	// Populated by the system.
	// Read-only.
	Status RegistryStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type RegistryStatus struct {
	// When the registry was first created.
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty" yaml:"creationTimestamp,omitempty"`

	// The IPv4 address for the bridge network.
	IPAddress string `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`

	// The public IPv4 address that the registry is listening on on the host machine.
	ListenAddress string `json:"listenAddress,omitempty" yaml:"listenAddress,omitempty"`

	// The public port that the registry is listening on on the host machine.
	HostPort int `json:"hostPort,omitempty" yaml:"hostPort,omitempty"`

	// The private port that the registry is listening on inside the registry network.
	//
	// We try to make this not configurable, because there's no real reason not
	// to use the default registry port 5000.
	ContainerPort int `json:"containerPort,omitempty" yaml:"containerPort,omitempty"`

	// Networks that the registry container is connected to.
	Networks []string `json:"networks,omitempty" yaml:"networks,omitempty"`

	// The ID of the container in Docker.
	ContainerID string `json:"containerId,omitempty" yaml:"containerId,omitempty"`

	// Current health status of the registry container.
	// Reflects underlying ContainerState.Status
	// https://github.com/moby/moby/blob/v20.10.3/api/types/types.go#L314
	State string

	// Labels attached to the running container.
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Env attached to the running container.
	Env []string `json:"env,omitempty" yaml:"env,omitempty"`

	// Image for the running container.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`
}

// RegistryList is a list of Registrys.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RegistryList struct {
	TypeMeta `json:",inline"`

	// List of registrys.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md
	Items []Registry `json:"items" protobuf:"bytes,2,rep,name=items"`
}
