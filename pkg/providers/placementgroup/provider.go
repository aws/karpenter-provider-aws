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
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
)

type Provider interface {
	// Get resolves the placement group for a nodeclass. It uses an in-memory cache keyed by
	// NodeClass UID and generation to avoid unnecessary EC2 API calls. When the cache entry
	// expires (TTL) or the NodeClass spec changes (bumping generation), the next call
	// re-resolves from EC2. Returns nil when no placement group is configured.
	// On transient EC2 errors, returns the last known good result and surfaces the error.
	// On not-found errors, clears the resolved state and returns nil.
	Get(context.Context, *v1.EC2NodeClass) (*PlacementGroup, error)
}

type DefaultProvider struct {
	sync.RWMutex

	ec2api sdk.EC2API
	cache  *cache.Cache

	// resolved stores the last known good placement group for each nodeclass, keyed by UID.
	// This is preserved across transient EC2 errors so callers can continue to use stale data.
	resolved map[types.UID]*PlacementGroup
}

func NewProvider(ec2api sdk.EC2API, placementGroupCache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api:   ec2api,
		cache:    placementGroupCache,
		resolved: make(map[types.UID]*PlacementGroup),
	}
}

// cacheKey returns a key that incorporates both the UID and generation,
// so spec changes naturally cause a cache miss without separate generation tracking.
func cacheKey(nodeClass *v1.EC2NodeClass) string {
	return fmt.Sprintf("%s:%d", nodeClass.UID, nodeClass.Generation)
}

func (p *DefaultProvider) Get(ctx context.Context, nodeClass *v1.EC2NodeClass) (*PlacementGroup, error) {
	uid := nodeClass.UID

	if nodeClass.Spec.PlacementGroupSelector == nil {
		p.Lock()
		delete(p.resolved, uid)
		p.cache.Delete(cacheKey(nodeClass))
		p.Unlock()
		return nil, nil
	}

	p.RLock()
	if _, ok := p.cache.Get(cacheKey(nodeClass)); ok {
		resolved := p.resolved[uid]
		p.RUnlock()
		return resolved, nil
	}
	p.RUnlock()

	term := *nodeClass.Spec.PlacementGroupSelector
	q := &Query{ID: term.ID, Name: term.Name}

	out, err := p.ec2api.DescribePlacementGroups(ctx, q.DescribePlacementGroupsInput())
	if err != nil {
		if awserrors.IsNotFound(err) {
			p.Lock()
			delete(p.resolved, uid)
			p.cache.Delete(cacheKey(nodeClass))
			p.Unlock()
			return nil, nil
		}
		// Transient error (EC2 outage, throttling) — return last known good, surface error
		p.RLock()
		resolved := p.resolved[uid]
		p.RUnlock()
		return resolved, fmt.Errorf("describing placement groups, %w", err)
	}

	if len(out.PlacementGroups) == 0 {
		p.Lock()
		delete(p.resolved, uid)
		p.cache.Delete(cacheKey(nodeClass))
		p.Unlock()
		return nil, nil
	}

	resolved := PlacementGroupFromEC2(&out.PlacementGroups[0])

	p.Lock()
	p.resolved[uid] = resolved
	p.cache.SetDefault(cacheKey(nodeClass), struct{}{})
	p.Unlock()

	return resolved, nil
}
