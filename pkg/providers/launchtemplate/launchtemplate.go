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
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/multierr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	"github.com/aws/karpenter-provider-aws/pkg/aws/sdk"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type Provider interface {
	EnsureAll(context.Context, *v1.EC2NodeClass, *karpv1.NodeClaim,
		[]*cloudprovider.InstanceType, string, map[string]string) ([]*LaunchTemplate, error)
	DeleteAll(context.Context, *v1.EC2NodeClass) error
	InvalidateCache(context.Context, string, string)
	ResolveClusterCIDR(context.Context) error
}

type LaunchTemplate struct {
	Name          string
	InstanceTypes []*cloudprovider.InstanceType
	ImageID       string
}

type DefaultProvider struct {
	sync.Mutex
	ec2api                sdk.EC2API
	eksapi                sdk.EKSAPI
	eksClient             eks.Client
	amiFamily             *amifamily.Resolver
	securityGroupProvider securitygroup.Provider
	subnetProvider        subnet.Provider
	cache                 *cache.Cache
	cm                    *pretty.ChangeMonitor
	KubeDNSIP             net.IP
	CABundle              *string
	ClusterEndpoint       string
	ClusterCIDR           atomic.Pointer[string]
}

func NewDefaultProvider(ctx context.Context, cache *cache.Cache, ec2api sdk.EC2API, eksapi sdk.EKSAPI, amiFamily *amifamily.Resolver,
	securityGroupProvider securitygroup.Provider, subnetProvider subnet.Provider,
	caBundle *string, startAsync <-chan struct{}, kubeDNSIP net.IP, clusterEndpoint string) *DefaultProvider {
	l := &DefaultProvider{
		ec2api:                ec2api,
		eksapi:                eksapi,
		amiFamily:             amiFamily,
		securityGroupProvider: securityGroupProvider,
		subnetProvider:        subnetProvider,
		cache:                 cache,
		CABundle:              caBundle,
		cm:                    pretty.NewChangeMonitor(),
		KubeDNSIP:             kubeDNSIP,
		ClusterEndpoint:       clusterEndpoint,
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

func (p *DefaultProvider) EnsureAll(ctx context.Context, nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim,
	instanceTypes []*cloudprovider.InstanceType, capacityType string, tags map[string]string) ([]*LaunchTemplate, error) {

	p.Lock()
	defer p.Unlock()

	options, err := p.createAMIOptions(ctx, nodeClass, lo.Assign(nodeClaim.Labels, map[string]string{karpv1.CapacityTypeLabelKey: capacityType}), tags)
	if err != nil {
		return nil, err
	}
	resolvedLaunchTemplates, err := p.amiFamily.Resolve(nodeClass, nodeClaim, instanceTypes, capacityType, options)
	if err != nil {
		return nil, err
	}
	var launchTemplates []*LaunchTemplate
	for _, resolvedLaunchTemplate := range resolvedLaunchTemplates {
		// Ensure the launch template exists, or create it
		ec2LaunchTemplate, err := p.ensureLaunchTemplate(ctx, resolvedLaunchTemplate)
		if err != nil {
			return nil, err
		}
		launchTemplates = append(launchTemplates, &LaunchTemplate{Name: *ec2LaunchTemplate.LaunchTemplateName, InstanceTypes: resolvedLaunchTemplate.InstanceTypes, ImageID: resolvedLaunchTemplate.AMIID})
	}
	return launchTemplates, nil
}

// InvalidateCache deletes a launch template from cache if it exists
func (p *DefaultProvider) InvalidateCache(ctx context.Context, ltName string, ltID string) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("launch-template-name", ltName, "launch-template-id", ltID))
	p.Lock()
	defer p.Unlock()
	defer p.cache.OnEvicted(p.cachedEvictedFunc(ctx))
	p.cache.OnEvicted(nil)
	log.FromContext(ctx).V(1).Info("invalidating launch template in the cache because it no longer exists")
	p.cache.Delete(ltName)
}

func LaunchTemplateName(options *amifamily.LaunchTemplate) string {
	return fmt.Sprintf("%s/%d", apis.Group, lo.Must(hashstructure.Hash(options, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})))
}

