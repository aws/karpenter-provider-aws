package resource

import (
	"context"
	"time"
)

const (
	karpenterClusterNameTag     = "karpenter.sh/managed-by"
	karpenterProvisionerNameTag = "karpenter.sh/provisioner-name"
	karpenterNodePoolTag        = "karpenter.sh/nodepool"
	karpenterLaunchTemplateTag  = "karpenter.k8s.aws/cluster"
	karpenterSecurityGroupTag   = "karpenter.sh/discovery"
	karpenterTestingTag         = "testing/cluster"
	k8sClusterTag               = "cluster.k8s.amazonaws.com/name"
	githubRunURLTag             = "github.com/run-url"
)

// Resource is a resource type that can be cleaned through a cluster clean-up operation
// and through an expiration-based cleanup operation
type Resource interface {
	Type() string
	Get(ctx context.Context, clusterName string) (ids []string, err error)
	GetExpired(ctx context.Context, expirationTime time.Time) (ids []string, err error)
	Cleanup(ctx context.Context, ids []string) (cleaned []string, err error)
}
