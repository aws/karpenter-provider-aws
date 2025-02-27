package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	SchemeBuilder.Register(&NodeConfig{}, &NodeConfigList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion

// NodeConfig is the primary configuration object for `nodeadm`.
type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeConfig `json:"items"`
}

type NodeConfigSpec struct {
	Cluster    ClusterDetails    `json:"cluster,omitempty"`
	Containerd ContainerdOptions `json:"containerd,omitempty"`
	Instance   InstanceOptions   `json:"instance,omitempty"`
	Kubelet    KubeletOptions    `json:"kubelet,omitempty"`
}

// ClusterDetails contains the coordinates of your EKS cluster.
// These details can be found using the [DescribeCluster API](https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeCluster.html).
type ClusterDetails struct {
	// Name is the name of your EKS cluster
	Name string `json:"name,omitempty"`

	// APIServerEndpoint is the URL of your EKS cluster's kube-apiserver.
	APIServerEndpoint string `json:"apiServerEndpoint,omitempty"`

	// CertificateAuthority is a base64-encoded string of your cluster's certificate authority chain.
	CertificateAuthority []byte `json:"certificateAuthority,omitempty"`

	// CIDR is your cluster's Pod IP CIDR. This value is used to infer your cluster's DNS address.
	CIDR string `json:"cidr,omitempty"`

	// EnableOutpost determines how your node is configured when running on an AWS Outpost.
	EnableOutpost *bool `json:"enableOutpost,omitempty"`

	// ID is an identifier for your cluster; this is only used when your node is running on an AWS Outpost.
	ID string `json:"id,omitempty"`
}

// KubeletOptions are additional parameters passed to `kubelet`.
type KubeletOptions struct {
	// Config is a [`KubeletConfiguration`](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1/)
	// that will be merged with the defaults.
	Config map[string]runtime.RawExtension `json:"config,omitempty"`

	// Flags are [command-line `kubelet`` arguments](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/).
	// that will be appended to the defaults.
	Flags []string `json:"flags,omitempty"`
}

// ContainerdOptions are additional parameters passed to `containerd`.
type ContainerdOptions struct {
	// Config is inline [`containerd` configuration TOML](https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md)
	// that will be [imported](https://github.com/containerd/containerd/blob/32169d591dbc6133ef7411329b29d0c0433f8c4d/docs/man/containerd-config.toml.5.md?plain=1#L146-L154)
	// by the default configuration file.
	Config string `json:"config,omitempty"`
}

// InstanceOptions determines how the node's operating system and devices are configured.
type InstanceOptions struct {
	LocalStorage LocalStorageOptions `json:"localStorage,omitempty"`
}

// LocalStorageOptions control how [EC2 instance stores](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/InstanceStorage.html)
// are used when available.
type LocalStorageOptions struct {
	Strategy LocalStorageStrategy `json:"strategy,omitempty"`
}

// LocalStorageStrategy specifies how to handle an instance's local storage devices.
// +kubebuilder:validation:Enum={RAID0, Mount}
type LocalStorageStrategy string

const (
	// LocalStorageRAID0 will create a single raid0 volume from any local disks
	LocalStorageRAID0 LocalStorageStrategy = "RAID0"

	// LocalStorageMount will mount each local disk individually
	LocalStorageMount LocalStorageStrategy = "Mount"
)
