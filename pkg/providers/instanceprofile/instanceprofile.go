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

package instanceprofile

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	awserrors "github.com/aws/karpenter/pkg/errors"
)

var (
	instanceStateFilter = &ec2.Filter{
		Name:   aws.String("instance-state-name"),
		Values: aws.StringSlice([]string{ec2.InstanceStateNamePending, ec2.InstanceStateNameRunning, ec2.InstanceStateNameStopping, ec2.InstanceStateNameStopped, ec2.InstanceStateNameShuttingDown}),
	}
)

type Provider struct {
	region string
	iamapi iamiface.IAMAPI
	ec2api ec2iface.EC2API
	cache  *cache.Cache
}

func NewProvider(region string, iamapi iamiface.IAMAPI, ec2api ec2iface.EC2API, cache *cache.Cache) *Provider {
	return &Provider{
		region: region,
		iamapi: iamapi,
		ec2api: ec2api,
		cache:  cache,
	}
}

func (p *Provider) Create(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, tags map[string]string) (string, error) {
	localTags := lo.Assign(tags, map[string]string{v1beta1.LabelNodeClass: nodeClass.Name, v1.LabelTopologyRegion: p.region})
	profileName := GetProfileName(ctx, p.region, nodeClass)
	delete(localTags, corev1beta1.NodePoolLabelKey)

	// An instance profile exists for this NodeClass
	if _, ok := p.cache.Get(string(nodeClass.UID)); ok {
		return profileName, nil
	}
	// Validate if the instance profile exists and has the correct role assigned to it
	var instanceProfile *iam.InstanceProfile
	out, err := p.iamapi.GetInstanceProfileWithContext(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: aws.String(profileName)})
	if err != nil {
		if !awserrors.IsNotFound(err) {
			return "", fmt.Errorf("getting instance profile %q, %w", profileName, err)
		}
		o, err := p.iamapi.CreateInstanceProfileWithContext(ctx, &iam.CreateInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
			Tags:                lo.MapToSlice(tags, func(k, v string) *iam.Tag { return &iam.Tag{Key: aws.String(k), Value: aws.String(v)} }),
		})
		if err != nil {
			return "", fmt.Errorf("creating instance profile %q, %w", profileName, err)
		}
		instanceProfile = o.InstanceProfile
	} else {
		instanceProfile = out.InstanceProfile
	}
	if len(instanceProfile.Roles) == 1 {
		return profileName, nil
	}
	if _, err = p.iamapi.AddRoleToInstanceProfileWithContext(ctx, &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
		RoleName:            aws.String(nodeClass.Spec.Role),
	}); err != nil {
		return "", fmt.Errorf("adding role %q to instance profile %q, %w", nodeClass.Spec.Role, profileName, err)
	}
	p.cache.SetDefault(string(nodeClass.UID), nil)
	return profileName, nil
}

func (p *Provider) AssociatedInstances(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) ([]string, error) {
	profileName := GetProfileName(ctx, p.region, nodeClass)

	// Get all instances that are using our instance profile name and are not yet terminated
	var ids []string
	if err := p.ec2api.DescribeInstancesPagesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("iam-instance-profile.name"),
				Values: aws.StringSlice([]string{profileName}),
			},
			instanceStateFilter,
		},
	}, func(page *ec2.DescribeInstancesOutput, _ bool) bool {
		for _, res := range page.Reservations {
			ids = append(ids, lo.Map(res.Instances, func(i *ec2.Instance, _ int) string {
				return aws.StringValue(i.InstanceId)
			})...)
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("getting associated instances for instance profile %q, %w", profileName, err)
	}
	return ids, nil
}

func (p *Provider) Delete(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) error {
	profileName := GetProfileName(ctx, p.region, nodeClass)
	out, err := p.iamapi.GetInstanceProfileWithContext(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	})
	if err != nil {
		return awserrors.IgnoreNotFound(fmt.Errorf("getting instance profile %q, %w", profileName, err))
	}
	if len(out.InstanceProfile.Roles) > 0 {
		for _, role := range out.InstanceProfile.Roles {
			if _, err = p.iamapi.RemoveRoleFromInstanceProfileWithContext(ctx, &iam.RemoveRoleFromInstanceProfileInput{
				InstanceProfileName: aws.String(profileName),
				RoleName:            role.RoleName,
			}); err != nil {
				return fmt.Errorf("removing role %q from instance profile %q, %w", aws.StringValue(role.RoleName), profileName, err)
			}
		}
	}
	if _, err = p.iamapi.DeleteInstanceProfileWithContext(ctx, &iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	}); err != nil {
		return awserrors.IgnoreNotFound(fmt.Errorf("deleting instance profile %q, %w", profileName, err))
	}
	return nil
}

func GetProfileName(ctx context.Context, region string, nodeClass *v1beta1.EC2NodeClass) string {
	return fmt.Sprintf("%s/%d", settings.FromContext(ctx).ClusterName, lo.Must(hashstructure.Hash(fmt.Sprintf("%s/%s", nodeClass.Name, region), hashstructure.FormatV2, &hashstructure.HashOptions{})))
}
