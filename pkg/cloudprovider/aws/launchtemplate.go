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
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"k8s.io/client-go/transport"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/launchtemplate"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
)

const (
	launchTemplateNameFormat  = "Karpenter-%s-%s"
	kubernetesVersionCacheKey = "kubernetesVersion"
)

type LaunchTemplateProvider struct {
	sync.Mutex
	logger  *zap.SugaredLogger
	ec2api  ec2iface.EC2API
	builder *launchtemplate.Builder
	cache   *cache.Cache
}

func NewLaunchTemplateProvider(ctx context.Context, ec2api ec2iface.EC2API, builder *launchtemplate.Builder) *LaunchTemplateProvider {
	l := &LaunchTemplateProvider{
		ec2api:  ec2api,
		logger:  logging.FromContext(ctx).Named("launchtemplate"),
		builder: builder,
		cache:   cache.New(CacheTTL, CacheCleanupInterval),
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
	ClusterName string
	Tags        map[string]string
	Options     *ec2.RequestLaunchTemplateData
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType, additionalLabels map[string]string) (map[*v1alpha1.LauchtemplateReference][]cloudprovider.InstanceType, error) {
	osProvider := launchtemplate.OSProviderOf(&constraints.AWS)
	k8sVersion, err := p.builder.K8sClient.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	caBundle, err := p.GetCABundle(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ca bundle for user data, %w", err)
	}
	nodeLabelArgs := functional.UnionStringMaps(additionalLabels, constraints.Labels)
	configuration := &launchtemplate.Configuration{
		Constraints:            constraints,
		ClusterName:            injection.GetOptions(ctx).ClusterName,
		ClusterEndpoint:        injection.GetOptions(ctx).ClusterEndpoint,
		DefaultInstanceProfile: injection.GetOptions(ctx).AWSDefaultInstanceProfile,
		KubernetesVersion:      *k8sVersion,
		NodeLabels:             nodeLabelArgs,
		CABundle:               caBundle,
	}
	launchTemplates, err := osProvider.GetLaunchTemplates(ctx, p.builder, configuration, instanceTypes)
	if err != nil {
		return nil, err
	}
	result := make(map[*v1alpha1.LauchtemplateReference][]cloudprovider.InstanceType)
	for templateInput, compatibleInstanceTypes := range launchTemplates {
		if templateInput.ByReference != nil {
			// If Launch Template is directly specified then just use it
			result[templateInput.ByReference] = compatibleInstanceTypes
		} else {
			input := templateInput.ByContent
			launchTemplate, err := p.ensureLaunchTemplate(ctx, &launchTemplateOptions{
				ClusterName: injection.GetOptions(ctx).ClusterName,
				Tags:        constraints.Tags,
				Options:     input,
			})
			if err != nil {
				return nil, err
			}
			result[&v1alpha1.LauchtemplateReference{
				LaunchTemplateName: launchTemplate.LaunchTemplateName,
			}] = compatibleInstanceTypes
		}
	}
	return result, nil
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
	p.cache.SetDefault(name, launchTemplate)
	return launchTemplate, nil
}

func (p *LaunchTemplateProvider) createLaunchTemplate(ctx context.Context, options *launchTemplateOptions) (*ec2.LaunchTemplate, error) {
	output, err := p.ec2api.CreateLaunchTemplateWithContext(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(launchTemplateName(options)),
		LaunchTemplateData: options.Options,
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
		p.logger.Errorf("Unable to delete launch template, %v", err)
		return
	}
	p.logger.Debugf("Deleted launch template %v", aws.StringValue(launchTemplate.LaunchTemplateId))
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
