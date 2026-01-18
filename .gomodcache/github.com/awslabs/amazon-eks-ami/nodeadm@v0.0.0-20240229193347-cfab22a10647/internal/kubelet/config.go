package kubelet

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"dario.cat/mergo"

	"go.uber.org/zap"
	"golang.org/x/mod/semver"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8skubelet "k8s.io/kubelet/config/v1beta1"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/smithy-go/ptr"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/containerd"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/system"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/util"
)

const (
	kubeletConfigRoot = "/etc/kubernetes/kubelet"
	kubeletConfigFile = "config.json"
	kubeletConfigDir  = "config.json.d"
	kubeletConfigPerm = 0644
)

func (k *kubelet) writeKubeletConfig(cfg *api.NodeConfig) error {
	kubeletVersion, err := GetKubeletVersion()
	if err != nil {
		return err
	}
	// tracking: https://github.com/kubernetes/enhancements/issues/3983
	// for enabling drop-in configuration
	if semver.Compare(kubeletVersion, "v1.29.0") < 0 {
		return k.writeKubeletConfigToFile(cfg)
	} else {
		return k.writeKubeletConfigToDir(cfg)
	}
}

// kubeletConfig is an internal-only representation of the kubelet configuration
// that is generated using sane defaults for EKS. It is a subset of the upstream
// KubeletConfiguration types:
// https://pkg.go.dev/k8s.io/kubelet/config/v1beta1#KubeletConfiguration
type kubeletConfig struct {
	Address                  string                           `json:"address"`
	Authentication           k8skubelet.KubeletAuthentication `json:"authentication"`
	Authorization            k8skubelet.KubeletAuthorization  `json:"authorization"`
	CgroupDriver             string                           `json:"cgroupDriver"`
	CgroupRoot               string                           `json:"cgroupRoot"`
	ClusterDNS               []string                         `json:"clusterDNS"`
	ClusterDomain            string                           `json:"clusterDomain"`
	ContainerRuntimeEndpoint string                           `json:"containerRuntimeEndpoint"`
	EvictionHard             map[string]string                `json:"evictionHard,omitempty"`
	FeatureGates             map[string]bool                  `json:"featureGates"`
	HairpinMode              string                           `json:"hairpinMode"`
	KubeAPIBurst             *int                             `json:"kubeAPIBurst,omitempty"`
	KubeAPIQPS               *int                             `json:"kubeAPIQPS,omitempty"`
	KubeReserved             map[string]string                `json:"kubeReserved,omitempty"`
	KubeReservedCgroup       *string                          `json:"kubeReservedCgroup,omitempty"`
	Logging                  loggingConfiguration             `json:"logging"`
	MaxPods                  int32                            `json:"maxPods,omitempty"`
	ProtectKernelDefaults    bool                             `json:"protectKernelDefaults"`
	ProviderID               *string                          `json:"providerID,omitempty"`
	ReadOnlyPort             int                              `json:"readOnlyPort"`
	RegisterWithTaints       []v1.Taint                       `json:"registerWithTaints,omitempty"`
	SerializeImagePulls      bool                             `json:"serializeImagePulls"`
	ServerTLSBootstrap       bool                             `json:"serverTLSBootstrap"`
	SystemReservedCgroup     *string                          `json:"systemReservedCgroup,omitempty"`
	TLSCipherSuites          []string                         `json:"tlsCipherSuites"`
	metav1.TypeMeta          `json:",inline"`
}

type loggingConfiguration struct {
	Verbosity int `json:"verbosity"`
}

// Creates an internal kubelet configuration from the public facing bootstrap
// kubelet configuration with additional sane defaults.
func defaultKubeletSubConfig() kubeletConfig {
	return kubeletConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeletConfiguration",
			APIVersion: "kubelet.config.k8s.io/v1beta1",
		},
		Address: "0.0.0.0",
		Authentication: k8skubelet.KubeletAuthentication{
			Anonymous: k8skubelet.KubeletAnonymousAuthentication{
				Enabled: ptr.Bool(false),
			},
			Webhook: k8skubelet.KubeletWebhookAuthentication{
				Enabled:  ptr.Bool(true),
				CacheTTL: metav1.Duration{Duration: time.Minute * 2},
			},
			X509: k8skubelet.KubeletX509Authentication{
				ClientCAFile: caCertificatePath,
			},
		},
		Authorization: k8skubelet.KubeletAuthorization{
			Mode: "Webhook",
			Webhook: k8skubelet.KubeletWebhookAuthorization{
				CacheAuthorizedTTL:   metav1.Duration{Duration: time.Minute * 5},
				CacheUnauthorizedTTL: metav1.Duration{Duration: time.Second * 30},
			},
		},
		CgroupDriver:             "systemd",
		CgroupRoot:               "/",
		ClusterDomain:            "cluster.local",
		ContainerRuntimeEndpoint: containerd.ContainerRuntimeEndpoint,
		EvictionHard: map[string]string{
			"memory.available":  "100Mi",
			"nodefs.available":  "10%",
			"nodefs.inodesFree": "5%",
		},
		FeatureGates: map[string]bool{
			"RotateKubeletServerCertificate": true,
		},
		HairpinMode:           "hairpin-veth",
		ProtectKernelDefaults: true,
		ReadOnlyPort:          0,
		Logging: loggingConfiguration{
			Verbosity: 2,
		},
		SerializeImagePulls: false,
		ServerTLSBootstrap:  true,
		TLSCipherSuites: []string{
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			"TLS_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_RSA_WITH_AES_256_GCM_SHA384",
		},
	}
}

