package configprovider

import internalapi "github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"

// ConfigProvider is an interface for providing the node configuration.
type ConfigProvider interface {
	// Provide returns the internal version of the source configuration
	Provide() (*internalapi.NodeConfig, error)
}
