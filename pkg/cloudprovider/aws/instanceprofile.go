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
	KarpenterNodeInstanceProfileNameFormat = "KarpenterNodeInstanceProfile-%s"
)

type InstanceProfileProvider struct {
	iamapi     iamiface.IAMAPI
	kubeClient client.Client
	cache      *cache.Cache
}

func NewInstanceProfileProvider(iamapi iamiface.IAMAPI, kubeClient client.Client) *InstanceProfileProvider {
	return &InstanceProfileProvider{
		iamapi:     iamapi,
		kubeClient: kubeClient,
		cache:      cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (p *InstanceProfileProvider) Get(ctx context.Context, cluster *v1alpha1.ClusterSpec) (*iam.InstanceProfile, error) {
	if instanceProfile, ok := p.cache.Get(cluster.Name); ok {
		return instanceProfile.(*iam.InstanceProfile), nil
	}
	return p.getInstanceProfile(ctx, cluster)
}

func (p *InstanceProfileProvider) getInstanceProfile(ctx context.Context, cluster *v1alpha1.ClusterSpec) (*iam.InstanceProfile, error) {
	instanceProfileName := fmt.Sprintf(KarpenterNodeInstanceProfileNameFormat, cluster.Name)
	output, err := p.iamapi.GetInstanceProfileWithContext(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
	})
	if err != nil {
		return nil, fmt.Errorf("retrieving instance profile %s, %w", instanceProfileName, err)
	}
	for _, role := range output.InstanceProfile.Roles {
		if err := p.addToAWSAuthConfigmap(ctx, role); err != nil {
			return nil, fmt.Errorf("adding role %s, %w", *role.RoleName, err)
		}
	}
	zap.S().Debugf("Successfully discovered instance profile %s for cluster %s", *output.InstanceProfile.InstanceProfileName, cluster.Name)
	p.cache.Set(cluster.Name, output.InstanceProfile, CacheTTL)
	return output.InstanceProfile, nil
}

func (p *InstanceProfileProvider) addToAWSAuthConfigmap(ctx context.Context, role *iam.Role) error {
	awsAuth := &v1.ConfigMap{}
	if err := p.kubeClient.Get(ctx, types.NamespacedName{Name: "aws-auth", Namespace: "kube-system"}, awsAuth); err != nil {
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
	if err := p.kubeClient.Update(ctx, awsAuth); err != nil {
		return fmt.Errorf("updating configmap aws-auth, %w", err)
	}
	zap.S().Debugf("Successfully patched configmap aws-auth with roleArn %s", *role.Arn)
	return nil
}
