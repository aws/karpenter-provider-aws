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

	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type NodeClass interface {
	client.Object
	PlacementGroupSelector() *v1.PlacementGroupSelector
}

type Provider interface {
	Get(context.Context, NodeClass) (*PlacementGroup, error)
}

type DefaultProvider struct {
	ec2api              sdk.EC2API
	pgCache             *cache.Cache
	pgAvailabilityCache *cache.Cache
}

func NewProvider(ec2api sdk.EC2API, pgCache *cache.Cache, pgAvailabilityCache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api:              ec2api,
		pgCache:             pgCache,
		pgAvailabilityCache: pgAvailabilityCache,
	}
}

func (p *DefaultProvider) Get(ctx context.Context, nodeClass NodeClass) (*PlacementGroup, error) {
	if nodeClass.PlacementGroupSelector() == nil {
		return nil, nil
	}

	term := nodeClass.PlacementGroupSelector()
	q := &Query{ID: term.ID, Name: term.Name}
	key := q.CacheKey()

	if _, ok := p.pgCache.Get(key); ok {
		if pg, ok := p.pgAvailabilityCache.Get(key); ok {
			return pg.(*PlacementGroup), nil
		}
		return nil, nil
	}

	out, err := p.ec2api.DescribePlacementGroups(ctx, q.DescribePlacementGroupsInput())
	if err != nil {
		if pg, ok := p.pgAvailabilityCache.Get(key); ok {
			return pg.(*PlacementGroup), fmt.Errorf("describing placement groups, %w", err)
		}
		return nil, fmt.Errorf("describing placement groups, %w", err)
	}

	var resolved *PlacementGroup
	if len(out.PlacementGroups) > 0 {
		resolved = PlacementGroupFromEC2(&out.PlacementGroups[0])
		p.pgAvailabilityCache.SetDefault(key, resolved)
	}
	p.pgCache.SetDefault(key, struct{}{})
	return resolved, nil
}
