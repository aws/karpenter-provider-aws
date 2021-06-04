package aws

import (
	"context"
	v1 "k8s.io/api/core/v1"
)

// Capacity cloud provider implementation using AWS Fleet.
type Termination struct {
	instanceProvider *InstanceProvider
}

func (t *Termination) Terminate(ctx context.Context, nodes []*v1.Node) error {
	return t.instanceProvider.Terminate(ctx, nodes)
}