// Update the ClusterDNS of the internal kubelet config using a heuristic based
// on the cluster service IP CIDR address.
func (ksc *kubeletConfig) withFallbackClusterDns(cluster *api.ClusterDetails) error {
	clusterDns, err := cluster.GetClusterDns()
	if err != nil {
		return err
	}
	ksc.ClusterDNS = []string{clusterDns}
	return nil
}

// To support worker nodes to continue to communicate and connect to local cluster even when the Outpost
// is disconnected from the parent AWS Region, the following specific setup are required:
//   - append entries to /etc/hosts with the mappings of control plane host IP address and API server
//     domain name. So that the domain name can be resolved to IP addresses locally.
//   - use aws-iam-authenticator as bootstrap auth for kubelet TLS bootstrapping which downloads client
//     X.509 certificate and generate kubelet kubeconfig file which uses the client cert. So that the
//     worker node can be authentiacated through X.509 certificate which works for both connected and
//     disconnected state.
func (ksc *kubeletConfig) withOutpostSetup(cfg *api.NodeConfig) error {
	if enabled := cfg.Spec.Cluster.EnableOutpost; enabled != nil && *enabled {
		zap.L().Info("Setting up outpost..")

		if cfg.Spec.Cluster.ID == "" {
			return fmt.Errorf("clusterId cannot be empty when outpost is enabled.")
		}
		apiUrl, err := url.Parse(cfg.Spec.Cluster.APIServerEndpoint)
		if err != nil {
			return err
		}

		// TODO: cleanup
		ipAddresses, err := net.LookupHost(apiUrl.Host)
		if err != nil {
			return err
		}
		var ipHostMappings []string
		for _, ip := range ipAddresses {
			ipHostMappings = append(ipHostMappings, fmt.Sprintf("%s\t%s", ip, apiUrl.Host))
		}
		output := strings.Join(ipHostMappings, "\n") + "\n"

		if err != nil {
			return err
		}

		// append to /etc/hosts file with shuffled mappings of "IP address to API server domain name"
		f, err := os.OpenFile("/etc/hosts", os.O_APPEND|os.O_WRONLY, kubeletConfigPerm)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.WriteString(output); err != nil {
			return err
		}
	}
	return nil
}

func (ksc *kubeletConfig) withNodeIp(cfg *api.NodeConfig, flags map[string]string) error {
	nodeIp, err := getNodeIp(context.TODO(), imds.New(imds.Options{}), cfg)
	if err != nil {
		return err
	}
	flags["node-ip"] = nodeIp
	zap.L().Info("Setup IP for node", zap.String("ip", nodeIp))
	return nil
}

func (ksc *kubeletConfig) withVersionToggles(kubeletVersion string, flags map[string]string) {
	// TODO: remove when 1.26 is EOL
	if semver.Compare(kubeletVersion, "v1.27.0") < 0 {
		// --container-runtime flag is gone in 1.27+
		flags["container-runtime"] = "remote"
		// --container-runtime-endpoint moved to kubelet config start from 1.27
		// https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.27.md?plain=1#L1800-L1801
		flags["container-runtime-endpoint"] = ksc.ContainerRuntimeEndpoint
	}

	// TODO: Remove this during 1.27 EOL
	// Enable Feature Gate for KubeletCredentialProviders in versions less than 1.28 since this feature flag was removed in 1.28.
	if semver.Compare(kubeletVersion, "v1.28.0") < 0 {
		ksc.FeatureGates["KubeletCredentialProviders"] = true
	}

	// for K8s versions that suport API Priority & Fairness, increase our API server QPS
	// in 1.27, the default is already increased to 50/100, so use the higher defaults
	if semver.Compare(kubeletVersion, "v1.22.0") >= 0 && semver.Compare(kubeletVersion, "v1.27.0") < 0 {
		ksc.KubeAPIQPS = ptr.Int(10)
		ksc.KubeAPIBurst = ptr.Int(20)
	}
}