func (p *DefaultProvider) createAMIOptions(ctx context.Context, nodeClass *v1.EC2NodeClass, labels, tags map[string]string) (*amifamily.Options, error) {
	// Remove any labels passed into userData that are prefixed with "node-restriction.kubernetes.io" or "kops.k8s.io" since the kubelet can't
	// register the node with any labels from this domain: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction
	for k := range labels {
		labelDomain := karpv1.GetLabelDomain(k)
		if strings.HasSuffix(labelDomain, corev1.LabelNamespaceNodeRestriction) || strings.HasSuffix(labelDomain, "kops.k8s.io") {
			delete(labels, k)
		}
	}
	// Relying on the status rather than an API call means that Karpenter is subject to a race
	// condition where EC2NodeClass spec changes haven't propagated to the status once a node
	// has launched.
	// If a user changes their EC2NodeClass and shortly after Karpenter launches a node,
	// in the worst case, the node could be drifted and re-created.
	// TODO @aengeda: add status generation fields to gate node creation until the status is updated from a spec change
	// Get constrained security groups
	if len(nodeClass.Status.SecurityGroups) == 0 {
		return nil, fmt.Errorf("no security groups are present in the status")
	}
	return &amifamily.Options{
		ClusterName:              options.FromContext(ctx).ClusterName,
		ClusterEndpoint:          p.ClusterEndpoint,
		ClusterCIDR:              p.ClusterCIDR.Load(),
		InstanceProfile:          nodeClass.Status.InstanceProfile,
		InstanceStorePolicy:      nodeClass.Spec.InstanceStorePolicy,
		SecurityGroups:           nodeClass.Status.SecurityGroups,
		Tags:                     tags,
		Labels:                   labels,
		CABundle:                 p.CABundle,
		KubeDNSIP:                p.KubeDNSIP,
		AssociatePublicIPAddress: nodeClass.Spec.AssociatePublicIPAddress,
		NodeClassName:            nodeClass.Name,
	}, nil
}

func (p *DefaultProvider) ensureLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (*ec2types.LaunchTemplate, error) {
	var launchTemplate *ec2types.LaunchTemplate
	name := LaunchTemplateName(options)
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("launch-template-name", name))
	// Read from cache
	if launchTemplate, ok := p.cache.Get(name); ok {
		p.cache.SetDefault(name, launchTemplate)
		return launchTemplate.(*ec2types.LaunchTemplate), nil
	}
	// Attempt to find an existing LT.
	output, err := p.ec2api.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []string{*aws.String(name)},
	})
	// Create LT if one doesn't exist
	if awserrors.IsNotFound(err) {
		launchTemplate, err = p.createLaunchTemplate(ctx, options)
		if err != nil {
			return nil, fmt.Errorf("creating launch template, %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("describing launch templates, %w", err)
	} else if len(output.LaunchTemplates) != 1 {
		return nil, fmt.Errorf("expected to find one launch template, but found %d", len(output.LaunchTemplates))
	} else {
		if p.cm.HasChanged("launchtemplate-"+name, name) {
			log.FromContext(ctx).V(1).Info("discovered launch template")
		}
		launchTemplate = &output.LaunchTemplates[0]
	}
	p.cache.SetDefault(name, launchTemplate)
	return launchTemplate, nil
}

func convertTagsToSlice(tags []*ec2types.Tag) []ec2types.Tag {
	result := make([]ec2types.Tag, len(tags))
	for i, tag := range tags {
		result[i] = *tag
	}
	return result
}

func convertBlockDeviceMappings(mappings []*ec2types.LaunchTemplateBlockDeviceMapping) []ec2types.LaunchTemplateBlockDeviceMappingRequest {
	result := make([]ec2types.LaunchTemplateBlockDeviceMappingRequest, len(mappings))
	for i, mapping := range mappings {
		result[i] = ec2types.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName: mapping.DeviceName,
			// Assign other fields from the mapping
		}
	}
	return result
}

func convertStringPointerSliceToStringSlice(pointers []*string) []string {
	result := make([]string, len(pointers))
	for i, p := range pointers {
		if p != nil {
			result[i] = *p
		}
	}
	return result
}

func getMetadataEndpointState(value *string) ec2types.LaunchTemplateInstanceMetadataEndpointState {
	if value != nil {
		return ec2types.LaunchTemplateInstanceMetadataEndpointState(*value)
	}
	return ec2types.LaunchTemplateInstanceMetadataEndpointStateDisabled
}

