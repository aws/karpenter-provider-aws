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

package cloudprovider

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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	corev1alpha1 "github.com/aws/karpenter-core/pkg/apis/v1alpha1"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/amifamily"
	awscontext "github.com/aws/karpenter/pkg/context"
	awserrors "github.com/aws/karpenter/pkg/errors"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

const (
	launchTemplateNameFormat  = "Karpenter-%s-%s"
	karpenterManagedTagKey    = "karpenter.k8s.aws/cluster"
	kubernetesVersionCacheKey = "kubernetesVersion"
)

type LaunchTemplateProvider struct {
	sync.Mutex
	ec2api                ec2iface.EC2API
	kubernetesInterface   kubernetes.Interface
	amiFamily             *amifamily.Resolver
	securityGroupProvider *SecurityGroupProvider
	cache                 *cache.Cache
	caBundle              *string
	cm                    *pretty.ChangeMonitor
	kubeDNSIP             net.IP
}

func NewLaunchTemplateProvider(ctx context.Context, ec2api ec2iface.EC2API, kubernetesInterface kubernetes.Interface, amiFamily *amifamily.Resolver, securityGroupProvider *SecurityGroupProvider, caBundle *string, startAsync <-chan struct{}, kubeDNSIP net.IP) *LaunchTemplateProvider {
	l := &LaunchTemplateProvider{
		ec2api:                ec2api,
		kubernetesInterface:   kubernetesInterface,
		amiFamily:             amiFamily,
		securityGroupProvider: securityGroupProvider,
		cache:                 cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval),
		caBundle:              caBundle,
		cm:                    pretty.NewChangeMonitor(),
		kubeDNSIP:             kubeDNSIP,
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

func launchTemplateName(options *amifamily.LaunchTemplate) string {
	hash, err := hashstructure.Hash(options, hashstructure.FormatV2, nil)
	if err != nil {
		panic(fmt.Sprintf("hashing launch template, %s", err))
	}
	return fmt.Sprintf(launchTemplateNameFormat, options.ClusterName, fmt.Sprint(hash))
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, machine *corev1alpha1.Machine,
	instanceTypes []*cloudprovider.InstanceType, additionalLabels map[string]string) (map[string][]*cloudprovider.InstanceType, error) {

	p.Lock()
	defer p.Unlock()
	// If Launch Template is directly specified then just use it
	if nodeTemplate.Spec.LaunchTemplateName != nil {
		return map[string][]*cloudprovider.InstanceType{ptr.StringValue(nodeTemplate.Spec.LaunchTemplateName): instanceTypes}, nil
	}
	instanceProfile, err := p.getInstanceProfile(ctx, nodeTemplate)
	if err != nil {
		return nil, err
	}
	// Get constrained security groups
	securityGroupsIDs, err := p.securityGroupProvider.Get(ctx, nodeTemplate)
	if err != nil {
		return nil, err
	}
	kubeServerVersion, err := p.kubeServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	resolvedLaunchTemplates, err := p.amiFamily.Resolve(ctx, nodeTemplate, machine, instanceTypes, &amifamily.Options{
		ClusterName:             awssettings.FromContext(ctx).ClusterName,
		ClusterEndpoint:         awssettings.FromContext(ctx).ClusterEndpoint,
		AWSENILimitedPodDensity: awssettings.FromContext(ctx).EnableENILimitedPodDensity,
		InstanceProfile:         instanceProfile,
		SecurityGroupsIDs:       securityGroupsIDs,
		Tags:                    lo.Assign(awssettings.FromContext(ctx).Tags, nodeTemplate.Spec.Tags),
		Labels:                  lo.Assign(machine.Labels, additionalLabels),
		CABundle:                p.caBundle,
		KubernetesVersion:       kubeServerVersion,
		KubeDNSIP:               p.kubeDNSIP,
	})
	if err != nil {
		return nil, err
	}
	launchTemplates := map[string][]*cloudprovider.InstanceType{}
	for _, resolvedLaunchTemplate := range resolvedLaunchTemplates {
		// Ensure the launch template exists, or create it
		ec2LaunchTemplate, err := p.ensureLaunchTemplate(ctx, resolvedLaunchTemplate)
		if err != nil {
			return nil, err
		}
		launchTemplates[*ec2LaunchTemplate.LaunchTemplateName] = resolvedLaunchTemplate.InstanceTypes
	}
	return launchTemplates, nil
}

func (p *LaunchTemplateProvider) ensureLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (*ec2.LaunchTemplate, error) {
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
			logging.FromContext(ctx).Debugf("discovered launch template")
		}
		launchTemplate = output.LaunchTemplates[0]
	}
	p.cache.SetDefault(name, launchTemplate)
	return launchTemplate, nil
}

