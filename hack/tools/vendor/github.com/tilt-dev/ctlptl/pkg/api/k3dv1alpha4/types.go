package k3dv1alpha4

import "time"

// NOTE(nicks): Forked from
// https://github.com/k3d-io/k3d/blob/v5.4.6/pkg/config/v1alpha4/types.go
// Modified to work with k8s api infra.

// TypeMeta partially copies apimachinery/pkg/apis/meta/v1.TypeMeta
// No need for a direct dependence; the fields are stable.
type TypeMeta struct {
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
}

type ObjectMeta struct {
	Name string `mapstructure:"name,omitempty" json:"name,omitempty" yaml:"name,omitempty"`
}

type VolumeWithNodeFilters struct {
	Volume      string   `mapstructure:"volume" yaml:"volume,omitempty" json:"volume,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty" json:"nodeFilters,omitempty"`
}

type PortWithNodeFilters struct {
	Port        string   `mapstructure:"port" yaml:"port,omitempty" json:"port,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty" json:"nodeFilters,omitempty"`
}

type LabelWithNodeFilters struct {
	Label       string   `mapstructure:"label" yaml:"label,omitempty" json:"label,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty" json:"nodeFilters,omitempty"`
}

type EnvVarWithNodeFilters struct {
	EnvVar      string   `mapstructure:"envVar" yaml:"envVar,omitempty" json:"envVar,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty" json:"nodeFilters,omitempty"`
}

type K3sArgWithNodeFilters struct {
	Arg         string   `mapstructure:"arg" yaml:"arg,omitempty" json:"arg,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty" json:"nodeFilters,omitempty"`
}

