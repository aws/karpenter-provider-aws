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
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

const (
	// CreationQPS limits the number of requests per second to CreateFleet
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits
	CreationQPS = 2
	// CreationBurst limits the additional burst requests.
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits
	CreationBurst = 100
	// CacheTTL restricts QPS to AWS APIs to this interval for verifying setup
	// resources. This value represents the maximum eventual consistency between
	// AWS actual state and the controller's ability to provision those
	// resources. Cache hits enable faster provisioning and reduced API load on
	// AWS APIs, which can have a serious impact on performance and scalability.
	// DO NOT CHANGE THIS VALUE WITHOUT DUE CONSIDERATION
	CacheTTL = 60 * time.Second
	// CacheCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	CacheCleanupInterval = 10 * time.Minute
)

type Builder struct {
	K8sClient             K8sClient
	SsmClient             SSMClient
	AmiResolver           AMIResolver
	SecurityGroupprovider SecurityGroupResolver
}

func NewBuilder(k8sClient K8sClient, ssm SSMClient, ami AMIResolver, sp SecurityGroupResolver) *Builder {
	return &Builder{
		K8sClient:             k8sClient,
		SsmClient:             ssm,
		AmiResolver:           ami,
		SecurityGroupprovider: sp,
	}
}

func convertBlockDeviceMapping(m *ec2.BlockDeviceMapping) *ec2.LaunchTemplateBlockDeviceMappingRequest {
	merged := ec2.LaunchTemplateBlockDeviceMappingRequest{
		DeviceName:  m.DeviceName,
		NoDevice:    m.NoDevice,
		VirtualName: m.VirtualName,
	}
	if m.Ebs != nil {
		merged.Ebs = &ec2.LaunchTemplateEbsBlockDeviceRequest{
			VolumeType:          m.Ebs.VolumeType,
			VolumeSize:          m.Ebs.VolumeSize,
			Iops:                m.Ebs.Iops,
			Throughput:          m.Ebs.Throughput,
			KmsKeyId:            m.Ebs.KmsKeyId,
			SnapshotId:          m.Ebs.SnapshotId,
			Encrypted:           m.Ebs.Encrypted,
			DeleteOnTermination: m.Ebs.DeleteOnTermination,
		}
	}
	return &merged
}

func (b *Builder) adaptVolumeMapping(defaults *ec2.BlockDeviceMapping, volumeOverrides *v1alpha1.BlockDeviceMapping) *ec2.LaunchTemplateBlockDeviceMappingRequest {
	merged := convertBlockDeviceMapping(defaults)
	if volumeOverrides != nil {
		if volumeOverrides.Ebs != nil {
			if merged.Ebs == nil {
				merged.Ebs = &ec2.LaunchTemplateEbsBlockDeviceRequest{
					VolumeType: aws.String(ec2.VolumeTypeGp3),
					VolumeSize: aws.Int64(20),
				}
				// If .Ebs is present in override, clear instance-store parameters
				merged.VirtualName = nil
				merged.NoDevice = nil
			}
			if volumeOverrides.Ebs.VolumeType != nil {
				merged.Ebs.VolumeType = volumeOverrides.Ebs.VolumeType
			}
			if volumeOverrides.Ebs.VolumeSize != nil {
				merged.Ebs.VolumeSize = aws.Int64(volumeOverrides.Ebs.VolumeSize.ScaledValue(resource.Giga))
			}
			if volumeOverrides.Ebs.Iops != nil {
				merged.Ebs.Iops = volumeOverrides.Ebs.Iops
			}
			if volumeOverrides.Ebs.Throughput != nil {
				merged.Ebs.Throughput = volumeOverrides.Ebs.Throughput
			}
			if volumeOverrides.Ebs.KmsKeyID != nil {
				merged.Ebs.KmsKeyId = volumeOverrides.Ebs.KmsKeyID
			}
		}
	}
	if merged.Ebs != nil {
		// Hardcode termination behaviour as everything else does not make sense.
		merged.Ebs.DeleteOnTermination = aws.Bool(true)
		// Hardcode security best practice, not allowing for unencrypted volumes at all.
		merged.Ebs.Encrypted = aws.Bool(true)
	}
	return merged
}

