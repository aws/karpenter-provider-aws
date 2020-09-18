package environment

import (
	"github.com/ellistarn/karpenter/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Environment interface {
	Start() error
	Stop() error
	GetClient() client.Client
	NewNamespace() (*test.Namespace, error)
}
