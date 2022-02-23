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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/mitchellh/hashstructure/v2"
	"go.uber.org/zap"
	core "k8s.io/api/core/v1"
	"k8s.io/client-go/transport"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/patrickmn/go-cache"
)

const (
	launchTemplateNameFormat = "Karpenter-%s-%s"
)

type LaunchTemplateProvider struct {
	sync.Mutex
	logger                *zap.SugaredLogger
	ec2api                ec2iface.EC2API
	amiProvider           *AMIProvider
	securityGroupProvider *SecurityGroupProvider
	cache                 *cache.Cache
}

func NewLaunchTemplateProvider(ctx context.Context, ec2api ec2iface.EC2API, amiProvider *AMIProvider, securityGroupProvider *SecurityGroupProvider) *LaunchTemplateProvider {
	l := &LaunchTemplateProvider{
		ec2api:                ec2api,
		logger:                logging.FromContext(ctx).Named("launchtemplate"),
		amiProvider:           amiProvider,
		securityGroupProvider: securityGroupProvider,
		cache:                 cache.New(CacheTTL, CacheCleanupInterval),
	}
	l.cache.OnEvicted(l.onCacheEvicted)
	l.hydrateCache(ctx)
	return l
}

func launchTemplateName(options *launchTemplateOptions) string {
	hash, err := hashstructure.Hash(options, hashstructure.FormatV2, nil)
	if err != nil {
		panic(fmt.Sprintf("hashing launch template, %s", err))
	}
	return fmt.Sprintf(launchTemplateNameFormat, options.ClusterName, fmt.Sprint(hash))
}

// launchTemplateOptions is hashed and results in the creation of a real EC2
// LaunchTemplate. Do not change this struct without thinking through the impact
// to the number of LaunchTemplates that will result from this change.
type launchTemplateOptions struct {
	// Edge-triggered fields that will only change on kube events.
	ClusterName     string
	UserData        string
	InstanceProfile string
	// Level-triggered fields that may change out of sync.
	SecurityGroupsIds []string
	AMIID             string
	Tags              map[string]string
	MetadataOptions   *v1alpha1.MetadataOptions
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType, additionalLabels map[string]string) (map[string][]cloudprovider.InstanceType, error) {
	// If Launch Template is directly specified then just use it
	if constraints.LaunchTemplate != nil {
		return map[string][]cloudprovider.InstanceType{ptr.StringValue(constraints.LaunchTemplate): instanceTypes}, nil
	}
	instanceProfile, err := p.getInstanceProfile(ctx, constraints)
	if err != nil {
		return nil, err
	}
	// Get constrained security groups
	securityGroupsIds, err := p.securityGroupProvider.Get(ctx, constraints)
	if err != nil {
		return nil, err
	}
	// Get constrained AMI ID
	amis, err := p.amiProvider.Get(ctx, constraints, instanceTypes)
	if err != nil {
		return nil, err
	}
	// Construct launch templates
	launchTemplates := map[string][]cloudprovider.InstanceType{}
	caBundle, err := p.GetCABundle(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ca bundle for user data, %w", err)
	}
	for amiID, instanceTypes := range amis {
		// Ensure the launch template exists, or create it
		launchTemplate, err := p.ensureLaunchTemplate(ctx, &launchTemplateOptions{
			UserData:          p.getUserData(ctx, constraints, instanceTypes, additionalLabels, caBundle),
			ClusterName:       injection.GetOptions(ctx).ClusterName,
			InstanceProfile:   instanceProfile,
			AMIID:             amiID,
			SecurityGroupsIds: securityGroupsIds,
			Tags:              constraints.Tags,
			MetadataOptions:   constraints.GetMetadataOptions(),
		})
		if err != nil {
			return nil, err
		}
		launchTemplates[aws.StringValue(launchTemplate.LaunchTemplateName)] = instanceTypes
	}
	return launchTemplates, nil
}

