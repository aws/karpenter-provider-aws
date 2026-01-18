package kubelet

import (
	"fmt"
	"strings"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/util"
)

const (
	kubeletEnvironmentFilePath = "/etc/eks/kubelet/environment"
	kubeletArgsEnvironmentName = "NODEADM_KUBELET_ARGS"
)

// Write environment variables needed for kubelet runtime. This should be the
// last method called on the kubelet object so that environment side effects of
// other methods are properly recored
func (k *kubelet) writeKubeletEnvironment(cfg *api.NodeConfig) error {
	// transform kubelet flags into a single string and write them to the
	// kubelet environment variable
	var kubeletFlags []string
	for flag, value := range k.flags {
		kubeletFlags = append(kubeletFlags, fmt.Sprintf("--%s=%s", flag, value))
	}
	// append user-provided flags at the end to give them precedence
	kubeletFlags = append(kubeletFlags, cfg.Spec.Kubelet.Flags...)
	// expose these flags via an environment variable scoped to nodeadm
	k.environment[kubeletArgsEnvironmentName] = strings.Join(kubeletFlags, " ")
	// write additional environment variables
	var kubeletEnvironment []string
	for eKey, eValue := range k.environment {
		kubeletEnvironment = append(kubeletEnvironment, fmt.Sprintf(`%s="%s"`, eKey, eValue))
	}
	return util.WriteFileWithDir(kubeletEnvironmentFilePath, []byte(strings.Join(kubeletEnvironment, "\n")), kubeletConfigPerm)
}

// Add values to the environment variables map in a terse manner
func (k *kubelet) setEnv(envName string, envArg string) {
	k.environment[envName] = envArg
}
