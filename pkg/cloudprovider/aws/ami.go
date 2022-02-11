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
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
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

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, accelerator, etc
func (p *AMIProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType) (map[string][]cloudprovider.InstanceType, error) {
	version, err := p.kubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("kube server version, %w", err)
	}
	// Separate instance types by unique queries
	amiQueries := map[string][]cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		query := p.getSSMQuery(constraints, instanceType, version)
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
	p.cache.SetDefault(query, ami)
	logging.FromContext(ctx).Debugf("Discovered %s for query %s", ami, query)
	return ami, nil
}

func (p *AMIProvider) getSSMQuery(constraints *v1alpha1.Constraints, instanceType cloudprovider.InstanceType, version string) string {
	switch aws.StringValue(constraints.AMIFamily) {
	case v1alpha1.AMIFamilyBottlerocket:
		return p.getBottlerocketAlias(version, instanceType)
	case v1alpha1.AMIFamilyUbuntu:
		return p.getUbuntuAlias(version, instanceType)
	}
	return p.getAL2Alias(version, instanceType)
}

// getAL2Alias returns a properly-formatted alias for an Amazon Linux AMI from SSM
func (p *AMIProvider) getAL2Alias(version string, instanceType cloudprovider.InstanceType) string {
	amiSuffix := ""
	if !instanceType.NvidiaGPUs().IsZero() || !instanceType.AWSNeurons().IsZero() {
		amiSuffix = "-gpu"
	} else if instanceType.Architecture() == v1alpha5.ArchitectureArm64 {
		amiSuffix = fmt.Sprintf("-%s", instanceType.Architecture())
	}
	return fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, amiSuffix)
}

// getBottlerocketAlias returns a properly-formatted alias for a Bottlerocket AMI from SSM
func (p *AMIProvider) getBottlerocketAlias(version string, instanceType cloudprovider.InstanceType) string {
	arch := "x86_64"
	amiSuffix := ""
	if !instanceType.NvidiaGPUs().IsZero() {
		amiSuffix = "-nvidia"
	}
	if instanceType.Architecture() == v1alpha5.ArchitectureArm64 {
		arch = instanceType.Architecture()
	}
	return fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s%s/%s/latest/image_id", version, amiSuffix, arch)
}

// getUbuntuAlias returns a properly-formatted alias for an Ubuntu AMI from SSM
func (p *AMIProvider) getUbuntuAlias(version string, instanceType cloudprovider.InstanceType) string {
	return fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/%s/hvm/ebs-gp2/ami-id", version, instanceType.Architecture())
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
