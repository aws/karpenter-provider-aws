package containerd

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"text/template"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/util"
	"go.uber.org/zap"
)

const ContainerRuntimeEndpoint = "unix:///run/containerd/containerd.sock"

const (
	containerdConfigFile      = "/etc/containerd/config.toml"
	containerdConfigImportDir = "/etc/containerd/config.d"
	containerdConfigPerm      = 0644
)

var (
	//go:embed config.template.toml
	containerdConfigTemplateData string
	containerdConfigTemplate     = template.Must(template.New(containerdConfigFile).Parse(containerdConfigTemplateData))
)

type containerdTemplateVars struct {
	SandboxImage string
}

func writeContainerdConfig(cfg *api.NodeConfig) error {
	// write nodeadm's generated containerd config to the default path
	containerdConfig, err := generateContainerdConfig(cfg)
	if err != nil {
		return err
	}
	zap.L().Info("Writing containerd config to file..", zap.String("path", containerdConfigFile))
	if err := util.WriteFileWithDir(containerdConfigFile, containerdConfig, containerdConfigPerm); err != nil {
		return err
	}
	if len(cfg.Spec.Containerd.Config) > 0 {
		containerConfigImportPath := filepath.Join(containerdConfigImportDir, "00-nodeadm.toml")
		zap.L().Info("Writing user containerd config to drop-in file..", zap.String("path", containerConfigImportPath))
		return util.WriteFileWithDir(containerConfigImportPath, []byte(cfg.Spec.Containerd.Config), containerdConfigPerm)
	}
	return nil
}

func generateContainerdConfig(cfg *api.NodeConfig) ([]byte, error) {
	configVars := containerdTemplateVars{
		SandboxImage: cfg.Status.Defaults.SandboxImage,
	}
	var buf bytes.Buffer
	if err := containerdConfigTemplate.Execute(&buf, configVars); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