func (b *Builder) blockDeviceMappings(ami *ec2.Image, rootVolume *v1alpha1.EbsVolume, extraMappings []v1alpha1.BlockDeviceMapping) []*ec2.LaunchTemplateBlockDeviceMappingRequest {
	// First, complement the rootVolume with its DeviceName provided by the AMI image and
	// add it to the set of mappings.
	root := v1alpha1.BlockDeviceMapping{
		DeviceName: ami.RootDeviceName,
		Ebs:        rootVolume,
	}
	extraMappings = append([]v1alpha1.BlockDeviceMapping{root}, extraMappings...)
	mappings := make([]*ec2.LaunchTemplateBlockDeviceMappingRequest, len(extraMappings))
OUTER:
	for _, defaultMapping := range ami.BlockDeviceMappings {
		for i := range extraMappings {
			m := &extraMappings[i]
			if defaultMapping.DeviceName == m.DeviceName {
				// If an override is present merge it with the base configuration from the AMI.
				mappings[i] = b.adaptVolumeMapping(defaultMapping, m)
				continue OUTER
			}
		}
		if defaultMapping.Ebs != nil {
			// If no override is present and the mapping from the AMI is an EBS one, add it.
			// TODO Later we might add support for instance-store volumes as well.
			mappings = append(mappings, convertBlockDeviceMapping(defaultMapping))
		}
	}
	return mappings
}

func orDefaultSecurityGroupSelector(configuration *Configuration, filter map[string]string) map[string]string {
	if len(filter) > 0 {
		filters := make(map[string]string)
		for key, value := range filter {
			// Support placeholder for cluster name, to enable cluster name agnostic
			// filtering.
			if key == "kubernetes.io/cluster/{{ClusterName}}" {
				key = fmt.Sprintf("kubernetes.io/cluster/%s", configuration.ClusterName)
			}
			filters[key] = value
		}
		return filters
	}
	// If not filter is provided use the default filter, which will select all SecurityGroups
	// owned by the specific EKS cluster instance.
	return map[string]string{
		fmt.Sprintf("kubernetes.io/cluster/%s", configuration.ClusterName): "owned",
	}
}

func (b *Builder) Template(ctx context.Context, provider OsProvider, input *v1alpha1.BasicLaunchTemplateInput, config *Configuration, ami *ec2.Image, instanceTypes []cloudprovider.InstanceType) (*ec2.RequestLaunchTemplateData, error) {
	template := &ec2.RequestLaunchTemplateData{}
	var instanceProfile *string
	if input != nil {
		instanceProfile = input.InstanceProfile
		template.MetadataOptions = input.MetadataOptions.WithDefaults()
		template.BlockDeviceMappings = b.blockDeviceMappings(ami, input.RootVolume, input.ExtraBlockDevices)
		securityGroupIds, err := b.SecurityGroupprovider.Get(ctx, orDefaultSecurityGroupSelector(config, input.SecurityGroupSelector))
		if err != nil {
			return nil, err
		}
		template.SecurityGroupIds = aws.StringSlice(securityGroupIds)
	} else {
		securityGroupIds, err := b.SecurityGroupprovider.Get(ctx, orDefaultSecurityGroupSelector(config, nil))
		if err != nil {
			return nil, err
		}
		template.SecurityGroupIds = aws.StringSlice(securityGroupIds)
		template.MetadataOptions = (*v1alpha1.MetadataOptions)(nil).WithDefaults()
	}
	if template.ImageId == nil {
		template.ImageId = ami.ImageId
	}
	instanceProfileSpec, err := b.prepareInstanceProfile(instanceProfile, config)
	if err != nil {
		return nil, err
	}
	template.IamInstanceProfile = instanceProfileSpec
	raw, err := provider.GetUserData(ctx, b, config, instanceTypes)
	if err != nil {
		return nil, err
	}
	if raw != nil {
		template.UserData = pointer.String(base64.StdEncoding.EncodeToString([]byte(*raw)))
	}
	// Apply tags as CreateFleet does not allow tagging volumes.
	template.TagSpecifications = b.prepareTags(config.Constraints)
	return template, nil
}

