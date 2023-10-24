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
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	awserrors "github.com/aws/karpenter/pkg/errors"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/instanceprofile"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
	"github.com/aws/karpenter/pkg/utils"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

const (
	launchTemplateNameFormat = "karpenter.k8s.aws/%s"
	karpenterManagedTagKey   = "karpenter.k8s.aws/cluster"
)

type LaunchTemplate struct {
	Name          string
	InstanceTypes []*cloudprovider.InstanceType
	ImageID       string
}

type Provider struct {
	sync.Mutex
	ec2api                  ec2iface.EC2API
	amiFamily               *amifamily.Resolver
	securityGroupProvider   *securitygroup.Provider
	subnetProvider          *subnet.Provider
	instanceProfileProvider *instanceprofile.Provider
	cache                   *cache.Cache
	caBundle                *string
	cm                      *pretty.ChangeMonitor
	KubeDNSIP               net.IP
	ClusterEndpoint         string
}

func NewProvider(ctx context.Context, cache *cache.Cache, ec2api ec2iface.EC2API, amiFamily *amifamily.Resolver,
	securityGroupProvider *securitygroup.Provider, subnetProvider *subnet.Provider, instanceProfileProvider *instanceprofile.Provider,
	caBundle *string, startAsync <-chan struct{}, kubeDNSIP net.IP, clusterEndpoint string) *Provider {
	l := &Provider{
		ec2api:                  ec2api,
		amiFamily:               amiFamily,
		securityGroupProvider:   securityGroupProvider,
		subnetProvider:          subnetProvider,
		instanceProfileProvider: instanceProfileProvider,
		cache:                   cache,
		caBundle:                caBundle,
		cm:                      pretty.NewChangeMonitor(),
		KubeDNSIP:               kubeDNSIP,
		ClusterEndpoint:         clusterEndpoint,
	}
	l.cache.OnEvicted(l.cachedEvictedFunc(ctx))
	go func() {
		// only hydrate cache once elected leader
		select {
		case <-startAsync:
		case <-ctx.Done():
			return
		}
		l.hydrateCache(ctx)
	}()
	return l
}

func (p *Provider) EnsureAll(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, nodeClaim *corev1beta1.NodeClaim,
	instanceTypes []*cloudprovider.InstanceType, capacityType string, tags map[string]string) ([]*LaunchTemplate, error) {

	p.Lock()
	defer p.Unlock()
	// If Launch Template is directly specified then just use it
	if nodeClass.Spec.LaunchTemplateName != nil {
		return []*LaunchTemplate{{Name: ptr.StringValue(nodeClass.Spec.LaunchTemplateName), InstanceTypes: instanceTypes}}, nil
	}

	options, err := p.createAMIOptions(ctx, nodeClass, lo.Assign(nodeClaim.Labels, map[string]string{corev1beta1.CapacityTypeLabelKey: capacityType}), tags)
	if err != nil {
		return nil, err
	}
	resolvedLaunchTemplates, err := p.amiFamily.Resolve(ctx, nodeClass, nodeClaim, instanceTypes, options)
	if err != nil {
		return nil, err
	}
	var launchTemplates []*LaunchTemplate
	for _, resolvedLaunchTemplate := range resolvedLaunchTemplates {
		// Ensure the launch template exists, or create it
		ec2LaunchTemplate, err := p.ensureLaunchTemplate(ctx, capacityType, resolvedLaunchTemplate)
		if err != nil {
			return nil, err
		}
		launchTemplates = append(launchTemplates, &LaunchTemplate{Name: *ec2LaunchTemplate.LaunchTemplateName, InstanceTypes: resolvedLaunchTemplate.InstanceTypes, ImageID: resolvedLaunchTemplate.AMIID})
	}
	return launchTemplates, nil
}

