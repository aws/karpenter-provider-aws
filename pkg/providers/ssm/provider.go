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

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
)

type Provider interface {
	List(context.Context, string) (map[string]string, error)
	Get(context.Context, string) (string, error)
}

type DefaultProvider struct {
	sync.Mutex
	cache *cache.Cache
	ssmapi ssmiface.SSMAPI
}

func NewDefaultProvider(ssmapi ssmiface.SSMAPI, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ssmapi: ssmapi,
		cache: cache,
	}
}

func (p *DefaultProvider) List(ctx context.Context, path string) (map[string]string, error) {
	p.Lock()
	defer p.Unlock()
	if paths, ok := p.cache.Get(path); ok {
		return paths.(map[string]string), nil
	}
	values := map[string]string{}
	if err := p.ssmapi.GetParametersByPathPagesWithContext(ctx, &ssm.GetParametersByPathInput{
		Recursive: lo.ToPtr(true),
		Path: &path,
	}, func(out *ssm.GetParametersByPathOutput, _ bool) bool {
		for _, parameter := range out.Parameters {
			if parameter.Name == nil || parameter.Value == nil {
				continue
			}
			values[*parameter.Name] = *parameter.Value
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("getting ssm parameters for path %q, %w", path, err)
	}
	p.cache.SetDefault(path, values)
	return values, nil
}

func (p *DefaultProvider) Get(ctx context.Context, path string) (string, error) {
	p.Lock()
	defer p.Unlock()
	if val, ok := p.cache.Get(path); ok {
		return val.(string), nil
	}
	out, err := p.ssmapi.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: &path})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q, %w", path, err)
	}
	return lo.FromPtr(out.Parameter.Value), err
}
