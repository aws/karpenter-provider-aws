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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/patrickmn/go-cache"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
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

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, acclerator, etc
func (p *AMIProvider) Get(ctx context.Context, instanceTypes []cloudprovider.InstanceType) (map[string][]cloudprovider.InstanceType, error) {
	version, err := p.kubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("kube server version, %w", err)
	}
	// Separate instance types by unique queries
	amiQueries := map[string][]cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		query := p.getSSMQuery(instanceType, version)
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

func (p *AMIProvider) getAMIID(ctx context.Context, query string) (string, error) {
	if id, ok := p.cache.Get(query); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(query)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter, %w", err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.cache.Set(query, ami, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered ami %s for query %s", ami, query)
	return ami, nil
}

func (p *AMIProvider) getSSMQuery(instanceType cloudprovider.InstanceType, version string) string {
	var amiSuffix string
	if !instanceType.NvidiaGPUs().IsZero() || !instanceType.AWSNeurons().IsZero() {
		amiSuffix = "-gpu"
	} else if instanceType.Architecture() == v1alpha5.ArchitectureArm64 {
		amiSuffix = "-arm64"
	}
	return fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, amiSuffix)
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
	p.cache.Set(kubernetesVersionCacheKey, version, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered kubernetes version %s", version)
	return version, nil
}
