/*
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

package bootstrap

import (
	"github.com/pelletier/go-toml/v2"
)

func NewBottlerocketConfig(userdata *string) (*BottlerocketConfig, error) {
	c := &BottlerocketConfig{}
	if userdata == nil {
		return c, nil
	}
	if err := c.UnmarshalTOML([]byte(*userdata)); err != nil {
		return c, err
	}
	return c, nil
}

// BottlerocketConfig is the root of the bottlerocket config, see more here https://github.com/bottlerocket-os/bottlerocket#using-user-data
type BottlerocketConfig struct {
	SettingsRaw map[string]interface{} `toml:"settings"`
	Settings    BottlerocketSettings   `toml:"-"`
}

// BottlerocketSettings is a subset of all configuration in https://github.com/bottlerocket-os/bottlerocket/blob/d427c40931cba6e6bedc5b75e9c084a6e1818db9/sources/models/src/lib.rs#L260
// These settings apply across all K8s versions that karpenter supports.
type BottlerocketSettings struct {
	Kubernetes        BottlerocketKubernetes      `toml:"kubernetes"`
	BootstrapCommands map[string]BootstrapCommand `toml:"bootstrap-commands,omitempty"`
}

// BottlerocketKubernetes is k8s specific configuration for bottlerocket api
type BottlerocketKubernetes struct {
	APIServer                          *string                                   `toml:"api-server"`
	CloudProvider                      *string                                   `toml:"cloud-provider"`
	ClusterCertificate                 *string                                   `toml:"cluster-certificate"`
	ClusterName                        *string                                   `toml:"cluster-name"`
	ClusterDNSIP                       *ClusterDNS                               `toml:"cluster-dns-ip,omitempty"`
	CredentialProviders                map[string]BottlerocketCredentialProvider `toml:"credential-providers,omitempty"`
	NodeLabels                         map[string]string                         `toml:"node-labels,omitempty"`
	NodeTaints                         map[string][]string                       `toml:"node-taints,omitempty"`
	MaxPods                            *int                                      `toml:"max-pods,omitempty"`
	StaticPods                         map[string]BottlerocketStaticPod          `toml:"static-pods,omitempty"`
	EvictionHard                       map[string]string                         `toml:"eviction-hard,omitempty"`
	EvictionSoft                       map[string]string                         `toml:"eviction-soft,omitempty"`
	EvictionSoftGracePeriod            map[string]string                         `toml:"eviction-soft-grace-period,omitempty"`
	EvictionMaxPodGracePeriod          *int                                      `toml:"eviction-max-pod-grace-period,omitempty"`
	KubeReserved                       map[string]string                         `toml:"kube-reserved,omitempty"`
	SystemReserved                     map[string]string                         `toml:"system-reserved,omitempty"`
	AllowedUnsafeSysctls               []string                                  `toml:"allowed-unsafe-sysctls,omitempty"`
	ServerTLSBootstrap                 *bool                                     `toml:"server-tls-bootstrap,omitempty"`
	RegistryQPS                        *int                                      `toml:"registry-qps,omitempty"`
	RegistryBurst                      *int                                      `toml:"registry-burst,omitempty"`
	EventQPS                           *int                                      `toml:"event-qps,omitempty"`
	EventBurst                         *int                                      `toml:"event-burst,omitempty"`
	KubeAPIQPS                         *int                                      `toml:"kube-api-qps,omitempty"`
	KubeAPIBurst                       *int                                      `toml:"kube-api-burst,omitempty"`
	ContainerLogMaxSize                *string                                   `toml:"container-log-max-size,omitempty"`
	ContainerLogMaxFiles               *int                                      `toml:"container-log-max-files,omitempty"`
	CPUManagerPolicy                   *string                                   `toml:"cpu-manager-policy,omitempty"`
	CPUManagerReconcilePeriod          *string                                   `toml:"cpu-manager-reconcile-period,omitempty"`
	TopologyManagerScope               *string                                   `toml:"topology-manager-scope,omitempty"`
	TopologyManagerPolicy              *string                                   `toml:"topology-manager-policy,omitempty"`
	ImageGCHighThresholdPercent        *string                                   `toml:"image-gc-high-threshold-percent,omitempty"`
	ImageGCLowThresholdPercent         *string                                   `toml:"image-gc-low-threshold-percent,omitempty"`
	CPUCFSQuota                        *bool                                     `toml:"cpu-cfs-quota-enforced,omitempty"`
	ShutdownGracePeriod                *string                                   `toml:"shutdown-grace-period,omitempty"`
	ShutdownGracePeriodForCriticalPods *string                                   `toml:"shutdown-grace-period-for-critical-pods,omitempty"`
	ClusterDomain                      *string                                   `toml:"cluster-domain,omitempty"`
	SeccompDefault                     *bool                                     `toml:"seccomp-default,omitempty"`
	PodPidsLimit                       *int                                      `toml:"pod-pids-limit,omitempty"`
	DeviceOwnershipFromSecurityContext *bool                                     `toml:"device-ownership-from-security-context,omitempty"`
	SingleProcessOOMKill               *bool                                     `toml:"single-process-oom-kill,omitempty"`
	ContainerLogMaxWorkers             *int                                      `toml:"container-log-max-workers,omitempty"`
	ContainerLogMonitorInterval        *string                                   `toml:"container-log-monitor-interval,omitempty"`
}
type BottlerocketStaticPod struct {
	Enabled  *bool   `toml:"enabled,omitempty"`
	Manifest *string `toml:"manifest,omitempty"`
}

// BottlerocketCredentialProvider is k8s specific configuration for Bottlerocket Kubelet image credential provider
// See Bottlerocket struct at https://github.com/bottlerocket-os/bottlerocket/blob/d427c40931cba6e6bedc5b75e9c084a6e1818db9/sources/models/modeled-types/src/kubernetes.rs#L1307
type BottlerocketCredentialProvider struct {
	Enabled       *bool             `toml:"enabled"`
	CacheDuration *string           `toml:"cache-duration,omitempty"`
	ImagePatterns []string          `toml:"image-patterns"`
	Environment   map[string]string `toml:"environment,omitempty"`
}

type BootstrapCommandMode string

const (
	BootstrapCommandModeAlways BootstrapCommandMode = "always"
	BootstrapCommandModeOnce   BootstrapCommandMode = "once"
	BootstrapCommandModeOff    BootstrapCommandMode = "off"
)

// BootstrapCommand model defined in the Bottlerocket Core Kit in
// https://github.com/bottlerocket-os/bottlerocket-core-kit/blob/fdf32c291ad18370de3a5fdc4c20a9588bc14177/sources/bootstrap-commands/src/main.rs#L57
type BootstrapCommand struct {
	Commands  [][]string           `toml:"commands"`
	Mode      BootstrapCommandMode `toml:"mode"`
	Essential bool                 `toml:"essential"`
}

func (c *BottlerocketConfig) UnmarshalTOML(data []byte) error {
	// unmarshal known settings
	s := struct {
		Settings BottlerocketSettings `toml:"settings"`
	}{}
	if err := toml.Unmarshal(data, &s); err != nil {
		return err
	}
	// unmarshal untyped settings
	if err := toml.Unmarshal(data, c); err != nil {
		return err
	}
	c.Settings = s.Settings
	return nil
}

func (c *BottlerocketConfig) MarshalTOML() ([]byte, error) {
	if c.SettingsRaw == nil {
		c.SettingsRaw = map[string]interface{}{}
	}
	c.SettingsRaw["kubernetes"] = c.Settings.Kubernetes
	if c.Settings.BootstrapCommands != nil {
		c.SettingsRaw["bootstrap-commands"] = c.Settings.BootstrapCommands
	}
	return toml.Marshal(c)
}