// Invalidate deletes a launch template from cache if it exists
func (p *Provider) Invalidate(ctx context.Context, ltName string, ltID string) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("launch-template-name", ltName, "launch-template-id", ltID))
	p.Lock()
	defer p.Unlock()
	defer p.cache.OnEvicted(p.cachedEvictedFunc(ctx))
	p.cache.OnEvicted(nil)
	logging.FromContext(ctx).Debugf("invalidating launch template in the cache because it no longer exists")
	p.cache.Delete(ltName)
}

func launchTemplateName(options *amifamily.LaunchTemplate) string {
	hash, err := hashstructure.Hash(options, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		panic(fmt.Sprintf("hashing launch template, %s", err))
	}
	return fmt.Sprintf(launchTemplateNameFormat, fmt.Sprint(hash))
}

func (p *Provider) createAMIOptions(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, labels, tags map[string]string) (*amifamily.Options, error) {
	// Remove any labels passed into userData that are prefixed with "node-restriction.kubernetes.io" since the kubelet can't
	// register the node with any labels from this domain: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction
	for k := range labels {
		if strings.HasPrefix(k, v1.LabelNamespaceNodeRestriction) {
			delete(labels, k)
		}
	}
	instanceProfile, err := p.getInstanceProfile(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	// Get constrained security groups
	securityGroups, err := p.securityGroupProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	if len(securityGroups) == 0 {
		return nil, fmt.Errorf("no security groups exist given constraints")
	}
	options := &amifamily.Options{
		ClusterName:             options.FromContext(ctx).ClusterName,
		ClusterEndpoint:         p.ClusterEndpoint,
		AWSENILimitedPodDensity: settings.FromContext(ctx).EnableENILimitedPodDensity,
		InstanceProfile:         instanceProfile,
		SecurityGroups: lo.Map(securityGroups, func(s *ec2.SecurityGroup, _ int) v1beta1.SecurityGroup {
			return v1beta1.SecurityGroup{ID: aws.StringValue(s.GroupId), Name: aws.StringValue(s.GroupName)}
		}),
		Tags:      tags,
		Labels:    labels,
		CABundle:  p.caBundle,
		KubeDNSIP: p.KubeDNSIP,
	}
	if ok, err := p.subnetProvider.CheckAnyPublicIPAssociations(ctx, nodeClass); err != nil {
		return nil, err
	} else if !ok {
		// If all referenced subnets do not assign public IPv4 addresses to EC2 instances therein, we explicitly set
		// AssociatePublicIpAddress to 'false' in the Launch Template, generated based on this configuration struct.
		// This is done to help comply with AWS account policies that require explicitly setting of that field to 'false'.
		// https://github.com/aws/karpenter/issues/3815
		options.AssociatePublicIPAddress = aws.Bool(false)
	}
	return options, nil
}

func (p *Provider) ensureLaunchTemplate(ctx context.Context, capacityType string, options *amifamily.LaunchTemplate) (*ec2.LaunchTemplate, error) {
	var launchTemplate *ec2.LaunchTemplate
	name := launchTemplateName(options)
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("launch-template-name", name))
	// Read from cache
	if launchTemplate, ok := p.cache.Get(name); ok {
		p.cache.SetDefault(name, launchTemplate)
		return launchTemplate.(*ec2.LaunchTemplate), nil
	}
	// Attempt to find an existing LT.
	output, err := p.ec2api.DescribeLaunchTemplatesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{aws.String(name)},
	})
	// Create LT if one doesn't exist
	if awserrors.IsNotFound(err) {
		launchTemplate, err = p.createLaunchTemplate(ctx, capacityType, options)
		if err != nil {
			return nil, fmt.Errorf("creating launch template, %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("describing launch templates, %w", err)
	} else if len(output.LaunchTemplates) != 1 {
		return nil, fmt.Errorf("expected to find one launch template, but found %d", len(output.LaunchTemplates))
	} else {
		if p.cm.HasChanged("launchtemplate-"+name, name) {
			logging.FromContext(ctx).Debugf("discovered launch template")
		}
		launchTemplate = output.LaunchTemplates[0]
	}
	p.cache.SetDefault(name, launchTemplate)
	return launchTemplate, nil
}

func (p *Provider) createLaunchTemplate(ctx context.Context, capacityType string, options *amifamily.LaunchTemplate) (*ec2.LaunchTemplate, error) {
	userData, err := options.UserData.Script()
	if err != nil {
		return nil, err
	}
	launchTemplateDataTags := []*ec2.LaunchTemplateTagSpecificationRequest{
		{ResourceType: aws.String(ec2.ResourceTypeNetworkInterface), Tags: utils.MergeTags(options.Tags)},
	}
	// Add the spot-instances-request tag if trying to launch spot capacity
	if capacityType == corev1beta1.CapacityTypeSpot {
		launchTemplateDataTags = append(launchTemplateDataTags, &ec2.LaunchTemplateTagSpecificationRequest{ResourceType: aws.String(ec2.ResourceTypeSpotInstancesRequest), Tags: utils.MergeTags(options.Tags)})
	}
	networkInterface := p.generateNetworkInterface(options)
	output, err := p.ec2api.CreateLaunchTemplateWithContext(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(launchTemplateName(options)),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			BlockDeviceMappings: p.blockDeviceMappings(options.BlockDeviceMappings),
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(options.InstanceProfile),
			},
			Monitoring: &ec2.LaunchTemplatesMonitoringRequest{
				Enabled: aws.Bool(options.DetailedMonitoring),
			},
			// If the network interface is defined, the security groups are defined within it
			SecurityGroupIds: lo.Ternary(networkInterface != nil, nil, lo.Map(options.SecurityGroups, func(s v1beta1.SecurityGroup, _ int) *string { return aws.String(s.ID) })),
			UserData:         aws.String(userData),
			ImageId:          aws.String(options.AMIID),
			MetadataOptions: &ec2.LaunchTemplateInstanceMetadataOptionsRequest{
				HttpEndpoint:            options.MetadataOptions.HTTPEndpoint,
				HttpProtocolIpv6:        options.MetadataOptions.HTTPProtocolIPv6,
				HttpPutResponseHopLimit: options.MetadataOptions.HTTPPutResponseHopLimit,
				HttpTokens:              options.MetadataOptions.HTTPTokens,
			},
			NetworkInterfaces: networkInterface,
			TagSpecifications: launchTemplateDataTags,
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeLaunchTemplate),
				Tags:         utils.MergeTags(options.Tags, map[string]string{karpenterManagedTagKey: options.ClusterName}),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).With("id", aws.StringValue(output.LaunchTemplate.LaunchTemplateId)).Debugf("created launch template")
	return output.LaunchTemplate, nil
}