func (p *LaunchTemplateProvider) ensureLaunchTemplate(ctx context.Context, options *launchTemplateOptions) (*ec2.LaunchTemplate, error) {
	// Ensure that multiple threads don't attempt to create the same launch template
	p.Lock()
	defer p.Unlock()

	var launchTemplate *ec2.LaunchTemplate
	name := launchTemplateName(options)
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
	if isNotFound(err) {
		launchTemplate, err = p.createLaunchTemplate(ctx, options)
		if err != nil {
			return nil, fmt.Errorf("creating launch template, %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("describing launch templates, %w", err)
	} else if len(output.LaunchTemplates) != 1 {
		return nil, fmt.Errorf("expected to find one launch template, but found %d", len(output.LaunchTemplates))
	} else {
		logging.FromContext(ctx).Debugf("Discovered launch template %s", name)
		launchTemplate = output.LaunchTemplates[0]
	}
	// 4. Save in cache to reduce API calls
	p.cache.SetDefault(name, launchTemplate)
	return launchTemplate, nil
}

// needsDocker returns true if the instance type is unable to use
// containerd directly
func needsDocker(is []cloudprovider.InstanceType) bool {
	for _, i := range is {
		if !i.AWSNeurons().IsZero() || !i.NvidiaGPUs().IsZero() {
			return true
		}
	}
	return false
}

func (p *LaunchTemplateProvider) createLaunchTemplate(ctx context.Context, options *launchTemplateOptions) (*ec2.LaunchTemplate, error) {
	output, err := p.ec2api.CreateLaunchTemplateWithContext(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(launchTemplateName(options)),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			BlockDeviceMappings: []*ec2.LaunchTemplateBlockDeviceMappingRequest{{
				DeviceName: aws.String("/dev/xvda"),
				Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
					Encrypted:  aws.Bool(true),
					VolumeSize: aws.Int64(20),
				},
			}},
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(options.InstanceProfile),
			},
			SecurityGroupIds: aws.StringSlice(options.SecurityGroupsIds),
			UserData:         aws.String(options.UserData),
			ImageId:          aws.String(options.AMIID),
			MetadataOptions: &ec2.LaunchTemplateInstanceMetadataOptionsRequest{
				HttpEndpoint:            options.MetadataOptions.HTTPEndpoint,
				HttpProtocolIpv6:        options.MetadataOptions.HTTPProtocolIPv6,
				HttpPutResponseHopLimit: options.MetadataOptions.HTTPPutResponseHopLimit,
				HttpTokens:              options.MetadataOptions.HTTPTokens,
			},
		},
		TagSpecifications: []*ec2.TagSpecification{{
			ResourceType: aws.String(ec2.ResourceTypeLaunchTemplate),
			Tags:         v1alpha1.MergeTags(ctx, options.Tags),
		}},
	})
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Debugf("Created launch template, %s", *output.LaunchTemplate.LaunchTemplateName)
	return output.LaunchTemplate, nil
}

// hydrateCache queries for existing Launch Templates created by Karpenter for the current cluster and adds to the LT cache.
// Any error during hydration will result in a panic
func (p *LaunchTemplateProvider) hydrateCache(ctx context.Context) {
	queryKey := fmt.Sprintf(launchTemplateNameFormat, injection.GetOptions(ctx).ClusterName, "*")
	p.logger.Debugf("Hydrating the launch template cache with names matching \"%s\"", queryKey)
	if err := p.ec2api.DescribeLaunchTemplatesPagesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []*ec2.Filter{{Name: aws.String("launch-template-name"), Values: []*string{aws.String(queryKey)}}},
	}, func(output *ec2.DescribeLaunchTemplatesOutput, _ bool) bool {
		for _, lt := range output.LaunchTemplates {
			p.cache.SetDefault(*lt.LaunchTemplateName, lt)
		}
		return true
	}); err != nil {
		panic(fmt.Sprintf("Unable to hydrate the AWS launch template cache, %s", err))
	}
	p.logger.Debugf("Finished hydrating the launch template cache with %d items", p.cache.ItemCount())
}

