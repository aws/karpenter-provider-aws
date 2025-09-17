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

	"github.com/awslabs/operatorpkg/option"
	"github.com/awslabs/operatorpkg/serrors"
	"go.uber.org/multierr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/karpenter/pkg/scheduling"

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

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type DefaultProviderOpts = option.Function[defaultProviderOpts]

type defaultProviderOpts struct {
	launchModeProvider LaunchModeProvider
}

func WithLaunchModeProvider(provider LaunchModeProvider) DefaultProviderOpts {
	return func(opts *defaultProviderOpts) {
		opts.launchModeProvider = provider
	}
}

type DefaultProvider struct {
	sync.Mutex
	LaunchModeProvider
	ec2api                sdk.EC2API
	eksapi                sdk.EKSAPI
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

func NewDefaultProvider(
	ctx context.Context,
	cache *cache.Cache,
	ec2api sdk.EC2API,
	eksapi sdk.EKSAPI,
	amiFamily amifamily.Resolver,
	securityGroupProvider securitygroup.Provider,
	subnetProvider subnet.Provider,
	caBundle *string,
	startAsync <-chan struct{},
	kubeDNSIP net.IP,
	clusterEndpoint string,
	opts ...DefaultProviderOpts,
) *DefaultProvider {
	resolvedOpts := option.Resolve(opts...)
	if resolvedOpts.launchModeProvider == nil {
		resolvedOpts.launchModeProvider = defaultLaunchModeProvider{}
	}
	l := &DefaultProvider{
		LaunchModeProvider:    resolvedOpts.launchModeProvider,
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
		ClusterIPFamily:       lo.Ternary(kubeDNSIP != nil && kubeDNSIP.To4() == nil, corev1.IPv6Protocol, corev1.IPv4Protocol),
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
func (p *DefaultProvider) EnsureAll(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	instanceTypes []*cloudprovider.InstanceType,
	capacityType string,
	tags map[string]string,
) ([]*LaunchTemplate, error) {
	p.Lock()
	defer p.Unlock()

	opts, err := p.CreateAMIOptions(ctx, nodeClass, lo.Assign(
		nodeClaim.Labels,
		scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...).Labels(), // Inject single-value requirements into userData
		map[string]string{karpv1.CapacityTypeLabelKey: capacityType},
	), tags)
	if err != nil {
		return nil, err
	}
	resolvedLaunchTemplates, err := p.amiFamily.Resolve(nodeClass, nodeClaim, instanceTypes, capacityType, opts)
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
		launchTemplates = append(launchTemplates, &LaunchTemplate{
			Name:                  *ec2LaunchTemplate.LaunchTemplateName,
			InstanceTypes:         resolvedLaunchTemplate.InstanceTypes,
			ImageID:               resolvedLaunchTemplate.AMIID,
			CapacityReservationID: resolvedLaunchTemplate.CapacityReservationID,
		})
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
	return fmt.Sprintf("%s/%d", v1.LaunchTemplateNamePrefix, lo.Must(hashstructure.Hash(options, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})))
}

func (p *DefaultProvider) CreateAMIOptions(ctx context.Context, nodeClass *v1.EC2NodeClass, labels, tags map[string]string) (*amifamily.Options, error) {
	// Remove any labels passed into userData that are prefixed with "node-restriction.kubernetes.io" or "kops.k8s.io" since the kubelet can't
	// register the node with any labels from this domain: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction
	for k := range labels {
		labelDomain := karpv1.GetLabelDomain(k)
		if strings.HasSuffix(labelDomain, corev1.LabelNamespaceNodeRestriction) || strings.HasSuffix(labelDomain, "kops.k8s.io") {
			delete(labels, k)
		}
	}
	labels = InjectDoNotSyncTaintsLabel(nodeClass.AMIFamily(), labels)
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
		Ipv4PrefixCount:          nodeClass.Spec.Ipv4PrefixCount,
		NodeClassName:            nodeClass.Name,
	}, nil
}

