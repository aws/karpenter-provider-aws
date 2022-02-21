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

package v1alpha5

import (
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

// KubeletConfiguration defines args to be used when configuring kubelet on provisioned nodes.
// They are a subset of the upstream types, recognizing not all options may be supported.
// Wherever possible, the types and names should reflect the upstream kubelet types from
// https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/#kubelet-config-k8s-io-v1beta1-KubeletConfiguration
type KubeletConfiguration struct {
	// ClusterDNS is a list of IP addresses for the cluster DNS server.
	// Note that not all providers may use all addresses.
	//+optional
	ClusterDNS []string `json:"clusterDNS,omitempty"`
	// EventRecordQPS is the maximum event creations per second. If 0,
	// there is no limit enforced.
	EventRecordQPS *int32 `json:"eventRecordQPS,omitempty"`
	// EventBurst is the maximum size of a burst of event creations,
	// temporarily allows event creations to burst to this number, while still not exceeding eventRecordQPS.
	EventBurst *int32 `json:"eventBurst,omitempty"`
	// RegistryPullQPS is the limit of registry pulls per second.
	// The value must not be a negative number. Setting it to 0 means no limit.
	RegistryPullQPS *int32 `json:"registryPullQPS,omitempty"`
	// RegistryBurst is the maximum size of bursty pulls, temporarily allows pulls to burst to this number,
	// while still not exceeding registryPullQPS.
	RegistryBurst *int32 `json:"registryBurst,omitempty"`
	// KubeAPIQPS is the QPS to use while talking with kubernetes apiserver.
	KubeAPIQPS *int32 `json:"kubeAPIQPS,omitempty"`
	// KubeAPIBurst is the burst to allow while talking with kubernetes API server.
	KubeAPIBurst *int32 `json:"kubeAPIBurst,omitempty"`
	// ContainerLogMaxSize is a quantity defining the maximum size of the container log file before it is rotated.
	ContainerLogMaxSize *string `json:"containerLogMaxSize,omitempty"`
	// ContainerLogMaxFiles specifies the maximum number of container log files that can be present for a container.
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`
	// AllowedUnsafeSysctls a comma separated whitelist of unsafe sysctls or sysctl patterns (ending in `∗`).
	AllowedUnsafeSysctls []string `json:"allowedUnsafeSysctls,omitempty"`
	// EvictionHard is a map of signal names to quantities that defines hard eviction thresholds.
	EvictionHard map[string]string `json:"evictionHard,omitempty"`
}

func (k *KubeletConfiguration) validate() (errs *apis.FieldError) {
	return errs.Also(
		addErrIfNegative(k.EventRecordQPS, "eventRecordQPS"),
		addErrIfNegative(k.EventBurst, "eventBurst"),
		addErrIfNegative(k.RegistryPullQPS, "registryPullQPS"),
		addErrIfNegative(k.RegistryBurst, "registryBurst"),
		addErrIfNegative(k.KubeAPIQPS, "kubeAPIQPS"),
		addErrIfNegative(k.KubeAPIBurst, "kubeAPIBurst"),
		addErrIfNegative(k.ContainerLogMaxFiles, "containerLogMaxFiles"),
	)
}

func addErrIfNegative(num *int32, name string) (errs *apis.FieldError) {
	if ptr.Int32Value(num) < 0 {
		return apis.ErrInvalidValue("cannot be negative", name)
	}
	return errs
}
