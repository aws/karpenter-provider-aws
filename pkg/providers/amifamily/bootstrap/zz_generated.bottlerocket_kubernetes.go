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

// GENERATED FILE. DO NOT EDIT DIRECTLY.
// Update hack/code/bottlerocket_settings_gen/main.go and re-generate to edit.
// source: https://github.com/bottlerocket-os/bottlerocket-settings-sdk/blob/bottlerocket-settings-models-v0.31.0/bottlerocket-settings-models/settings-extensions/kubernetes/src/lib.rs

// BottlerocketKubernetes is k8s specific configuration for bottlerocket api
type BottlerocketKubernetes struct {
	APIServer                          *string                                            `toml:"api-server"`
	CloudProvider                      *string                                            `toml:"cloud-provider"`
	ClusterCertificate                 *string                                            `toml:"cluster-certificate"`
	ClusterName                        *string                                            `toml:"cluster-name"`
	ClusterDNSIP                       *string                                            `toml:"cluster-dns-ip,omitempty"`
	CredentialProviders                map[string]BottlerocketCredentialProvider          `toml:"credential-providers,omitempty"`
	NodeLabels                         map[string]string                                  `toml:"node-labels,omitempty"`
	NodeTaints                         map[string][]string                                `toml:"node-taints,omitempty"`
	MaxPods                            *int                                               `toml:"max-pods,omitempty"`
	StaticPods                         map[string]BottlerocketStaticPod                   `toml:"static-pods,omitempty"`
	EvictionHard                       map[string]string                                  `toml:"eviction-hard,omitempty"`
	EvictionSoft                       map[string]string                                  `toml:"eviction-soft,omitempty"`
	EvictionSoftGracePeriod            map[string]string                                  `toml:"eviction-soft-grace-period,omitempty"`
	EvictionMaxPodGracePeriod          *int                                               `toml:"eviction-max-pod-grace-period,omitempty"`
	KubeReserved                       map[string]string                                  `toml:"kube-reserved,omitempty"`
	SystemReserved                     map[string]string                                  `toml:"system-reserved,omitempty"`
	AllowedUnsafeSysctls               []string                                           `toml:"allowed-unsafe-sysctls,omitempty"`
	ServerTLSBootstrap                 *bool                                              `toml:"server-tls-bootstrap,omitempty"`
	RegistryQPS                        *int                                               `toml:"registry-qps,omitempty"`
	RegistryBurst                      *int                                               `toml:"registry-burst,omitempty"`
	EventQPS                           *int                                               `toml:"event-qps,omitempty"`
	EventBurst                         *int                                               `toml:"event-burst,omitempty"`
	KubeAPIQPS                         *int                                               `toml:"kube-api-qps,omitempty"`
	KubeAPIBurst                       *int                                               `toml:"kube-api-burst,omitempty"`
	ContainerLogMaxSize                *string                                            `toml:"container-log-max-size,omitempty"`
	ContainerLogMaxFiles               *int                                               `toml:"container-log-max-files,omitempty"`
	CPUManagerPolicy                   *string                                            `toml:"cpu-manager-policy,omitempty"`
	CPUManagerPolicyOptions            []string                                           `toml:"cpu-manager-policy-options,omitempty"`
	CPUManagerReconcilePeriod          *string                                            `toml:"cpu-manager-reconcile-period,omitempty"`
	MemoryManagerPolicy                *string                                            `toml:"memory-manager-policy,omitempty"`
	MemoryManagerReservedMemory        map[string]BottlerocketMemoryManagerReservedMemory `toml:"memory-manager-reserved-memory,omitempty"`
	MemorySwapBehavior                 *string                                            `toml:"memory-swap-behavior,omitempty"`
	MemoryReservationPolicy            *string                                            `toml:"memory-reservation-policy,omitempty"`
	MemoryThrottlingFactor             *float64                                           `toml:"memory-throttling-factor,omitempty"`
	TopologyManagerScope               *string                                            `toml:"topology-manager-scope,omitempty"`
	TopologyManagerPolicy              *string                                            `toml:"topology-manager-policy,omitempty"`
	TopologyManagerPolicyOptions       *BottlerocketTopologyManagerPolicyOptions          `toml:"topology-manager-policy-options,omitempty"`
	ImageGCHighThresholdPercent        *string                                            `toml:"image-gc-high-threshold-percent,omitempty"`
	ImageGCLowThresholdPercent         *string                                            `toml:"image-gc-low-threshold-percent,omitempty"`
	IdsPerPod                          *int                                               `toml:"ids-per-pod,omitempty"`
	CPUCFSQuota                        *bool                                              `toml:"cpu-cfs-quota-enforced,omitempty"`
	ShutdownGracePeriod                *string                                            `toml:"shutdown-grace-period,omitempty"`
	ShutdownGracePeriodForCriticalPods *string                                            `toml:"shutdown-grace-period-for-critical-pods,omitempty"`
	ClusterDomain                      *string                                            `toml:"cluster-domain,omitempty"`
	SeccompDefault                     *bool                                              `toml:"seccomp-default,omitempty"`
	PodPidsLimit                       *int                                               `toml:"pod-pids-limit,omitempty"`
	DeviceOwnershipFromSecurityContext *bool                                              `toml:"device-ownership-from-security-context,omitempty"`
	SingleProcessOOMKill               *bool                                              `toml:"single-process-oom-kill,omitempty"`
	ContainerLogMaxWorkers             *int                                               `toml:"container-log-max-workers,omitempty"`
	ContainerLogMonitorInterval        *string                                            `toml:"container-log-monitor-interval,omitempty"`
	HostnameOverrideSource             *string                                            `toml:"hostname-override-source,omitempty"`
	VerbosityLevel                     *uint32                                            `toml:"log-level,omitempty"`
	AuthenticationMode                 *string                                            `toml:"authentication-mode,omitempty"`
	BootstrapToken                     *string                                            `toml:"bootstrap-token,omitempty"`
	StandaloneMode                     *bool                                              `toml:"standalone-mode,omitempty"`
	ImageMinimumGCAge                  *string                                            `toml:"image-minimum-gc-age,omitempty"`
	ImageMaximumGCAge                  *string                                            `toml:"image-maximum-gc-age,omitempty"`
	ProviderID                         *string                                            `toml:"provider-id,omitempty"`
	ServerCertificate                  *string                                            `toml:"server-certificate,omitempty"`
	ServerKey                          *string                                            `toml:"server-key,omitempty"`
	ReservedCPUs                       *string                                            `toml:"reserved-cpus,omitempty"`
	StaticPodsEnabled                  *bool                                              `toml:"static-pods-enabled,omitempty"`
	NodeIP                             *string                                            `toml:"node-ip,omitempty"`
	PodInfraContainerImage             *string                                            `toml:"pod-infra-container-image,omitempty"`
	HostnameOverride                   *string                                            `toml:"hostname-override,omitempty"`
	MaxParallelImagePulls              *int                                               `toml:"max-parallel-image-pulls,omitempty"`
	ContainerRuntimeEndpoint           *string                                            `toml:"container-runtime-endpoint,omitempty"`
}