func getMetadataProtocolIPv6State(protocolIPv6 *string) ec2types.LaunchTemplateInstanceMetadataProtocolIpv6 {
	if protocolIPv6 != nil {
		switch *protocolIPv6 {
		case "disabled":
			return ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Disabled
		case "enabled":
			return ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Enabled
		}
	}
	return ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Disabled
}

func getMetadataTokensState(tokens *string) ec2types.LaunchTemplateHttpTokensState {
	if tokens != nil {
		switch *tokens {
		case "required":
			return ec2types.LaunchTemplateHttpTokensStateRequired
		case "optional":
			return ec2types.LaunchTemplateHttpTokensStateOptional
		}
	}
	return ec2types.LaunchTemplateHttpTokensStateOptional
}

func (p *DefaultProvider) createLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (*ec2types.LaunchTemplate, error) {
	userData, err := options.UserData.Script()
	if err != nil {
		return nil, err
	}
	launchTemplateDataTags := []*ec2types.LaunchTemplateTagSpecification{
		{
			ResourceType: ec2types.ResourceTypeNetworkInterface,
			Tags:         convertTagsToSlice(utils.MergeTags(options.Tags)),
		},
	}

	launchTemplateTagSpecificationRequests := make([]ec2types.LaunchTemplateTagSpecificationRequest, len(launchTemplateDataTags))
	for i, tag := range launchTemplateDataTags {
		launchTemplateTagSpecificationRequests[i] = ec2types.LaunchTemplateTagSpecificationRequest{
			ResourceType: tag.ResourceType,
			Tags:         tag.Tags,
		}
	}

	// Add the spot-instances-request tag if trying to launch spot capacity
	if options.CapacityType == karpv1.CapacityTypeSpot {
		tags := utils.MergeTags(options.Tags)
		launchTemplateDataTags = append(launchTemplateDataTags, &ec2types.LaunchTemplateTagSpecification{
			ResourceType: "spot-instances",
			Tags:         make([]ec2types.Tag, len(tags)),
		})
		for i, tag := range tags {
			launchTemplateDataTags[len(launchTemplateDataTags)-1].Tags[i] = *tag
		}
	}
	networkInterfaces := p.generateNetworkInterfaces(options)
	networkInterfaceRequests := make([]ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest, len(networkInterfaces))
	for i, intf := range networkInterfaces {
		networkInterfaceRequests[i] = ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
			DeviceIndex:              intf.DeviceIndex,
			InterfaceType:            intf.InterfaceType,
			Groups:                   intf.Groups,
			NetworkCardIndex:         intf.NetworkCardIndex,
			AssociatePublicIpAddress: intf.AssociatePublicIpAddress,
		}
	}

	output, err := p.ec2api.CreateLaunchTemplate(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(LaunchTemplateName(options)),
		LaunchTemplateData: &ec2types.RequestLaunchTemplateData{
			BlockDeviceMappings: convertBlockDeviceMappings(p.blockDeviceMappings(options.BlockDeviceMappings)),
			IamInstanceProfile: &ec2types.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(options.InstanceProfile),
			},
			Monitoring: &ec2types.LaunchTemplatesMonitoringRequest{
				Enabled: aws.Bool(options.DetailedMonitoring),
			},
			// If the network interface is defined, the security groups are defined within it
			SecurityGroupIds: lo.Ternary(networkInterfaces != nil, nil, convertStringPointerSliceToStringSlice(lo.Map(options.SecurityGroups, func(s v1.SecurityGroup, _ int) *string { return aws.String(s.ID) }))),
			UserData:         aws.String(userData),
			ImageId:          aws.String(options.AMIID),
			MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptionsRequest{
				HttpEndpoint:            getMetadataEndpointState(options.MetadataOptions.HTTPEndpoint),
				HttpProtocolIpv6:        getMetadataProtocolIPv6State(options.MetadataOptions.HTTPProtocolIPv6),
				HttpPutResponseHopLimit: aws.Int32(int32(*options.MetadataOptions.HTTPPutResponseHopLimit)),
				HttpTokens:              getMetadataTokensState(options.MetadataOptions.HTTPTokens),
			},
			NetworkInterfaces: networkInterfaceRequests,
			TagSpecifications: launchTemplateTagSpecificationRequests,
		},
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeLaunchTemplate,
				Tags:         convertTagsToSlice(utils.MergeTags(options.Tags, map[string]string{v1.TagManagedLaunchTemplate: options.ClusterName, v1.LabelNodeClass: options.NodeClassName})),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	log.FromContext(ctx).WithValues("id", aws.ToString(output.LaunchTemplate.LaunchTemplateId)).V(1).Info("created launch template")
	return output.LaunchTemplate, nil
}