func (ksc *kubeletConfig) withCloudProvider(kubeletVersion string, cfg *api.NodeConfig, flags map[string]string) {
	if semver.Compare(kubeletVersion, "v1.26.0") >= 0 {
		// ref: https://github.com/kubernetes/kubernetes/pull/121367
		flags["cloud-provider"] = "external"
		// provider ID needs to be specified when the cloud provider is external
		ksc.ProviderID = ptr.String(getProviderId(cfg.Status.Instance.AvailabilityZone, cfg.Status.Instance.ID))
		// TODO set the --hostname-override equal to the EC2 PrivateDnsName
	} else {
		flags["cloud-provider"] = "aws"
	}
}

// When the DefaultReservedResources flag is enabled, override the kubelet
// config with reserved cgroup values on behalf of the user
func (ksc *kubeletConfig) withDefaultReservedResources(cfg *api.NodeConfig) {
	ksc.SystemReservedCgroup = ptr.String("/system")
	ksc.KubeReservedCgroup = ptr.String("/runtime")
	maxPods, ok := MaxPodsPerInstanceType[cfg.Status.Instance.Type]
	if !ok {
		ksc.MaxPods = CalcMaxPods(cfg.Status.Instance.Region, cfg.Status.Instance.Type)
	} else {
		ksc.MaxPods = int32(maxPods)
	}
	ksc.KubeReserved = map[string]string{
		"cpu":               fmt.Sprintf("%dm", getCPUMillicoresToReserve()),
		"ephemeral-storage": "1Gi",
		"memory":            fmt.Sprintf("%dMi", getMemoryMebibytesToReserve(ksc.MaxPods)),
	}
}

// withPodInfraContainerImage determines whether to add the
// '--pod-infra-container-image' flag, which is used to ensure the sandbox image
// is not garbage collected.
//
// TODO: revisit once the minimum supportted version catches up or the container
// runtime is moved to containerd 2.0
func (ksc *kubeletConfig) withPodInfraContainerImage(cfg *api.NodeConfig, kubeletVersion string, flags map[string]string) error {
	// the flag is a noop on 1.29+, since the behavior was changed to use the
	// CRI image pinning behavior and no longer considers the flag value.
	// see: https://github.com/kubernetes/kubernetes/pull/118544
	if semver.Compare(kubeletVersion, "v1.29.0") < 0 {
		flags["pod-infra-container-image"] = cfg.Status.Defaults.SandboxImage
	}
	return nil
}

func (k *kubelet) GenerateKubeletConfig(cfg *api.NodeConfig) (*kubeletConfig, error) {
	// Get the kubelet/kubernetes version to help conditionally enable features
	kubeletVersion, err := GetKubeletVersion()
	if err != nil {
		return nil, err
	}
	zap.L().Info("Detected kubelet version", zap.String("version", kubeletVersion))

	kubeletConfig := defaultKubeletSubConfig()

	if err := kubeletConfig.withFallbackClusterDns(&cfg.Spec.Cluster); err != nil {
		return nil, err
	}
	if err := kubeletConfig.withOutpostSetup(cfg); err != nil {
		return nil, err
	}
	if err := kubeletConfig.withNodeIp(cfg, k.flags); err != nil {
		return nil, err
	}
	if err := kubeletConfig.withPodInfraContainerImage(cfg, kubeletVersion, k.flags); err != nil {
		return nil, err
	}

	kubeletConfig.withVersionToggles(kubeletVersion, k.flags)
	kubeletConfig.withCloudProvider(kubeletVersion, cfg, k.flags)
	kubeletConfig.withDefaultReservedResources(cfg)

	return &kubeletConfig, nil
}

// WriteConfig writes the kubelet config to a file.
// This should only be used for kubelet versions < 1.28.
func (k *kubelet) writeKubeletConfigToFile(cfg *api.NodeConfig) error {
	kubeletConfig, err := k.GenerateKubeletConfig(cfg)
	if err != nil {
		return err
	}

	var kubeletConfigBytes []byte
	if cfg.Spec.Kubelet.Config != nil && len(cfg.Spec.Kubelet.Config) > 0 {
		mergedMap, err := util.DocumentMerge(kubeletConfig, cfg.Spec.Kubelet.Config, mergo.WithOverride)
		if err != nil {
			return err
		}
		if kubeletConfigBytes, err = json.MarshalIndent(mergedMap, "", strings.Repeat(" ", 4)); err != nil {
			return err
		}
	} else {
		var err error
		if kubeletConfigBytes, err = json.MarshalIndent(kubeletConfig, "", strings.Repeat(" ", 4)); err != nil {
			return err
		}
	}

	configPath := path.Join(kubeletConfigRoot, kubeletConfigFile)
	k.flags["config"] = configPath

	zap.L().Info("Writing kubelet config to file..", zap.String("path", configPath))
	return util.WriteFileWithDir(configPath, kubeletConfigBytes, kubeletConfigPerm)
}