func (b *Builder) prepareInstanceProfile(instanceProfile *string, config *Configuration) (*ec2.LaunchTemplateIamInstanceProfileSpecificationRequest, error) {
	if instanceProfile == nil && len(config.DefaultInstanceProfile) > 0 {
		instanceProfile = &config.DefaultInstanceProfile
	}
	if instanceProfile != nil {
		var instanceProfileName *string
		var instanceProfileArn *string
		if strings.HasPrefix(*instanceProfile, "arn:") {
			instanceProfileArn = instanceProfile
		} else {
			instanceProfileName = instanceProfile
		}
		return &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
			Name: instanceProfileName,
			Arn:  instanceProfileArn,
		}, nil
	}
	return nil, errors.New("neither provider specific instanceProfile nor --aws-default-instance-profile is specified")
}

func (b *Builder) prepareTags(constraints *v1alpha1.Constraints) []*ec2.LaunchTemplateTagSpecificationRequest {
	var tags []*ec2.Tag
	if constraints.Tags != nil {
		tags = make([]*ec2.Tag, len(constraints.Tags))
		for key, value := range constraints.Tags {
			tags = append(tags, &ec2.Tag{
				Key:   pointer.String(key),
				Value: pointer.String(value),
			})
		}
	}
	if tags != nil {
		tagRequests := make([]*ec2.LaunchTemplateTagSpecificationRequest, 2)
		tagRequests = append(tagRequests, &ec2.LaunchTemplateTagSpecificationRequest{
			ResourceType: pointer.String(ec2.ResourceTypeInstance),
			Tags:         tags,
		})
		tagRequests = append(tagRequests, &ec2.LaunchTemplateTagSpecificationRequest{
			ResourceType: pointer.String(ec2.ResourceTypeVolume),
			Tags:         tags,
		})
		return tagRequests
	}
	return nil
}

func (b *Builder) PrepareLauncheTemplates(ctx context.Context, provider OsProvider, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[Input][]cloudprovider.InstanceType, error) {
	IDMapping, err := b.mapInstanceTypesToImageID(ctx, provider, config, instanceTypes)
	if err != nil {
		return nil, err
	}
	imageMapping, err := b.resolveAmis(ctx, IDMapping)
	if err != nil {
		return nil, err
	}
	mapping := make(map[Input][]cloudprovider.InstanceType)
	for ami, instanceTypes := range imageMapping {
		template, err := provider.PrepareLaunchTemplate(ctx, b, config, ami, instanceTypes)
		if err != nil {
			return nil, err
		}
		mapping[Input{
			ByContent: template,
		}] = instanceTypes
	}
	return mapping, nil
}

func (b *Builder) resolveAmis(ctx context.Context, input map[string][]cloudprovider.InstanceType) (map[*ec2.Image][]cloudprovider.InstanceType, error) {
	mapping := make(map[*ec2.Image][]cloudprovider.InstanceType)
	for imageID, values := range input {
		image, err := b.AmiResolver.GetImage(ctx, imageID)
		if err != nil {
			return nil, err
		}
		mapping[image] = values
	}
	return mapping, nil
}

func (b *Builder) mapInstanceTypesToImageID(ctx context.Context, provider OsProvider, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[string][]cloudprovider.InstanceType, error) {
	mapping := make(map[string][]cloudprovider.InstanceType)
	for _, t := range instanceTypes {
		imageID, err := provider.GetImageID(ctx, b, config, t)
		if err != nil {
			return nil, err
		}
		if entry, ok := mapping[imageID]; ok {
			mapping[imageID] = append(entry, t)
		} else {
			mapping[imageID] = []cloudprovider.InstanceType{t}
		}
	}
	return mapping, nil
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