func (p *DefaultProvider) ensureLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (ec2types.LaunchTemplate, error) {
	var launchTemplate ec2types.LaunchTemplate
	name := LaunchTemplateName(options)
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("launch-template-name", name))
	// Read from cache
	if launchTemplate, ok := p.cache.Get(name); ok {
		p.cache.SetDefault(name, launchTemplate)
		return launchTemplate.(ec2types.LaunchTemplate), nil
	}
	// Attempt to find an existing LT.
	output, err := p.ec2api.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []string{name},
	})
	// Create LT if one doesn't exist
	if awserrors.IsNotFound(err) {
		launchTemplate, err = p.createLaunchTemplate(ctx, options)
		if err != nil {
			return ec2types.LaunchTemplate{}, fmt.Errorf("creating launch template, %w", err)
		}
	} else if err != nil {
		return ec2types.LaunchTemplate{}, fmt.Errorf("describing launch templates, %w", err)
	} else if len(output.LaunchTemplates) != 1 {
		return ec2types.LaunchTemplate{}, serrors.Wrap(fmt.Errorf("expected to find one launch template"), "launch-template-count", len(output.LaunchTemplates))
	} else {
		if p.cm.HasChanged("launchtemplate-"+name, name) {
			log.FromContext(ctx).V(1).Info("discovered launch template")
		}
		launchTemplate = output.LaunchTemplates[0]
	}
	p.cache.SetDefault(name, launchTemplate)
	return launchTemplate, nil
}

func (p *DefaultProvider) createLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (ec2types.LaunchTemplate, error) {
	userData, err := options.UserData.Script()
	if err != nil {
		return ec2types.LaunchTemplate{}, err
	}
	createLaunchTemplateInput := NewCreateLaunchTemplateInputBuilder(options, p.ClusterIPFamily, userData).WithLaunchModeProvider(p).Build(ctx)
	output, err := p.ec2api.CreateLaunchTemplate(ctx, createLaunchTemplateInput)
	if err != nil {
		return ec2types.LaunchTemplate{}, err
	}
	log.FromContext(ctx).WithValues("id", aws.ToString(output.LaunchTemplate.LaunchTemplateId)).V(1).Info("created launch template")
	return lo.FromPtr(output.LaunchTemplate), nil
}

// generateNetworkInterfaces generates network interfaces for the launch template.
func generateNetworkInterfaces(options *amifamily.LaunchTemplate, clusterIPFamily corev1.IPFamily) []ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest {
	if options.EFACount != 0 {
		return lo.Times(options.EFACount, func(i int) ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest {
			return ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
				//nolint: gosec
				NetworkCardIndex: lo.ToPtr(int32(i)),
				// Some networking magic to ensure that one network card has higher priority than all the others (important if an instance needs a public IP w/o adding an EIP to every network card)
				DeviceIndex:     lo.ToPtr(lo.Ternary[int32](i == 0, 0, 1)),
				InterfaceType:   lo.ToPtr(string(ec2types.NetworkInterfaceTypeEfa)),
				Ipv4PrefixCount: options.Ipv4PrefixCount,
				Groups:          lo.Map(options.SecurityGroups, func(s v1.SecurityGroup, _ int) string { return s.ID }),
				// Instances launched with multiple pre-configured network interfaces cannot set AssociatePublicIPAddress to true. This is an EC2 limitation. However, this does not apply for instances
				// with a single EFA network interface, and we should support those use cases. Launch failures with multiple enis should be considered user misconfiguration.
				AssociatePublicIpAddress: options.AssociatePublicIPAddress,
				PrimaryIpv6:              lo.Ternary(clusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(true), nil),
				Ipv6AddressCount:         lo.Ternary(clusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(int32(1)), nil),
			}
		})
	}

	return []ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
		{
			AssociatePublicIpAddress: options.AssociatePublicIPAddress,
			DeviceIndex:              aws.Int32(0),
			Ipv4PrefixCount:          options.Ipv4PrefixCount,
			Groups: lo.Map(options.SecurityGroups, func(s v1.SecurityGroup, _ int) string {
				return s.ID
			}),
			PrimaryIpv6:      lo.Ternary(clusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(true), nil),
			Ipv6AddressCount: lo.Ternary(clusterIPFamily == corev1.IPv6Protocol, lo.ToPtr(int32(1)), nil),
		},
	}
}

func blockDeviceMappings(blockDeviceMappings []*v1.BlockDeviceMapping) []ec2types.LaunchTemplateBlockDeviceMappingRequest {
	if len(blockDeviceMappings) == 0 {
		// The EC2 API fails with empty slices and expects nil.
		return nil
	}
	var blockDeviceMappingsRequest []ec2types.LaunchTemplateBlockDeviceMappingRequest
	for _, blockDeviceMapping := range blockDeviceMappings {
		blockDeviceMappingsRequest = append(blockDeviceMappingsRequest, ec2types.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName: blockDeviceMapping.DeviceName,
			Ebs: &ec2types.LaunchTemplateEbsBlockDeviceRequest{
				DeleteOnTermination: blockDeviceMapping.EBS.DeleteOnTermination,
				Encrypted:           blockDeviceMapping.EBS.Encrypted,
				VolumeType:          ec2types.VolumeType(aws.ToString(blockDeviceMapping.EBS.VolumeType)),
				//Lints here can be removed when we update options.EBS.IOPS and Throughput type to be int32
				//nolint: gosec
				Iops: lo.EmptyableToPtr(int32(lo.FromPtr(blockDeviceMapping.EBS.IOPS))),
				//nolint: gosec
				Throughput:               lo.EmptyableToPtr(int32(lo.FromPtr(blockDeviceMapping.EBS.Throughput))),
				KmsKeyId:                 blockDeviceMapping.EBS.KMSKeyID,
				SnapshotId:               blockDeviceMapping.EBS.SnapshotID,
				VolumeInitializationRate: blockDeviceMapping.EBS.VolumeInitializationRate,
				VolumeSize:               volumeSize(blockDeviceMapping.EBS.VolumeSize),
			},
		})
	}
	return blockDeviceMappingsRequest
}

