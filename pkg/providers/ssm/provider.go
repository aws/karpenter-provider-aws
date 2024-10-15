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

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Provider interface {
	Get(context.Context, string) (string, error)
}

type DefaultProvider struct {
	sync.Mutex
	cache  *cache.Cache
	ssmapi sdk.SSMAPI
}

func NewDefaultProvider(ssmapi sdk.SSMAPI, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ssmapi: ssmapi,
		cache:  cache,
	}
}

func (p *DefaultProvider) Get(ctx context.Context, parameter string) (string, error) {
	p.Lock()
	defer p.Unlock()
	if result, ok := p.cache.Get(parameter); ok {
		return result.(string), nil
	}
	result, err := p.ssmapi.GetParameter(ctx, &ssm.GetParameterInput{
		Name: lo.ToPtr(parameter),
	})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q, %w", parameter, err)
	}
	p.cache.SetDefault(parameter, lo.FromPtr(result.Parameter.Value))
	log.FromContext(ctx).WithValues("parameter", parameter, "value", result.Parameter.Value).Info("discovered ssm parameter")
	return lo.FromPtr(result.Parameter.Value), nil
}
