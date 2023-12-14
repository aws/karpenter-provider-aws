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
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
)

type Provider struct {
	region string
	iamapi iamiface.IAMAPI
	cache  *cache.Cache
}

func NewProvider(region string, iamapi iamiface.IAMAPI, cache *cache.Cache) *Provider {
	return &Provider{
		region: region,
		iamapi: iamapi,
		cache:  cache,
	}
}

func (p *Provider) Create(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (string, error) {
	tags := lo.Assign(nodeClass.Spec.Tags, map[string]string{
		fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName): "owned",
		corev1beta1.ManagedByAnnotationKey:                                            options.FromContext(ctx).ClusterName,
		v1beta1.LabelNodeClass:                                                        nodeClass.Name,
		v1.LabelTopologyRegion:                                                        p.region,
	})
	profileName := GetProfileName(ctx, p.region, nodeClass)

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
	// Instance profiles can only have a single role assigned to them so this profile either has 1 or 0 roles
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html
	if len(instanceProfile.Roles) == 1 {
		if aws.StringValue(instanceProfile.Roles[0].RoleName) == nodeClass.Spec.Role {
			return profileName, nil
		}
		if _, err = p.iamapi.RemoveRoleFromInstanceProfileWithContext(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
			RoleName:            instanceProfile.Roles[0].RoleName,
		}); err != nil {
			return "", fmt.Errorf("removing role %q for instance profile %q, %w", aws.StringValue(instanceProfile.Roles[0].RoleName), profileName, err)
		}
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

func (p *Provider) Delete(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) error {
	profileName := GetProfileName(ctx, p.region, nodeClass)
	out, err := p.iamapi.GetInstanceProfileWithContext(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	})
	if err != nil {
		return awserrors.IgnoreNotFound(fmt.Errorf("getting instance profile %q, %w", profileName, err))
	}
	// Instance profiles can only have a single role assigned to them so this profile either has 1 or 0 roles
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html
	if len(out.InstanceProfile.Roles) == 1 {
		if _, err = p.iamapi.RemoveRoleFromInstanceProfileWithContext(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
			RoleName:            out.InstanceProfile.Roles[0].RoleName,
		}); err != nil {
			return fmt.Errorf("removing role %q from instance profile %q, %w", aws.StringValue(out.InstanceProfile.Roles[0].RoleName), profileName, err)
		}
	}
	if _, err = p.iamapi.DeleteInstanceProfileWithContext(ctx, &iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	}); err != nil {
		return awserrors.IgnoreNotFound(fmt.Errorf("deleting instance profile %q, %w", profileName, err))
	}
	return nil
}

// GetProfileName gets the string for the profile name based on the cluster name and the NodeClass UUID.
// The length of this string can never exceed the maximum instance profile name limit of 128 characters.
func GetProfileName(ctx context.Context, region string, nodeClass *v1beta1.EC2NodeClass) string {
	return fmt.Sprintf("%s_%d", options.FromContext(ctx).ClusterName, lo.Must(hashstructure.Hash(fmt.Sprintf("%s%s", region, nodeClass.Name), hashstructure.FormatV2, nil)))
}
