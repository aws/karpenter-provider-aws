package configprovider

import (
	"fmt"
	"io"
	"os"

	internalapi "github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	apibridge "github.com/awslabs/amazon-eks-ami/nodeadm/internal/api/bridge"
)

type fileConfigProvider struct {
	path string
}

func NewFileConfigProvider(path string) ConfigProvider {
	return &fileConfigProvider{
		path: path,
	}
}

func (fcs *fileConfigProvider) Provide() (*internalapi.NodeConfig, error) {
	file, err := os.Open(fcs.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, which is not currently supported: %s", fcs.path)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	config, err := apibridge.DecodeNodeConfig(data)
	if err != nil {
		return nil, err
	}
	return config, nil
}