// generateNetworkInterfaces generates network interfaces for the launch template.
func (p *DefaultProvider) generateNetworkInterfaces(options *amifamily.LaunchTemplate) []*ec2types.LaunchTemplateInstanceNetworkInterfaceSpecification {
	if options.EFACount != 0 {
		return lo.Times(options.EFACount, func(i int) *ec2types.LaunchTemplateInstanceNetworkInterfaceSpecification {
			groups := make([]string, len(options.SecurityGroups))
			for j, sg := range options.SecurityGroups {
				groups[j] = sg.ID
			}
			return &ec2types.LaunchTemplateInstanceNetworkInterfaceSpecification{
				NetworkCardIndex:         lo.ToPtr(int32(i)),
				DeviceIndex:              lo.ToPtr(lo.Ternary[int32](i == 0, 0, 1)),
				InterfaceType:            lo.ToPtr(string(ec2types.NetworkInterfaceTypeEfa)),
				Groups:                   groups,
				AssociatePublicIpAddress: options.AssociatePublicIPAddress,
			}
		})
	}

	if options.AssociatePublicIPAddress != nil {
		return []*ec2types.LaunchTemplateInstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: options.AssociatePublicIPAddress,
				DeviceIndex:              aws.Int32(0),
				Groups: lo.Map(options.SecurityGroups, func(s v1.SecurityGroup, _ int) string {
					return s.ID
				}),
			},
		}
	}
	return nil
}

func (p *DefaultProvider) blockDeviceMappings(blockDeviceMappings []*v1.BlockDeviceMapping) []*ec2types.LaunchTemplateBlockDeviceMapping {
	if len(blockDeviceMappings) == 0 {
		// The EC2 API fails with empty slices and expects nil.
		return nil
	}
	var blockDeviceMappingsRequest []*ec2types.LaunchTemplateBlockDeviceMapping
	for _, blockDeviceMapping := range blockDeviceMappings {
		blockDeviceMappingsRequest = append(blockDeviceMappingsRequest, &ec2types.LaunchTemplateBlockDeviceMapping{
			DeviceName: blockDeviceMapping.DeviceName,
			Ebs: &ec2types.LaunchTemplateEbsBlockDevice{
				DeleteOnTermination: blockDeviceMapping.EBS.DeleteOnTermination,
				Encrypted:           blockDeviceMapping.EBS.Encrypted,
				VolumeType:          ec2types.VolumeType(aws.ToString(blockDeviceMapping.EBS.VolumeType)),
				Iops:                aws.Int32(int32(*blockDeviceMapping.EBS.IOPS)),
				Throughput:          aws.Int32(int32(*blockDeviceMapping.EBS.Throughput)),
				KmsKeyId:            blockDeviceMapping.EBS.KMSKeyID,
				SnapshotId:          blockDeviceMapping.EBS.SnapshotID,
				VolumeSize:          aws.Int32(int32(*p.volumeSize(blockDeviceMapping.EBS.VolumeSize))),
			},
		})
	}
	return blockDeviceMappingsRequest
}

// volumeSize returns a GiB scaled value from a resource quantity or nil if the resource quantity passed in is nil
func (p *DefaultProvider) volumeSize(quantity *resource.Quantity) *int64 {
	if quantity == nil {
		return nil
	}
	// Converts the value to Gi and rounds up the value to the nearest Gi
	return aws.Int64(int64(math.Ceil(quantity.AsApproximateFloat64() / math.Pow(2, 30))))
}

