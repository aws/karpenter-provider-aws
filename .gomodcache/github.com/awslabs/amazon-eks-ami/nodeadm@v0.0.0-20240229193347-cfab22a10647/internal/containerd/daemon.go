package containerd

import (
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/daemon"
)

const ContainerdDaemonName = "containerd"

var _ daemon.Daemon = &containerd{}

type containerd struct {
	daemonManager daemon.DaemonManager
}

func NewContainerdDaemon(daemonManager daemon.DaemonManager) daemon.Daemon {
	return &containerd{
		daemonManager: daemonManager,
	}
}

func (cd *containerd) Configure(c *api.NodeConfig) error {
	return writeContainerdConfig(c)
}

func (cd *containerd) EnsureRunning() error {
	return cd.daemonManager.StartDaemon(ContainerdDaemonName)
}

func (cd *containerd) PostLaunch(c *api.NodeConfig) error {
	return cacheSandboxImage(c)
}

func (cd *containerd) Name() string {
	return ContainerdDaemonName
}
