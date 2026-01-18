package system

import (
	"os"
	"os/exec"
	"strings"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"go.uber.org/zap"
)

const localDiskAspectName = "local-disk"

func NewLocalDiskAspect() SystemAspect {
	return &localDiskAspect{}
}

type localDiskAspect struct{}

func (a *localDiskAspect) Name() string {
	return localDiskAspectName
}

func (a *localDiskAspect) Setup(cfg *api.NodeConfig) error {
	if cfg.Spec.Instance.LocalStorage.Strategy == "" {
		zap.L().Info("Not configuring local disks!")
		return nil
	}
	strategy := strings.ToLower(string(cfg.Spec.Instance.LocalStorage.Strategy))
	// #nosec G204 Subprocess launched with variable
	cmd := exec.Command("setup-local-disks", strategy)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
