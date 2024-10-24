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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
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
	ec2api                ec2iface.EC2API
	eksapi                eksiface.EKSAPI
	amiFamily             amifamily.Resolver
	securityGroupProvider securitygroup.Provider
	subnetProvider        subnet.Provider
	cache                 *cache.Cache
	cm                    *pretty.ChangeMonitor
	KubeDNSIP             net.IP
	CABundle              *string
	ClusterEndpoint       string
	ClusterCIDR           atomic.Pointer[string]
	ClusterIPFamily       corev1.IPFamily
}

func NewDefaultProvider(ctx context.Context, cache *cache.Cache, ec2api ec2iface.EC2API, eksapi eksiface.EKSAPI, amiFamily amifamily.Resolver,
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
		ClusterIPFamily: convertIPFamily(*lo.Must(eksapi.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
			Name: aws.String(options.FromContext(ctx).ClusterName),
		})).Cluster.KubernetesNetworkConfig.IpFamily),
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

func (p *DefaultProvider) ensureLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (*ec2.LaunchTemplate, error) {
	var launchTemplate *ec2.LaunchTemplate
	name := LaunchTemplateName(options)
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("launch-template-name", name))
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
		launchTemplate = output.LaunchTemplates[0]
	}
	p.cache.SetDefault(name, launchTemplate)
	return launchTemplate, nil
}

func (p *DefaultProvider) createLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (*ec2.LaunchTemplate, error) {
	userData, err := options.UserData.Script()
	if err != nil {
		return nil, err
	}
	launchTemplateDataTags := []*ec2.LaunchTemplateTagSpecificationRequest{
		{ResourceType: aws.String(ec2.ResourceTypeNetworkInterface), Tags: utils.MergeTags(options.Tags)},
	}
	// Add the spot-instances-request tag if trying to launch spot capacity
	if options.CapacityType == karpv1.CapacityTypeSpot {
		launchTemplateDataTags = append(launchTemplateDataTags, &ec2.LaunchTemplateTagSpecificationRequest{ResourceType: aws.String(ec2.ResourceTypeSpotInstancesRequest), Tags: utils.MergeTags(options.Tags)})
	}
	networkInterfaces := p.generateNetworkInterfaces(options)
	output, err := p.ec2api.CreateLaunchTemplateWithContext(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(LaunchTemplateName(options)),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			BlockDeviceMappings: p.blockDeviceMappings(options.BlockDeviceMappings),
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(options.InstanceProfile),
			},
			Monitoring: &ec2.LaunchTemplatesMonitoringRequest{
				Enabled: aws.Bool(options.DetailedMonitoring),
			},
			// If the network interface is defined, the security groups are defined within it
			SecurityGroupIds: lo.Ternary(networkInterfaces != nil, nil, lo.Map(options.SecurityGroups, func(s v1.SecurityGroup, _ int) *string { return aws.String(s.ID) })),
			UserData:         aws.String(userData),
			ImageId:          aws.String(options.AMIID),
			MetadataOptions: &ec2.LaunchTemplateInstanceMetadataOptionsRequest{
				HttpEndpoint:            options.MetadataOptions.HTTPEndpoint,
				HttpProtocolIpv6:        options.MetadataOptions.HTTPProtocolIPv6,
				HttpPutResponseHopLimit: options.MetadataOptions.HTTPPutResponseHopLimit,
				HttpTokens:              options.MetadataOptions.HTTPTokens,
				// We statically set the InstanceMetadataTags to "disabled" for all new instances since
				// account-wide defaults can override instance defaults on metadata settings
				// This can cause instance failure on accounts that default to instance tags since Karpenter
				// can't support instance tags with its current tags (e.g. kubernetes.io/cluster/*, karpenter.k8s.aws/ec2nodeclass)
				// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-options.html#instance-metadata-options-order-of-precedence
				InstanceMetadataTags: lo.ToPtr(ec2.InstanceMetadataTagsStateDisabled),
			},
			NetworkInterfaces: networkInterfaces,
			TagSpecifications: launchTemplateDataTags,
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeLaunchTemplate),
				Tags:         utils.MergeTags(options.Tags, map[string]string{v1.TagManagedLaunchTemplate: options.ClusterName, v1.LabelNodeClass: options.NodeClassName}),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	log.FromContext(ctx).WithValues("id", aws.StringValue(output.LaunchTemplate.LaunchTemplateId)).V(1).Info("created launch template")
	return output.LaunchTemplate, nil
}

