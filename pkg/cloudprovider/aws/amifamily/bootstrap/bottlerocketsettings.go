package bootstrap

// config is the root of the bottlerocket config, see more here https://github.com/bottlerocket-os/bottlerocket#using-user-data
type config struct {
	Settings settings `toml:"settings"`
}

// This is a subset of all configuration in https://github.com/bottlerocket-os/bottlerocket/blob/develop/sources/models/src/aws-k8s-1.22/mod.rs
// These settings apply across all K8s versions that karpenter supports.
// This is currently an opinionated subset and can evolve over time
type settings struct {
	Kubernetes     kubernetes      `toml:"kubernetes"`
	HostContainers *hostContainers `toml:"host-containers,omitempty"`
	AWS            *awsConfig      `toml:"aws,omitempty"`
	Metrics        *metrics        `toml:"metrics,omitempty"`
	Kernel         *kernel         `toml:"kernel,omitempty"`
}

// kubernetes specific configuration for bottlerocket api
type kubernetes struct {
	APIServer                 string               `toml:"api-server"`
	ClusterCertificate        *string              `toml:"cluster-certificate"`
	ClusterName               *string              `toml:"cluster-name"`
	ClusterDNSIP              *string              `toml:"cluster-dns-ip,omitempty"`
	NodeLabels                map[string]string    `toml:"node-labels,omitempty"`
	NodeTaints                map[string][]string  `toml:"node-taints,omitempty"`
	MaxPods                   *int                 `toml:"max-pods,omitempty"`
	StaticPods                map[string]staticPod `toml:"static-pods,omitempty"`
	EvictionHard              map[string]string    `toml:"eviction-hard,omitempty"`
	KubeReserved              map[string]string    `toml:"kube-reserved,omitempty"`
	SystemReserved            map[string]string    `toml:"system-reserved,omitempty"`
	AllowedUnsafeSysctls      []*string            `toml:"allowed-unsafe-sysctls,omitempty"`
	ServerTLSBootstrap        *bool                `toml:"server-tls-bootstrap,omitempty"`
	RegistryQPS               *int                 `toml:"registry-qps,omitempty"`
	RegistryBurst             *int                 `toml:"registry-burst,omitempty"`
	EventQPS                  *int                 `toml:"event-qps,omitempty"`
	EventBurst                *int                 `toml:"event-burst,omitempty"`
	KubeAPIQPS                *int                 `toml:"kube-api-qps,omitempty"`
	KubeAPIBurst              *int                 `toml:"kube-api-burst,omitempty"`
	ContainerLogMaxSize       *string              `toml:"container-log-max-size,omitempty"`
	ContainerLogMaxFiles      *int                 `toml:"container-log-max-files,omitempty"`
	CPUManagerPolicy          *string              `toml:"cpu-manager-policy,omitempty"`
	CPUManagerReconcilePeriod *string              `toml:"cpu-manager-reconcile-period,omitempty"`
	TopologyManagerScope      *string              `toml:"topology-manager-scope,omitempty"`
	TopologyManagerPolicy     *string              `toml:"topology-manager-policy,omitempty"`
}

type staticPod struct {
	Enabled  *bool   `toml:"enabled,omitempty"`
	Manifest *string `toml:"manifest,omitempty"`
}

type awsConfig struct {
	Region *string `toml:"region,omitempty"`
}

type hostContainers struct {
	Admin   *admin   `toml:"admin,omitempty"`
	Control *control `toml:"control,omitempty"`
}

type admin struct {
	Enabled      *bool   `toml:"enabled,omitempty"`
	Source       *string `toml:"source,omitempty"`
	Superpowered *bool   `toml:"superpowered,omitempty"`
	UserData     *string `toml:"user-data,omitempty"`
}

type control struct {
	Enabled      *bool   `toml:"enabled,omitempty"`
	Source       *string `toml:"source,omitempty"`
	Superpowered *bool   `toml:"superpowered,omitempty"`
}

type metrics struct {
	MetricsURL    *string   `toml:"metrics-url,omitempty"`
	SendMetrics   *bool     `toml:"send-metrics,omitempty"`
	ServiceChecks []*string `toml:"service-checks,omitempty"`
}

type kernel struct {
	Lockdown *string           `toml:"lockdown,omitempty"`
	SysCtl   map[string]string `toml:"sysctl,omitempty"`
}
