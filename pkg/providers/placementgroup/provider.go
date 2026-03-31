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

	"github.com/patrickmn/go-cache"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
)

type Provider interface {
	// Get resolves the placement group for a nodeclass from EC2, stores it in-memory,
	// and returns it. Called by the nodeclass reconciler.
	Get(context.Context, *v1.EC2NodeClass) (*PlacementGroup, error)
	// GetForNodeClass returns the in-memory resolved placement group for a nodeclass.
	// Returns nil if no placement group is configured, has not been resolved yet,
	// or the resolved data is stale (nodeclass generation has changed since resolution).
	GetForNodeClass(*v1.EC2NodeClass) *PlacementGroup
	// Clear removes the in-memory resolved placement group for a nodeclass.
	Clear(*v1.EC2NodeClass)
}

// resolvedEntry pairs a resolved placement group with the nodeclass generation
// it was resolved for. This ensures that callers of GetForNodeClass never see
// stale data from a previous spec revision.
type resolvedEntry struct {
	pg         *PlacementGroup
	generation int64
}

type DefaultProvider struct {
	sync.RWMutex

	ec2api sdk.EC2API
	cache  *cache.Cache

	// resolved stores the resolved placement group for each nodeclass, keyed by nodeclass name
	resolved map[string]*resolvedEntry
}

func NewProvider(ec2api sdk.EC2API, placementGroupCache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api:   ec2api,
		cache:    placementGroupCache,
		resolved: make(map[string]*resolvedEntry),
	}
}

func (p *DefaultProvider) Get(ctx context.Context, nodeClass *v1.EC2NodeClass) (*PlacementGroup, error) {
	if nodeClass.Spec.PlacementGroupSelector == nil {
		p.Clear(nodeClass)
		return nil, nil
	}

	term := *nodeClass.Spec.PlacementGroupSelector
	q := &Query{ID: term.ID, Name: term.Name}

	p.RLock()
	if entry, ok := p.cache.Get(q.CacheKey()); ok {
		resolved := entry.(*PlacementGroup)
		p.RUnlock()
		p.Lock()
		p.resolved[nodeClass.Name] = &resolvedEntry{pg: resolved, generation: nodeClass.Generation}
		p.Unlock()
		return resolved, nil
	}
	p.RUnlock()

	out, err := p.ec2api.DescribePlacementGroups(ctx, q.DescribePlacementGroupsInput())
	if err != nil {
		if awserrors.IsNotFound(err) {
			p.Lock()
			p.cache.Delete(q.CacheKey())
			delete(p.resolved, nodeClass.Name)
			p.Unlock()
			return nil, nil
		}
		p.Clear(nodeClass)
		return nil, fmt.Errorf("describing placement groups, %w", err)
	}
	if len(out.PlacementGroups) == 0 {
		p.Clear(nodeClass)
		return nil, nil
	}

	resolved := PlacementGroupFromEC2(&out.PlacementGroups[0])

	p.Lock()
	p.cache.SetDefault(q.CacheKey(), resolved)
	p.resolved[nodeClass.Name] = &resolvedEntry{pg: resolved, generation: nodeClass.Generation}
	p.Unlock()

	return resolved, nil
}

func (p *DefaultProvider) GetForNodeClass(nodeClass *v1.EC2NodeClass) *PlacementGroup {
	p.RLock()
	defer p.RUnlock()
	entry := p.resolved[nodeClass.Name]
	if entry == nil || entry.generation != nodeClass.Generation {
		return nil
	}
	// Validate the resolved PG still matches the current spec selector as a defense-in-depth check
	if nodeClass.Spec.PlacementGroupSelector == nil {
		return nil
	}
	term := nodeClass.Spec.PlacementGroupSelector
	if term.ID != "" && entry.pg.ID != term.ID {
		return nil
	}
	if term.Name != "" && entry.pg.Name != term.Name {
		return nil
	}
	return entry.pg
}

func (p *DefaultProvider) Clear(nodeClass *v1.EC2NodeClass) {
	p.Lock()
	defer p.Unlock()
	delete(p.resolved, nodeClass.Name)
}