func (p *LaunchTemplateProvider) onCacheEvicted(key string, lt interface{}) {
	p.Lock()
	defer p.Unlock()
	if _, expiration, _ := p.cache.GetWithExpiration(key); expiration.After(time.Now()) {
		return
	}
	launchTemplate := lt.(*ec2.LaunchTemplate)
	if _, err := p.ec2api.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{LaunchTemplateId: launchTemplate.LaunchTemplateId}); err != nil {
		p.logger.Errorf("Unable to delete launch template, %v", err)
		return
	}
	p.logger.Debugf("Deleted launch template %v", aws.StringValue(launchTemplate.LaunchTemplateId))
}

func sortedTaints(ts []core.Taint) []core.Taint {
	sorted := append(ts[:0:0], ts...) // copy to avoid touching original
	sort.Slice(sorted, func(i, j int) bool {
		ti, tj := sorted[i], sorted[j]
		if ti.Key < tj.Key {
			return true
		}
		if ti.Key == tj.Key && ti.Value < tj.Value {
			return true
		}
		if ti.Value == tj.Value {
			return ti.Effect < tj.Effect
		}
		return false
	})
	return sorted
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func (p *LaunchTemplateProvider) getUserData(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType, additionalLabels map[string]string, caBundle *string) string {
	if aws.StringValue(constraints.AMIFamily) == v1alpha1.AMIFamilyBottlerocket {
		return p.getBottlerocketUserData(ctx, constraints, additionalLabels, caBundle)
	}
	return p.getAL2UserData(ctx, constraints, instanceTypes, additionalLabels, caBundle)
}

func (p *LaunchTemplateProvider) getBottlerocketUserData(ctx context.Context, constraints *v1alpha1.Constraints, additionalLabels map[string]string, caBundle *string) string {
	userData := fmt.Sprintf("[settings.kubernetes]\n\"cluster-name\" = \"%s\"\n\"api-server\" = \"%s\"\n", injection.GetOptions(ctx).ClusterName, injection.GetOptions(ctx).ClusterEndpoint)
	if len(constraints.KubeletConfiguration.ClusterDNS) > 0 {
		userData += fmt.Sprintf("\"cluster-dns-ip\" = \"%s\"\n", constraints.KubeletConfiguration.ClusterDNS[0])
	}
	if caBundle != nil {
		userData += fmt.Sprintf("\"cluster-certificate\" = \"%s\"\n", *caBundle)
	}
	nodeLabelArgs := functional.UnionStringMaps(additionalLabels, constraints.Labels)
	if len(nodeLabelArgs) > 0 {
		userData += "[settings.kubernetes.node-labels]\n"
		for key, val := range nodeLabelArgs {
			userData += fmt.Sprintf("\"%s\" = \"%s\"\n", key, val)
		}
	}
	if len(constraints.Taints) > 0 {
		userData += "[settings.kubernetes.node-taints]\n"
		sorted := sortedTaints(constraints.Taints)
		for _, taint := range sorted {
			userData += fmt.Sprintf("\"%s\"=\"%s:%s\"\n", taint.Key, taint.Value, taint.Effect)
		}
	}
	return base64.StdEncoding.EncodeToString([]byte(userData))
}

// getAL2UserData returns the exact same string for equivalent input,
// even if elements of those inputs are in differing orders,
// guaranteeing it won't cause spurious hash differences.
// AL2 userdata also works on Ubuntu
func (p *LaunchTemplateProvider) getAL2UserData(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType, additionalLabels map[string]string, caBundle *string) string {
	var containerRuntimeArg string
	if !needsDocker(instanceTypes) {
		containerRuntimeArg = "--container-runtime containerd"
	}

	var userData bytes.Buffer
	userData.WriteString(fmt.Sprintf(`#!/bin/bash -xe
exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1
/etc/eks/bootstrap.sh '%s' %s \
    --apiserver-endpoint '%s'`,
		injection.GetOptions(ctx).ClusterName,
		containerRuntimeArg,
		injection.GetOptions(ctx).ClusterEndpoint))
	if caBundle != nil {
		userData.WriteString(fmt.Sprintf(` \
    --b64-cluster-ca '%s'`,
			*caBundle))
	}

	nodeLabelArgs := p.getNodeLabelArgs(functional.UnionStringMaps(additionalLabels, constraints.Labels))
	nodeTaintsArgs := p.getNodeTaintArgs(constraints)
	kubeletExtraArgs := strings.Trim(strings.Join([]string{nodeLabelArgs, nodeTaintsArgs.String()}, " "), " ")

	if !injection.GetOptions(ctx).AWSENILimitedPodDensity {
		userData.WriteString(` \
    --use-max-pods=false`)
		kubeletExtraArgs += " --max-pods=110"
	}

	if len(kubeletExtraArgs) > 0 {
		userData.WriteString(fmt.Sprintf(` \
    --kubelet-extra-args '%s'`, kubeletExtraArgs))
	}
	if len(constraints.KubeletConfiguration.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(` \
    --dns-cluster-ip '%s'`, constraints.KubeletConfiguration.ClusterDNS[0]))
	}
	return base64.StdEncoding.EncodeToString(userData.Bytes())
}