func (p *LaunchTemplateProvider) createLaunchTemplate(ctx context.Context, options *amifamily.LaunchTemplate) (*ec2.LaunchTemplate, error) {
	userData, err := options.UserData.Script()
	if err != nil {
		return nil, err
	}
	output, err := p.ec2api.CreateLaunchTemplateWithContext(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(launchTemplateName(options)),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			BlockDeviceMappings: p.blockDeviceMappings(options.BlockDeviceMappings),
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(options.InstanceProfile),
			},
			SecurityGroupIds: aws.StringSlice(options.SecurityGroupsIDs),
			UserData:         aws.String(userData),
			ImageId:          aws.String(options.AMIID),
			MetadataOptions: &ec2.LaunchTemplateInstanceMetadataOptionsRequest{
				HttpEndpoint:            options.MetadataOptions.HTTPEndpoint,
				HttpProtocolIpv6:        options.MetadataOptions.HTTPProtocolIPv6,
				HttpPutResponseHopLimit: options.MetadataOptions.HTTPPutResponseHopLimit,
				HttpTokens:              options.MetadataOptions.HTTPTokens,
			},
			TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{
				{ResourceType: aws.String(ec2.ResourceTypeNetworkInterface), Tags: v1alpha1.MergeTags(ctx, options.Tags)},
			},
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeLaunchTemplate),
				Tags:         v1alpha1.MergeTags(ctx, options.Tags, map[string]string{karpenterManagedTagKey: options.ClusterName}),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).With("launch-template-id", aws.StringValue(output.LaunchTemplate.LaunchTemplateId)).Debugf("created launch template")
	return output.LaunchTemplate, nil
}

func (p *LaunchTemplateProvider) blockDeviceMappings(blockDeviceMappings []*v1alpha1.BlockDeviceMapping) []*ec2.LaunchTemplateBlockDeviceMappingRequest {
	if len(blockDeviceMappings) == 0 {
		// The EC2 API fails with empty slices and expects nil.
		return nil
	}
	blockDeviceMappingsRequest := []*ec2.LaunchTemplateBlockDeviceMappingRequest{}
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
func (p *LaunchTemplateProvider) volumeSize(quantity *resource.Quantity) *int64 {
	if quantity == nil {
		return nil
	}
	return aws.Int64(int64(quantity.AsApproximateFloat64() / math.Pow(2, 30)))
}

// Invalidate deletes a launch template from cache if it exists
func (p *LaunchTemplateProvider) Invalidate(ctx context.Context, ltName string, ltID string) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("launch-template-name", ltName, "launch-template-id", ltID))
	p.Lock()
	defer p.Unlock()
	defer p.cache.OnEvicted(p.cachedEvictedFunc(ctx))
	p.cache.OnEvicted(nil)
	logging.FromContext(ctx).Debugf("invalidating launch template in the cache because it no longer exists")
	p.cache.Delete(ltName)
}

// hydrateCache queries for existing Launch Templates created by Karpenter for the current cluster and adds to the LT cache.
// Any error during hydration will result in a panic
func (p *LaunchTemplateProvider) hydrateCache(ctx context.Context) {
	clusterName := awssettings.FromContext(ctx).ClusterName
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("tag-key", karpenterManagedTagKey, "tag-value", clusterName))
	logging.FromContext(ctx).Debugf("hydrating the launch template cache")
	if err := p.ec2api.DescribeLaunchTemplatesPagesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []*ec2.Filter{{Name: aws.String(fmt.Sprintf("tag:%s", karpenterManagedTagKey)), Values: []*string{aws.String(clusterName)}}},
	}, func(output *ec2.DescribeLaunchTemplatesOutput, _ bool) bool {
		for _, lt := range output.LaunchTemplates {
			p.cache.SetDefault(*lt.LaunchTemplateName, lt)
		}
		return true
	}); err != nil {
		logging.FromContext(ctx).Errorf(fmt.Sprintf("Unable to hydrate the AWS launch template cache, %s", err))
	}
	logging.FromContext(ctx).With("item-count", p.cache.ItemCount()).Debugf("finished hydrating the launch template cache")
}

func (p *LaunchTemplateProvider) cachedEvictedFunc(ctx context.Context) func(string, interface{}) {
	return func(key string, lt interface{}) {
		if key == kubernetesVersionCacheKey {
			return
		}
		p.Lock()
		defer p.Unlock()
		if _, expiration, _ := p.cache.GetWithExpiration(key); expiration.After(time.Now()) {
			return
		}
		launchTemplate := lt.(*ec2.LaunchTemplate)
		if _, err := p.ec2api.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{LaunchTemplateId: launchTemplate.LaunchTemplateId}); err != nil {
			logging.FromContext(ctx).Errorf("Unable to delete launch template, %v", err)
			return
		}
		logging.FromContext(ctx).Debugf("deleted launch template")
	}
}

func (p *LaunchTemplateProvider) getInstanceProfile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (string, error) {
	if nodeTemplate.Spec.InstanceProfile != nil {
		return aws.StringValue(nodeTemplate.Spec.InstanceProfile), nil
	}
	defaultProfile := awssettings.FromContext(ctx).DefaultInstanceProfile
	if defaultProfile == "" {
		return "", errors.New("neither spec.provider.instanceProfile nor --aws-default-instance-profile is specified")
	}
	return defaultProfile, nil
}

func (p *LaunchTemplateProvider) kubeServerVersion(ctx context.Context) (string, error) {
	if version, ok := p.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	serverVersion, err := p.kubernetesInterface.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	version := fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+"))
	p.cache.SetDefault(kubernetesVersionCacheKey, version)
	if p.cm.HasChanged("kubernete-version", version) {
		logging.FromContext(ctx).With("kubernete-version", version).Debugf("discovered kubernetes version")
	}
	return version, nil
}