type SimpleConfigRegistryCreateConfig struct {
	Name     string   `mapstructure:"name" yaml:"name,omitempty" json:"name,omitempty"`
	Host     string   `mapstructure:"host" yaml:"host,omitempty" json:"host,omitempty"`
	HostPort string   `mapstructure:"hostPort" yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
	Image    string   `mapstructure:"image" yaml:"image,omitempty" json:"image,omitempty"`
	Volumes  []string `mapstructure:"volumes" yaml:"volumes,omitempty" json:"volumes,omitempty"`
}

// SimpleConfigOptionsKubeconfig describes the set of options referring to the kubeconfig during cluster creation.
type SimpleConfigOptionsKubeconfig struct {
	UpdateDefaultKubeconfig bool `mapstructure:"updateDefaultKubeconfig" yaml:"updateDefaultKubeconfig,omitempty" json:"updateDefaultKubeconfig,omitempty"` // default: true
	SwitchCurrentContext    bool `mapstructure:"switchCurrentContext" yaml:"switchCurrentContext,omitempty" json:"switchCurrentContext,omitempty"`          //nolint:lll    // default: true
}

type SimpleConfigOptions struct {
	K3dOptions        SimpleConfigOptionsK3d        `mapstructure:"k3d" yaml:"k3d" json:"k3d"`
	K3sOptions        SimpleConfigOptionsK3s        `mapstructure:"k3s" yaml:"k3s" json:"k3s"`
	KubeconfigOptions SimpleConfigOptionsKubeconfig `mapstructure:"kubeconfig" yaml:"kubeconfig" json:"kubeconfig"`
	Runtime           SimpleConfigOptionsRuntime    `mapstructure:"runtime" yaml:"runtime" json:"runtime"`
}

type SimpleConfigOptionsRuntime struct {
	GPURequest    string                 `mapstructure:"gpuRequest" yaml:"gpuRequest,omitempty" json:"gpuRequest,omitempty"`
	ServersMemory string                 `mapstructure:"serversMemory" yaml:"serversMemory,omitempty" json:"serversMemory,omitempty"`
	AgentsMemory  string                 `mapstructure:"agentsMemory" yaml:"agentsMemory,omitempty" json:"agentsMemory,omitempty"`
	HostPidMode   bool                   `mapstructure:"hostPidMode" yyaml:"hostPidMode,omitempty" json:"hostPidMode,omitempty"`
	Labels        []LabelWithNodeFilters `mapstructure:"labels" yaml:"labels,omitempty" json:"labels,omitempty"`
}

type SimpleConfigOptionsK3d struct {
	Wait                bool                               `mapstructure:"wait" yaml:"wait" json:"wait"`
	Timeout             time.Duration                      `mapstructure:"timeout" yaml:"timeout,omitempty" json:"timeout,omitempty"`
	DisableLoadbalancer bool                               `mapstructure:"disableLoadbalancer" yaml:"disableLoadbalancer" json:"disableLoadbalancer"`
	DisableImageVolume  bool                               `mapstructure:"disableImageVolume" yaml:"disableImageVolume" json:"disableImageVolume"`
	NoRollback          bool                               `mapstructure:"disableRollback" yaml:"disableRollback" json:"disableRollback"`
	Loadbalancer        SimpleConfigOptionsK3dLoadbalancer `mapstructure:"loadbalancer" yaml:"loadbalancer,omitempty" json:"loadbalancer,omitempty"`
}

type SimpleConfigOptionsK3dLoadbalancer struct {
	ConfigOverrides []string `mapstructure:"configOverrides" yaml:"configOverrides,omitempty" json:"configOverrides,omitempty"`
}

type SimpleConfigOptionsK3s struct {
	ExtraArgs  []K3sArgWithNodeFilters `mapstructure:"extraArgs" yaml:"extraArgs,omitempty" json:"extraArgs,omitempty"`
	NodeLabels []LabelWithNodeFilters  `mapstructure:"nodeLabels" yaml:"nodeLabels,omitempty" json:"nodeLabels,omitempty"`
}

type SimpleConfigRegistries struct {
	Use    []string                          `mapstructure:"use" yaml:"use,omitempty" json:"use,omitempty"`
	Create *SimpleConfigRegistryCreateConfig `mapstructure:"create" yaml:"create,omitempty" json:"create,omitempty"`
	Config string                            `mapstructure:"config" yaml:"config,omitempty" json:"config,omitempty"` // registries.yaml (k3s config for containerd registry override)
}

type SimpleConfigHostAlias struct {
	IP        string   `mapstructure:"ip" yaml:"ip" json:"ip"`
	Hostnames []string `mapstructure:"hostnames" yaml:"hostnames" json:"hostnames"`
}

// SimpleConfig describes the toplevel k3d configuration file.
type SimpleConfig struct {
	TypeMeta     `yaml:",inline"`
	ObjectMeta   `mapstructure:"metadata" yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Servers      int                     `mapstructure:"servers" yaml:"servers,omitempty" json:"servers,omitempty"` //nolint:lll    // default 1
	Agents       int                     `mapstructure:"agents" yaml:"agents,omitempty" json:"agents,omitempty"`    //nolint:lll    // default 0
	ExposeAPI    SimpleExposureOpts      `mapstructure:"kubeAPI" yaml:"kubeAPI,omitempty" json:"kubeAPI,omitempty"`
	Image        string                  `mapstructure:"image" yaml:"image,omitempty" json:"image,omitempty"`
	Network      string                  `mapstructure:"network" yaml:"network,omitempty" json:"network,omitempty"`
	Subnet       string                  `mapstructure:"subnet" yaml:"subnet,omitempty" json:"subnet,omitempty"`
	ClusterToken string                  `mapstructure:"token" yaml:"clusterToken,omitempty" json:"clusterToken,omitempty"` // default: auto-generated
	Volumes      []VolumeWithNodeFilters `mapstructure:"volumes" yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Ports        []PortWithNodeFilters   `mapstructure:"ports" yaml:"ports,omitempty" json:"ports,omitempty"`
	Options      SimpleConfigOptions     `mapstructure:"options" yaml:"options,omitempty" json:"options,omitempty"`
	Env          []EnvVarWithNodeFilters `mapstructure:"env" yaml:"env,omitempty" json:"env,omitempty"`
	Registries   SimpleConfigRegistries  `mapstructure:"registries" yaml:"registries,omitempty" json:"registries,omitempty"`
	HostAliases  []SimpleConfigHostAlias `mapstructure:"hostAliases" yaml:"hostAliases,omitempty" json:"hostAliases,omitempty"`
}

// SimpleExposureOpts provides a simplified syntax compared to the original k3d.ExposureOpts
type SimpleExposureOpts struct {
	Host     string `mapstructure:"host" yaml:"host,omitempty" json:"host,omitempty"`
	HostIP   string `mapstructure:"hostIP" yaml:"hostIP,omitempty" json:"hostIP,omitempty"`
	HostPort string `mapstructure:"hostPort" yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
}
