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

package placementgroup

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Provider interface {
	Get(context.Context, *v1.PlacementGroup) (*ec2types.PlacementGroup, error)
}

type DefaultProvider struct {
	sync.Mutex
	ec2api sdk.EC2API
	cache  *cache.Cache
}

func NewDefaultProvider(ec2api sdk.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2api,
		cache:  cache,
	}
}

func (p *DefaultProvider) Get(ctx context.Context, placementGroup *v1.PlacementGroup) (*ec2types.PlacementGroup, error) {
	if placementGroup == nil {
		return nil, nil
	}

	p.Lock()
	defer p.Unlock()

	if cached, ok := p.cache.Get(cacheKey(placementGroup)); ok {
		pg := cached.(ec2types.PlacementGroup)
		return &pg, nil
	}

	input := &ec2.DescribePlacementGroupsInput{}
	if placementGroup.ID != "" {
		input.GroupIds = []string{placementGroup.ID}
	} else {
		input.GroupNames = []string{placementGroup.Name}
	}
	out, err := p.ec2api.DescribePlacementGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describing placement groups, %w", err)
	}
	if len(out.PlacementGroups) == 0 {
		return nil, nil
	}
	if len(out.PlacementGroups) != 1 {
		return nil, fmt.Errorf("expected one placement group, got %d", len(out.PlacementGroups))
	}
	p.cache.SetDefault(cacheKey(placementGroup), out.PlacementGroups[0])
	return &out.PlacementGroups[0], nil
}

func cacheKey(pg *v1.PlacementGroup) string {
	if pg.ID != "" {
		return "id:" + pg.ID
	}
	return "name:" + pg.Name
}
