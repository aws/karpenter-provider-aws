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

func (p *AMIProvider) Get(ctx context.Context, constraints *Constraints) (string, error) {
	version, err := p.kubeServerVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("kube server version, %w", err)
	}
	name := fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/%s/latest/image_id", version, KubeToAWSArchitectures[*constraints.Architecture])
	if id, ok := p.cache.Get(name); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(name)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter, %w", err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.cache.Set(name, ami, CacheTTL)
	logging.FromContext(ctx).Debugf("Discovered ami %s for query %s", ami, name)
	return ami, nil
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
