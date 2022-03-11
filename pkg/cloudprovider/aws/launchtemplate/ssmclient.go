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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"
)

type SSMClient interface {
	GetParameterWithContext(ctx context.Context, query string) (string, error)
}

type AWSSSMClient struct {
	ssm ssmiface.SSMAPI
}

func NewAWSSSMClient(ssm ssmiface.SSMAPI) *AWSSSMClient {
	return &AWSSSMClient{
		ssm: ssm,
	}
}

func (p *AWSSSMClient) GetParameterWithContext(ctx context.Context, query string) (string, error) {
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(query)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter, %w", err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	return ami, nil
}

type CachingSSMClient struct {
	cache *cache.Cache
	inner SSMClient
}

func NewCachingSSMClient(inner SSMClient) *CachingSSMClient {
	return &CachingSSMClient{
		inner: inner,
		cache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (p *CachingSSMClient) GetParameterWithContext(ctx context.Context, query string) (string, error) {
	if imageID, ok := p.cache.Get(query); ok {
		if cachedValue, ok := imageID.(string); ok {
			return cachedValue, nil
		}
		if cachedError, ok := imageID.(error); ok {
			return "", cachedError
		}
	}
	imageID, err := p.inner.GetParameterWithContext(ctx, query)
	if err != nil {
		// Csache negative lookup results to avoid running into API rate limits.
		p.cache.SetDefault(query, err)
		return "", err
	}
	p.cache.SetDefault(query, imageID)
	logging.FromContext(ctx).Debugf("Discovered '%s' for query '%s'", imageID, query)
	return imageID, nil
}
