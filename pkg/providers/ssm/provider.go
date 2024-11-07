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

package ssm

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"
)

type Provider interface {
	Get(context.Context, Parameter) (string, error)
}

type DefaultProvider struct {
	sync.Mutex
	cache  *cache.Cache
	ssmapi ssmiface.SSMAPI
}

func NewDefaultProvider(ssmapi ssmiface.SSMAPI, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ssmapi: ssmapi,
		cache:  cache,
	}
}

func (p *DefaultProvider) Get(ctx context.Context, parameter Parameter) (string, error) {
	p.Lock()
	defer p.Unlock()
	if entry, ok := p.cache.Get(parameter.CacheKey()); ok {
		return entry.(CacheEntry).Value, nil
	}
	result, err := p.ssmapi.GetParameterWithContext(ctx, parameter.GetParameterInput())
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q, %w", parameter.Name, err)
	}
	p.cache.SetDefault(parameter.CacheKey(), CacheEntry{
		Parameter: parameter,
		Value:     lo.FromPtr(result.Parameter.Value),
	})
	logging.FromContext(ctx).With("parameter", parameter.Name, "value", result.Parameter.Value).Info("discovered ssm parameter")
	return lo.FromPtr(result.Parameter.Value), nil
}