// generateNetworkInterface generates a network interface for the launch template.
// If all referenced subnets do not assign public IPv4 addresses to EC2 instances therein, we explicitly set
// AssociatePublicIpAddress to 'false' in the Launch Template, generated based on this configuration struct.
// This is done to help comply with AWS account policies that require explicitly setting that field to 'false'.
// https://github.com/aws/karpenter/issues/3815
func (p *Provider) generateNetworkInterface(options *amifamily.LaunchTemplate) []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest {
	if options.AssociatePublicIPAddress != nil {
		return []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
			{
				AssociatePublicIpAddress: options.AssociatePublicIPAddress,
				DeviceIndex:              aws.Int64(0),
				Groups:                   lo.Map(options.SecurityGroups, func(s v1beta1.SecurityGroup, _ int) *string { return aws.String(s.ID) }),
			},
		}
	}
	return nil
}

func (p *Provider) blockDeviceMappings(blockDeviceMappings []*v1beta1.BlockDeviceMapping) []*ec2.LaunchTemplateBlockDeviceMappingRequest {
	if len(blockDeviceMappings) == 0 {
		// The EC2 API fails with empty slices and expects nil.
		return nil
	}
	var blockDeviceMappingsRequest []*ec2.LaunchTemplateBlockDeviceMappingRequest
	for _, blockDeviceMapping := range blockDeviceMappings {
		blockDeviceMappingsRequest = append(blockDeviceMappingsRequest, &ec2.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName: blockDeviceMapping.DeviceName,
			Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
				DeleteOnTermination: blockDeviceMapping.EBS.DeleteOnTermination,
				Encrypted:           blockDeviceMapping.EBS.Encrypted,
				VolumeType:          blockDeviceMapping.EBS.VolumeType,
				Iops:                blockDeviceMapping.EBS.IOPS,
				Throughput:          blockDeviceMapping.EBS.Throughput,
				KmsKeyId:            blockDeviceMapping.EBS.KMSKeyID,
				SnapshotId:          blockDeviceMapping.EBS.SnapshotID,
				VolumeSize:          p.volumeSize(blockDeviceMapping.EBS.VolumeSize),
			},
		})
	}
	return blockDeviceMappingsRequest
}

