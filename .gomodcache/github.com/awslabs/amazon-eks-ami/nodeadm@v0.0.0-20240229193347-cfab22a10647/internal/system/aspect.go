package system

import "github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"

type SystemAspect interface {
	Name() string
	Setup(*api.NodeConfig) error
}
