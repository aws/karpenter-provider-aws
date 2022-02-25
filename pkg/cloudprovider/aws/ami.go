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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/patrickmn/go-cache"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/launchtemplate"
)

const kubernetesVersionCacheKey = "kubernetesVersion"

type AMIProvider struct {
	cache     *cache.Cache
	ssm       ssmiface.SSMAPI
	clientSet *kubernetes.Clientset
}

func NewAMIProvider(ssm ssmiface.SSMAPI, clientSet *kubernetes.Clientset) *AMIProvider {
	return &AMIProvider{
		ssm:       ssm,
		clientSet: clientSet,
		cache:     cache.New(CacheTTL, CacheCleanupInterval),
	}
}

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, accelerator, etc
func (p *AMIProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType, amiFamily launchtemplate.AMIFamily) (map[string][]cloudprovider.InstanceType, error) {
	version, err := p.kubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("kube server version, %w", err)
	}
	// Separate instance types by unique queries
	amiQueries := map[string][]cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		query := amiFamily.SSMAlias(version, instanceType)
		amiQueries[query] = append(amiQueries[query], instanceType)
	}
	// Separate instance types by unique AMIIDs
	amiIDs := map[string][]cloudprovider.InstanceType{}
	for query, instanceTypes := range amiQueries {
		amiID, err := p.getAMIID(ctx, query)
		if err != nil {
			return nil, err
		}
		amiIDs[amiID] = instanceTypes
	}
	return amiIDs, nil
}

func (p *AMIProvider) kubeServerVersion(ctx context.Context) (string, error) {
	if version, ok := p.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	serverVersion, err := p.clientSet.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	version := fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+"))
	p.cache.SetDefault(kubernetesVersionCacheKey, version)
	logging.FromContext(ctx).Debugf("Discovered kubernetes version %s", version)
	return version, nil
}

func (p *AMIProvider) getAMIID(ctx context.Context, query string) (string, error) {
	if id, ok := p.cache.Get(query); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(query)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter, %w", err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.cache.SetDefault(query, ami)
	logging.FromContext(ctx).Debugf("Discovered %s for query %s", ami, query)
	return ami, nil
}
