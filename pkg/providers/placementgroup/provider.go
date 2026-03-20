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

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
)

type Provider interface {
	// Get resolves a single placement group from a PlacementGroupSelectorTerm.
	Get(context.Context, v1.PlacementGroupSelectorTerm) (*ec2types.PlacementGroup, error)
}

type DefaultProvider struct {
	sync.Mutex

	ec2api sdk.EC2API
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
}

func NewProvider(ec2api sdk.EC2API, placementGroupCache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2api,
		cache:  placementGroupCache,
		cm:     pretty.NewChangeMonitor(),
	}
}

func (p *DefaultProvider) Get(ctx context.Context, term v1.PlacementGroupSelectorTerm) (*ec2types.PlacementGroup, error) {
	p.Lock()
	defer p.Unlock()

	q := &Query{ID: term.ID, Name: term.Name}

	if entry, ok := p.cache.Get(q.CacheKey()); ok {
		return entry.(*ec2types.PlacementGroup), nil
	}

	out, err := p.ec2api.DescribePlacementGroups(ctx, q.DescribePlacementGroupsInput())
	if err != nil {
		if awserrors.IsNotFound(err) {
			p.cache.Delete(q.CacheKey())
			return nil, nil
		}
		return nil, fmt.Errorf("describing placement groups, %w", err)
	}
	if len(out.PlacementGroups) == 0 {
		return nil, nil
	}

	pg := &out.PlacementGroups[0]
	p.cache.SetDefault(q.CacheKey(), pg)
	return pg, nil
}