// volumeSize returns a GiB scaled value from a resource quantity or nil if the resource quantity passed in is nil
func volumeSize(quantity *resource.Quantity) *int32 {
	if quantity == nil {
		return nil
	}
	// Converts the value to Gi and rounds up the value to the nearest Gi
	return lo.ToPtr(int32(math.Ceil(quantity.AsApproximateFloat64() / math.Pow(2, 30))))
}

// hydrateCache queries for existing Launch Templates created by Karpenter for the current cluster and adds to the LT cache.
// Any error during hydration will result in a panic
func (p *DefaultProvider) hydrateCache(ctx context.Context) {
	clusterName := options.FromContext(ctx).ClusterName
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("tag-key", v1.EKSClusterNameTagKey, "tag-value", clusterName))

	paginator := ec2.NewDescribeLaunchTemplatesPaginator(p.ec2api, &ec2.DescribeLaunchTemplatesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", v1.EKSClusterNameTagKey)),
				Values: []string{clusterName},
			},
		},
		// MaxResults for DescribeLaunchTemplates is capped at 200
		MaxResults: lo.ToPtr[int32](200),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to hydrate the AWS launch template cache")
			return
		}

		for _, lt := range page.LaunchTemplates {
			p.cache.SetDefault(*lt.LaunchTemplateName, lt)
		}
	}

	log.FromContext(ctx).WithValues("count", p.cache.ItemCount()).V(1).Info("hydrated launch template cache")
}

func (p *DefaultProvider) cachedEvictedFunc(ctx context.Context) func(string, interface{}) {
	return func(key string, lt interface{}) {
		p.Lock()
		defer p.Unlock()
		if _, expiration, _ := p.cache.GetWithExpiration(key); expiration.After(time.Now()) {
			return
		}
		launchTemplate := lt.(ec2types.LaunchTemplate)
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

	paginator := ec2.NewDescribeLaunchTemplatesPaginator(p.ec2api, &ec2.DescribeLaunchTemplatesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", v1.EKSClusterNameTagKey)),
				Values: []string{clusterName},
			},
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", v1.NodeClassTagKey)),
				Values: []string{nodeClass.Name},
			},
		},
		// MaxResults for DescribeLaunchTemplates is capped at 200
		MaxResults: lo.ToPtr[int32](200),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("fetching launch templates, %w", err)
		}

		for _, lt := range page.LaunchTemplates {
			ltNames = append(ltNames, lt.LaunchTemplateName)
		}
	}

	var deleteErr error
	for _, name := range ltNames {
		_, err := p.ec2api.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{LaunchTemplateName: name})
		deleteErr = multierr.Append(deleteErr, err)
	}
	if len(ltNames) > 0 {
		log.FromContext(ctx).WithValues("launchTemplates", utils.PrettySlice(lo.FromSlicePtr(ltNames), 5)).V(1).Info("deleted launch templates")
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
	out, err := p.eksapi.DescribeCluster(ctx, &eks.DescribeClusterInput{
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

// InjectDoNotSyncTaintsLabel adds a label for all non-custom AMI families. It is exported just for ease
// of testing.
// This label is to tell karpenter that it should *not* sync taints. This is to work around a race condition.
// By default, this label is not added to the custom AMI family as users may still want their taints synced. Startup
// taints will be racy for custom AMIs if they do not add this label, however.
// https://github.com/kubernetes-sigs/karpenter/issues/1772
func InjectDoNotSyncTaintsLabel(amiFamilyName string, labels map[string]string) map[string]string {
	if amiFamilyName != v1.AMIFamilyCustom {
		return lo.Assign(labels, map[string]string{karpv1.NodeDoNotSyncTaintsLabelKey: "true"})
	}
	return labels
}
