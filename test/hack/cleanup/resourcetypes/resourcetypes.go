/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resourcetypes

import (
	"context"
	"time"
)

const (
	karpenterClusterNameTag    = "karpenter.sh/managed-by"
	karpenterNodePoolTag       = "karpenter.sh/nodepool"
	karpenterLaunchTemplateTag = "karpenter.k8s.aws/cluster"
	karpenterSecurityGroupTag  = "karpenter.sh/discovery"
	karpenterTestingTag        = "testing/cluster"
	k8sClusterTag              = "cluster.k8s.amazonaws.com/name"
	githubRunURLTag            = "github.com/run-url"
)

// Type is a resource type that can be cleaned through a cluster clean-up operation
// and through an expiration-based cleanup operation
type Type interface {
	// String is the string representation of the type
	String() string
	// Get returns all resources of the type associated with the clusterName
	Get(ctx context.Context, clusterName string) (ids []string, err error)
	// GetExpired returns all resources of the type that were provisioned before the expirationTime
	GetExpired(ctx context.Context, expirationTime time.Time) (ids []string, err error)
	// Cleanup deletes all resources of the type by id and returns the resource ids it succeeded to delete
	// In general, if all resources can't be deleted by id with a single API call (like with DeleteInstances)
	// you should call the requests synchronously to avoid rate limiting against the number of requests made
	Cleanup(ctx context.Context, ids []string) (cleaned []string, err error)
}