// WriteKubeletConfigToDir writes nodeadm's generated kubelet config to the
// standard config file and writes the user's provided config to a directory for
// drop-in support. This is only supported on kubelet versions >= 1.28. see:
// https://kubernetes.io/docs/tasks/administer-cluster/kubelet-config-file/#kubelet-conf-d
func (k *kubelet) writeKubeletConfigToDir(cfg *api.NodeConfig) error {
	kubeletConfig, err := k.GenerateKubeletConfig(cfg)
	if err != nil {
		return err
	}
	kubeletConfigBytes, err := json.MarshalIndent(kubeletConfig, "", strings.Repeat(" ", 4))
	if err != nil {
		return err
	}

	configPath := path.Join(kubeletConfigRoot, kubeletConfigFile)
	k.flags["config"] = configPath

	zap.L().Info("Writing kubelet config to file..", zap.String("path", configPath))
	if err := util.WriteFileWithDir(configPath, kubeletConfigBytes, kubeletConfigPerm); err != nil {
		return err
	}

	if cfg.Spec.Kubelet.Config != nil && len(cfg.Spec.Kubelet.Config) > 0 {
		dirPath := path.Join(kubeletConfigRoot, kubeletConfigDir)
		k.flags["config-dir"] = dirPath

		zap.L().Info("Enabling kubelet config drop-in dir..")
		k.setEnv("KUBELET_CONFIG_DROPIN_DIR_ALPHA", "on")
		filePath := path.Join(dirPath, "00-nodeadm.conf")

		// merge in default type metadata like kind and apiVersion in case the
		// user has not specified this, as it is required to qualify a drop-in
		// config as a valid KubeletConfiguration
		userKubeletConfigMap, err := util.DocumentMerge(defaultKubeletSubConfig().TypeMeta, cfg.Spec.Kubelet.Config)
		if err != nil {
			return err
		}

		zap.L().Info("Writing user kubelet config to drop-in file..", zap.String("path", filePath))
		userKubeletConfigBytes, err := json.MarshalIndent(userKubeletConfigMap, "", strings.Repeat(" ", 4))
		if err != nil {
			return err
		}
		if err := util.WriteFileWithDir(filePath, userKubeletConfigBytes, kubeletConfigPerm); err != nil {
			return err
		}
	}

	return nil
}

func getProviderId(availabilityZone, instanceId string) string {
	return fmt.Sprintf("aws:///%s/%s", availabilityZone, instanceId)
}

// Get the IP of the node depending on the ipFamily configured for the cluster
func getNodeIp(ctx context.Context, imdsClient *imds.Client, cfg *api.NodeConfig) (string, error) {
	ipFamily, err := api.GetCIDRIpFamily(cfg.Spec.Cluster.CIDR)
	if err != nil {
		return "", err
	}
	switch ipFamily {
	case api.IPFamilyIPv4:
		ipv4Response, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: "local-ipv4",
		})
		if err != nil {
			return "", err
		}
		ip, err := io.ReadAll(ipv4Response.Content)
		if err != nil {
			return "", err
		}
		return string(ip), nil
	case api.IPFamilyIPv6:
		ipv6Response, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: fmt.Sprintf("network/interfaces/macs/%s/ipv6s", cfg.Status.Instance.MAC),
		})
		if err != nil {
			return "", err
		}
		ip, err := io.ReadAll(ipv6Response.Content)
		if err != nil {
			return "", err
		}
		return string(ip), nil
	default:
		return "", fmt.Errorf("invalid ip-family. %s is not one of %v", ipFamily, []api.IPFamily{api.IPFamilyIPv4, api.IPFamilyIPv6})
	}
}

func getCPUMillicoresToReserve() int {
	totalCPUMillicores, err := system.GetMilliNumCores()
	if err != nil {
		zap.L().Error("Error found when GetMilliNumCores", zap.Error(err))
		return 0
	}
	cpuRanges := []int{0, 1000, 2000, 4000, totalCPUMillicores}
	cpuPercentageReservedForRanges := []int{600, 100, 50, 25}
	cpuToReserve := 0

	for i, percentageToReserveForRange := range cpuPercentageReservedForRanges {
		startRange := cpuRanges[i]
		endRange := cpuRanges[i+1]
		cpuToReserve += getResourceToReserveInRange(totalCPUMillicores, startRange, endRange, percentageToReserveForRange)
	}

	return cpuToReserve
}

// getResourceToReserveInRange calculates the CPU resources to reserve for a given range.
func getResourceToReserveInRange(totalCPU, startRange, endRange, percentage int) int {
	if totalCPU <= startRange {
		return 0
	}
	reserved := totalCPU
	if reserved > endRange {
		reserved = endRange
	}
	return (reserved - startRange) * percentage / 10000
}

func getMemoryMebibytesToReserve(maxPods int32) int32 {
	return 11*maxPods + 255
}