// hydrateCache queries for existing Launch Templates created by Karpenter for the current cluster and adds to the LT cache.
// Any error during hydration will result in a panic
func (p *DefaultProvider) hydrateCache(ctx context.Context) {
	clusterName := options.FromContext(ctx).ClusterName
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("tag-key", v1.TagManagedLaunchTemplate, "tag-value", clusterName))
	if err := p.ec2api.DescribeLaunchTemplatesPages(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []ec2types.Filter{{Name: aws.String(fmt.Sprintf("tag:%s", v1.TagManagedLaunchTemplate)), Values: []string{*aws.String(clusterName)}}},
	}, func(output *ec2.DescribeLaunchTemplatesOutput, _ bool) bool {
		for _, lt := range output.LaunchTemplates {
			p.cache.SetDefault(*lt.LaunchTemplateName, lt)
		}
		return true
	}); err != nil {
		log.FromContext(ctx).Error(err, "unable to hydrate the AWS launch template cache")
	} else {
		log.FromContext(ctx).WithValues("count", p.cache.ItemCount()).V(1).Info("hydrated launch template cache")
	}
}

func (p *DefaultProvider) cachedEvictedFunc(ctx context.Context) func(string, interface{}) {
	return func(key string, lt interface{}) {
		p.Lock()
		defer p.Unlock()
		if _, expiration, _ := p.cache.GetWithExpiration(key); expiration.After(time.Now()) {
			return
		}
		launchTemplate := lt.(*ec2types.LaunchTemplate)
		if _, err := p.ec2api.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{LaunchTemplateId: launchTemplate.LaunchTemplateId}); awserrors.IgnoreNotFound(err) != nil {
			log.FromContext(ctx).WithValues("launch-template", launchTemplate.LaunchTemplateName).Error(err, "failed to delete launch template")
			return
		}
		log.FromContext(ctx).WithValues(
			"id", aws.ToString(launchTemplate.LaunchTemplateId),
			"name", aws.ToString(launchTemplate.LaunchTemplateName),
		).V(1).Info("deleted launch template")
	}
}

func (p *DefaultProvider) DeleteAll(ctx context.Context, nodeClass *v1.EC2NodeClass) error {
	clusterName := options.FromContext(ctx).ClusterName
	var ltNames []*string
	if err := p.ec2api.DescribeLaunchTemplatesPages(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String(fmt.Sprintf("tag:%s", v1.TagManagedLaunchTemplate)), Values: []string{*aws.String(clusterName)}},
			{Name: aws.String(fmt.Sprintf("tag:%s", v1.LabelNodeClass)), Values: []string{*aws.String(nodeClass.Name)}},
		},
	}, func(output *ec2.DescribeLaunchTemplatesOutput, _ bool) bool {
		for _, lt := range output.LaunchTemplates {
			ltNames = append(ltNames, lt.LaunchTemplateName)
		}
		return true
	}); err != nil {
		return fmt.Errorf("fetching launch templates, %w", err)
	}

	var deleteErr error
	for _, name := range ltNames {
		_, err := p.ec2api.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{LaunchTemplateName: name})
		deleteErr = multierr.Append(deleteErr, err)
	}
	if len(ltNames) > 0 {
		log.FromContext(ctx).WithValues("launchTemplates", utils.PrettySlice(ltNames, 5)).V(1).Info("deleted launch templates")
	}
	if deleteErr != nil {
		return fmt.Errorf("deleting launch templates, %w", deleteErr)
	}
	return nil
}

func (p *DefaultProvider) ResolveClusterCIDR(ctx context.Context) error {
	if p.ClusterCIDR.Load() != nil {
		return nil
	}
	out, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(options.FromContext(ctx).ClusterName),
	})
	if err != nil {
		return err
	}
	if ipv4CIDR := out.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr; ipv4CIDR != nil {
		p.ClusterCIDR.Store(ipv4CIDR)
		log.FromContext(ctx).WithValues("cluster-cidr", *ipv4CIDR).V(1).Info("discovered cluster CIDR")
		return nil
	}
	if ipv6CIDR := out.Cluster.KubernetesNetworkConfig.ServiceIpv6Cidr; ipv6CIDR != nil {
		p.ClusterCIDR.Store(ipv6CIDR)
		log.FromContext(ctx).WithValues("cluster-cidr", *ipv6CIDR).V(1).Info("discovered cluster CIDR")
		return nil
	}
	return fmt.Errorf("no CIDR found in DescribeCluster response")
}
