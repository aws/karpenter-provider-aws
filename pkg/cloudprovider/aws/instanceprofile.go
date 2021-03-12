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

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KarpenterNodeInstanceProfileName = "KarpenterNodeInstanceProfile"
)

type InstanceProfileProvider struct {
	iam                  iamiface.IAMAPI
	kubeClient           client.Client
	instanceProfileCache *cache.Cache
}

func NewInstanceProfileProvider(iam iamiface.IAMAPI, kubeClient client.Client) *InstanceProfileProvider {
	return &InstanceProfileProvider{
		iam:                  iam,
		kubeClient:           kubeClient,
		instanceProfileCache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (p *InstanceProfileProvider) Get(ctx context.Context, cluster *v1alpha1.ClusterSpec) (*iam.InstanceProfile, error) {
	if instanceProfile, ok := p.instanceProfileCache.Get(cluster.Name); ok {
		return instanceProfile.(*iam.InstanceProfile), nil
	}
	return p.getInstanceProfile(ctx, cluster)
}

func (p *InstanceProfileProvider) getInstanceProfile(ctx context.Context, cluster *v1alpha1.ClusterSpec) (*iam.InstanceProfile, error) {
	output, err := p.iam.GetInstanceProfileWithContext(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(KarpenterNodeInstanceProfileName),
	})
	if err != nil {
		return nil, fmt.Errorf("retriving instance profile %s, %w", KarpenterNodeInstanceProfileName, err)
	}
	for _, role := range output.InstanceProfile.Roles {
		if err := p.addToAWSAuthConfigmap(role); err != nil {
			return nil, fmt.Errorf("adding role %s, %w", *role.RoleName, err)
		}
	}
	zap.S().Debugf("Successfully discovered instance profile %s for cluster %s", *output.InstanceProfile.InstanceProfileName, cluster.Name)
	p.instanceProfileCache.Set(cluster.Name, output.InstanceProfile, CacheTTL)
	return output.InstanceProfile, nil
}

func (p *InstanceProfileProvider) addToAWSAuthConfigmap(role *iam.Role) error {
	awsAuth := &v1.ConfigMap{}
	if err := p.kubeClient.Get(context.TODO(), types.NamespacedName{Name: "aws-auth", Namespace: "kube-system"}, awsAuth); err != nil {
		return fmt.Errorf("retrieving configmap aws-auth, %w", err)
	}
	if strings.Contains(awsAuth.Data["mapRoles"], *role.Arn) {
		zap.S().Debugf("Successfully detected aws-auth configmap contains roleArn %s", *role.Arn)
		return nil
	}
	// Since the aws-auth configmap is stringly typed, this specific indentation is critical
	awsAuth.Data["mapRoles"] += fmt.Sprintf(`
- groups:
  - system:bootstrappers
  - system:nodes
  rolearn: %s
  username: system:node:{{EC2PrivateDNSName}}`, *role.Arn)
	if err := p.kubeClient.Update(context.TODO(), awsAuth); err != nil {
		return fmt.Errorf("updating configmap aws-auth, %w", err)
	}
	zap.S().Debugf("Successfully patched configmap aws-auth with roleArn %s", *role.Arn)
	return nil
}
