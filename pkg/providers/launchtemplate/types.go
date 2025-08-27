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

package launchtemplate

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/options"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Provider interface {
	EnsureAll(context.Context, *v1.EC2NodeClass, *karpv1.NodeClaim,
		[]*cloudprovider.InstanceType, string, map[string]string) ([]*LaunchTemplate, error)
	EnsureLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (ec2types.LaunchTemplate, error)
	DeleteAll(context.Context, *v1.EC2NodeClass) error
	InvalidateCache(context.Context, string, string)
	ResolveClusterCIDR(context.Context) error
	CreateAMIOptions(context.Context, *v1.EC2NodeClass, map[string]string, map[string]string) (*amifamily.Options, error)
}

type LaunchTemplate struct {
	Name                  string
	InstanceTypes         []*cloudprovider.InstanceType
	ImageID               string
	CapacityReservationID string
}

type LaunchMode int

const (
	LaunchModeOpen LaunchMode = iota
	LaunchModeTargeted
)

type LaunchModeProvider interface {
	LaunchMode(context.Context) LaunchMode
}

type defaultLaunchModeProvider struct{}

func (defaultLaunchModeProvider) LaunchMode(ctx context.Context) LaunchMode {
	if options.FromContext(ctx).FeatureGates.ReservedCapacity {
		return LaunchModeTargeted
	}
	return LaunchModeOpen
}

type CreateLaunchTemplateInputBuilder struct {
	LaunchModeProvider
	options         *amifamily.LaunchTemplate
	clusterIPFamily corev1.IPFamily
	userData        string
}

func NewCreateLaunchTemplateInputBuilder(
	options *amifamily.LaunchTemplate,
	clusterIPFamily corev1.IPFamily,
	userData string,
) *CreateLaunchTemplateInputBuilder {
	return &CreateLaunchTemplateInputBuilder{
		LaunchModeProvider: defaultLaunchModeProvider{},
		options:            options,
		clusterIPFamily:    clusterIPFamily,
		userData:           userData,
	}
}

func (b *CreateLaunchTemplateInputBuilder) WithLaunchModeProvider(provider LaunchModeProvider) *CreateLaunchTemplateInputBuilder {
	b.LaunchModeProvider = provider
	return b
}

func (b *CreateLaunchTemplateInputBuilder) Build(ctx context.Context) *ec2.CreateLaunchTemplateInput {
	launchTemplateDataTags := []ec2types.LaunchTemplateTagSpecificationRequest{{
		ResourceType: ec2types.ResourceTypeNetworkInterface,
		Tags:         utils.EC2MergeTags(b.options.Tags),
	}}
	if b.options.CapacityType == karpv1.CapacityTypeSpot {
		launchTemplateDataTags = append(launchTemplateDataTags, ec2types.LaunchTemplateTagSpecificationRequest{
			ResourceType: ec2types.ResourceTypeSpotInstancesRequest,
			Tags:         utils.EC2MergeTags(b.options.Tags),
		})
	}
	networkInterfaces := generateNetworkInterfaces(b.options, b.clusterIPFamily)
	lt := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: lo.ToPtr(LaunchTemplateName(b.options)),
		LaunchTemplateData: &ec2types.RequestLaunchTemplateData{
			BlockDeviceMappings: blockDeviceMappings(b.options.BlockDeviceMappings),
			IamInstanceProfile: &ec2types.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: lo.ToPtr(b.options.InstanceProfile),
			},
			Monitoring: &ec2types.LaunchTemplatesMonitoringRequest{
				Enabled: lo.ToPtr(b.options.DetailedMonitoring),
			},
			// If the network interface is defined, the security groups are defined within it
			SecurityGroupIds: lo.Ternary(networkInterfaces != nil, nil, lo.Map(b.options.SecurityGroups, func(s v1.SecurityGroup, _ int) string { return s.ID })),
			UserData:         lo.ToPtr(b.userData),
			ImageId:          lo.ToPtr(b.options.AMIID),
			MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptionsRequest{
				HttpEndpoint:     ec2types.LaunchTemplateInstanceMetadataEndpointState(lo.FromPtr(b.options.MetadataOptions.HTTPEndpoint)),
				HttpProtocolIpv6: ec2types.LaunchTemplateInstanceMetadataProtocolIpv6(lo.FromPtr(b.options.MetadataOptions.HTTPProtocolIPv6)),
				//Will be removed when we update options.MetadataOptions.HTTPPutResponseHopLimit type to be int32
				//nolint: gosec
				HttpPutResponseHopLimit: lo.ToPtr(int32(lo.FromPtr(b.options.MetadataOptions.HTTPPutResponseHopLimit))),
				HttpTokens:              ec2types.LaunchTemplateHttpTokensState(lo.FromPtr(b.options.MetadataOptions.HTTPTokens)),
				// We statically set the InstanceMetadataTags to "disabled" for all new instances since
				// account-wide defaults can override instance defaults on metadata settings
				// This can cause instance failure on accounts that default to instance tags since Karpenter
				// can't support instance tags with its current tags (e.g. kubernetes.io/cluster/*, karpenter.k8s.aws/ec2nodeclass)
				// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-options.html#instance-metadata-options-order-of-precedence
				InstanceMetadataTags: ec2types.LaunchTemplateInstanceMetadataTagsStateDisabled,
			},
			NetworkInterfaces: networkInterfaces,
			TagSpecifications: launchTemplateDataTags,
		},
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeLaunchTemplate,
				Tags:         utils.EC2MergeTags(b.options.Tags),
			},
		},
	}
	// Gate this specifically since the update to CapacityReservationPreference will opt od / spot launches out of open
	// ODCRs, which is a breaking change from the pre-native ODCR support behavior.
	if b.LaunchMode(ctx) == LaunchModeTargeted {
		lt.LaunchTemplateData.CapacityReservationSpecification = &ec2types.LaunchTemplateCapacityReservationSpecificationRequest{
			CapacityReservationPreference: lo.Ternary(
				b.options.CapacityType == karpv1.CapacityTypeReserved,
				ec2types.CapacityReservationPreferenceCapacityReservationsOnly,
				ec2types.CapacityReservationPreferenceNone,
			),
			CapacityReservationTarget: lo.Ternary(
				b.options.CapacityType == karpv1.CapacityTypeReserved,
				&ec2types.CapacityReservationTarget{
					CapacityReservationId: &b.options.CapacityReservationID,
				},
				nil,
			),
		}
		if b.options.CapacityReservationType == v1.CapacityReservationTypeCapacityBlock {
			lt.LaunchTemplateData.InstanceMarketOptions = &ec2types.LaunchTemplateInstanceMarketOptionsRequest{
				MarketType: ec2types.MarketTypeCapacityBlock,
			}
		}
	}
	return lt
}