func (p *LaunchTemplateProvider) getNodeLabelArgs(nodeLabels map[string]string) string {
	nodeLabelArgs := ""
	if len(nodeLabels) > 0 {
		labelStrings := []string{}
		// Must be in sorted order or else equivalent options won't
		// hash the same
		for _, k := range sortedKeys(nodeLabels) {
			if v1alpha5.AllowedLabelDomains.Has(k) {
				continue
			}
			labelStrings = append(labelStrings, fmt.Sprintf("%s=%v", k, nodeLabels[k]))
		}
		nodeLabelArgs = fmt.Sprintf("--node-labels=%s", strings.Join(labelStrings, ","))
	}
	return nodeLabelArgs
}

func (p *LaunchTemplateProvider) getNodeTaintArgs(constraints *v1alpha1.Constraints) bytes.Buffer {
	var nodeTaintsArgs bytes.Buffer
	if len(constraints.Taints) > 0 {
		nodeTaintsArgs.WriteString("--register-with-taints=")
		first := true
		// Must be in sorted order or else equivalent options won't
		// hash the same.
		sorted := sortedTaints(constraints.Taints)
		for _, taint := range sorted {
			if !first {
				nodeTaintsArgs.WriteString(",")
			}
			first = false
			nodeTaintsArgs.WriteString(fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return nodeTaintsArgs
}

func (p *LaunchTemplateProvider) getInstanceProfile(ctx context.Context, constraints *v1alpha1.Constraints) (string, error) {
	if constraints.InstanceProfile != nil {
		return aws.StringValue(constraints.InstanceProfile), nil
	}
	defaultProfile := injection.GetOptions(ctx).AWSDefaultInstanceProfile
	if defaultProfile == "" {
		return "", errors.New("neither spec.provider.instanceProfile nor --aws-default-instance-profile is specified")
	}
	return defaultProfile, nil
}

func (p *LaunchTemplateProvider) GetCABundle(ctx context.Context) (*string, error) {
	// Discover CA Bundle from the REST client. We could alternatively
	// have used the simpler client-go InClusterConfig() method.
	// However, that only works when Karpenter is running as a Pod
	// within the same cluster it's managing.
	restConfig := injection.GetConfig(ctx)
	if restConfig == nil {
		return nil, nil
	}
	transportConfig, err := restConfig.TransportConfig()
	if err != nil {
		logging.FromContext(ctx).Debugf("Unable to discover caBundle, loading transport config, %v", err)
		return nil, err
	}
	_, err = transport.TLSConfigFor(transportConfig) // fills in CAData!
	if err != nil {
		logging.FromContext(ctx).Debugf("Unable to discover caBundle, loading TLS config, %v", err)
		return nil, err
	}
	logging.FromContext(ctx).Debugf("Discovered caBundle, length %d", len(transportConfig.TLS.CAData))
	return ptr.String(base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)), nil
}