// volumeSize returns a GiB scaled value from a resource quantity or nil if the resource quantity passed in is nil
func (p *Provider) volumeSize(quantity *resource.Quantity) *int64 {
	if quantity == nil {
		return nil
	}
	// Converts the value to Gi and rounds up the value to the nearest Gi
	return aws.Int64(int64(math.Ceil(quantity.AsApproximateFloat64() / math.Pow(2, 30))))
}

// hydrateCache queries for existing Launch Templates created by Karpenter for the current cluster and adds to the LT cache.
// Any error during hydration will result in a panic
func (p *Provider) hydrateCache(ctx context.Context) {
	clusterName := options.FromContext(ctx).ClusterName
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("tag-key", karpenterManagedTagKey, "tag-value", clusterName))
	if err := p.ec2api.DescribeLaunchTemplatesPagesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []*ec2.Filter{{Name: aws.String(fmt.Sprintf("tag:%s", karpenterManagedTagKey)), Values: []*string{aws.String(clusterName)}}},
	}, func(output *ec2.DescribeLaunchTemplatesOutput, _ bool) bool {
		for _, lt := range output.LaunchTemplates {
			p.cache.SetDefault(*lt.LaunchTemplateName, lt)
		}
		return true
	}); err != nil {
		logging.FromContext(ctx).Errorf(fmt.Sprintf("Unable to hydrate the AWS launch template cache, %s", err))
	} else {
		logging.FromContext(ctx).With("count", p.cache.ItemCount()).Debugf("hydrated launch template cache")
	}
}

func (p *Provider) cachedEvictedFunc(ctx context.Context) func(string, interface{}) {
	return func(key string, lt interface{}) {
		p.Lock()
		defer p.Unlock()
		if _, expiration, _ := p.cache.GetWithExpiration(key); expiration.After(time.Now()) {
			return
		}
		launchTemplate := lt.(*ec2.LaunchTemplate)
		if _, err := p.ec2api.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{LaunchTemplateId: launchTemplate.LaunchTemplateId}); err != nil {
			logging.FromContext(ctx).With("launch-template", launchTemplate.LaunchTemplateName).Errorf("failed to delete launch template, %v", err)
			return
		}
		logging.FromContext(ctx).With(
			"id", aws.StringValue(launchTemplate.LaunchTemplateId),
			"name", aws.StringValue(launchTemplate.LaunchTemplateName),
		).Debugf("deleted launch template")
	}
}

func (p *Provider) getInstanceProfile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (string, error) {
	if nodeClass.Spec.InstanceProfile != nil {
		return aws.StringValue(nodeClass.Spec.InstanceProfile), nil
	}
	if nodeClass.Spec.Role != "" {
		if nodeClass.Status.InstanceProfile == "" {
			return "", cloudprovider.NewNodeClassNotReadyError(fmt.Errorf("instance profile hasn't resolved for role"))
		}
		return nodeClass.Status.InstanceProfile, nil
	}
	defaultProfile := settings.FromContext(ctx).DefaultInstanceProfile
	if defaultProfile == "" {
		return "", errors.New("neither spec.provider.instanceProfile nor --aws-default-instance-profile is specified")
	}
	return defaultProfile, nil
}