// generateNetworkInterfaces generates network interfaces for the launch template.
func (p *DefaultProvider) generateNetworkInterfaces(options *amifamily.LaunchTemplate) []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest {
	if options.EFACount != 0 {
		return lo.Times(options.EFACount, func(i int) *ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest {
			return &ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
				NetworkCardIndex: lo.ToPtr(int64(i)),
				// Some networking magic to ensure that one network card has higher priority than all the others (important if an instance needs a public IP w/o adding an EIP to every network card)
				DeviceIndex:   lo.ToPtr(lo.Ternary[int64](i == 0, 0, 1)),
				InterfaceType: lo.ToPtr(ec2.NetworkInterfaceTypeEfa),
				Groups:        lo.Map(options.SecurityGroups, func(s v1.SecurityGroup, _ int) *string { return aws.String(s.ID) }),
				// Instances launched with multiple pre-configured network interfaces cannot set AssociatePublicIPAddress to true. This is an EC2 limitation. However, this does not apply for instances
				// with a single EFA network interface, and we should support those use cases. Launch failures with multiple enis should be considered user misconfiguration.
				AssociatePublicIpAddress: options.AssociatePublicIPAddress,
				PrimaryIpv6:              lo.Ternary(p.ClusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(true), nil),
				Ipv6PrefixCount:          lo.Ternary(p.ClusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(int64(1)), nil),
			}
		})
	}

	if options.AssociatePublicIPAddress != nil {
		return []*ec2.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
			{
				AssociatePublicIpAddress: options.AssociatePublicIPAddress,
				DeviceIndex:              aws.Int64(0),
				Groups:                   lo.Map(options.SecurityGroups, func(s v1.SecurityGroup, _ int) *string { return aws.String(s.ID) }),
				PrimaryIpv6:              lo.Ternary(p.ClusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(true), nil),
				Ipv6PrefixCount:          lo.Ternary(p.ClusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(int64(1)), nil),
			},
		}
	}
	return nil
}

func (p *DefaultProvider) blockDeviceMappings(blockDeviceMappings []*v1.BlockDeviceMapping) []*ec2.LaunchTemplateBlockDeviceMappingRequest {
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
	if err := p.ec2api.DescribeLaunchTemplatesPagesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []*ec2.Filter{{Name: aws.String(fmt.Sprintf("tag:%s", v1.TagManagedLaunchTemplate)), Values: []*string{aws.String(clusterName)}}},
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
		launchTemplate := lt.(*ec2.LaunchTemplate)
		if _, err := p.ec2api.DeleteLaunchTemplateWithContext(ctx, &ec2.DeleteLaunchTemplateInput{LaunchTemplateId: launchTemplate.LaunchTemplateId}); awserrors.IgnoreNotFound(err) != nil {
			log.FromContext(ctx).WithValues("launch-template", launchTemplate.LaunchTemplateName).Error(err, "failed to delete launch template")
			return
		}
		log.FromContext(ctx).WithValues(
			"id", aws.StringValue(launchTemplate.LaunchTemplateId),
			"name", aws.StringValue(launchTemplate.LaunchTemplateName),
		).V(1).Info("deleted launch template")
	}
}

func (p *DefaultProvider) DeleteAll(ctx context.Context, nodeClass *v1.EC2NodeClass) error {
	clusterName := options.FromContext(ctx).ClusterName
	var ltNames []*string
	if err := p.ec2api.DescribeLaunchTemplatesPagesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []*ec2.Filter{
			{Name: aws.String(fmt.Sprintf("tag:%s", v1.TagManagedLaunchTemplate)), Values: []*string{aws.String(clusterName)}},
			{Name: aws.String(fmt.Sprintf("tag:%s", v1.LabelNodeClass)), Values: []*string{aws.String(nodeClass.Name)}},
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
		_, err := p.ec2api.DeleteLaunchTemplateWithContext(ctx, &ec2.DeleteLaunchTemplateInput{LaunchTemplateName: name})
		deleteErr = multierr.Append(deleteErr, err)
	}
	if len(ltNames) > 0 {
		log.FromContext(ctx).WithValues("launchTemplates", utils.PrettySlice(aws.StringValueSlice(ltNames), 5)).V(1).Info("deleted launch templates")
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
	out, err := p.eksapi.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
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

func convertIPFamily(ipFamily string) corev1.IPFamily {
	if strings.ToLower(ipFamily) == "ipv4" {
		return corev1.IPv4Protocol
	} else if strings.ToLower(ipFamily) == "ipv6" {
		return corev1.IPv6Protocol
	} else {
		return corev1.IPFamilyUnknown
	}
}
