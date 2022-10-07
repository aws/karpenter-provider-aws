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

package atomic

import (
	"context"
	"sync"

	"github.com/aws/karpenter/pkg/utils/ptr"
)

type Option func(Options) Options

type Options struct {
	ignoreCache bool
}

func IgnoreCacheOption(o Options) Options {
	o.ignoreCache = true
	return o
}

// CachedVal persistently stores a value in memory
type CachedVal[T any] struct {
	value   *T
	mu      sync.RWMutex
	Resolve func(context.Context) (T, error)
}

func (c *CachedVal[T]) Set(v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = &v
}

// TryGet attempts to get non-nil value from internal value. If field is nil, the function
// will attempt to resolve the value by calling fallback, setting the value stored in value in-place if found.
func (c *CachedVal[T]) TryGet(ctx context.Context, opts ...Option) (T, error) {
	o := resolveOptions(opts)
	c.mu.RLock()
	if c.value != nil && !o.ignoreCache {
		ret := *c.value
		c.mu.RUnlock()
		return ret, nil
	}
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	// We have to check if the field is set again here in case multiple threads make it past the read-locked section
	if c.value != nil && !o.ignoreCache {
		return *c.value, nil
	}
	if c.Resolve == nil {
		return *new(T), nil
	}
	ret, err := c.Resolve(ctx)
	if err != nil {
		return *new(T), err
	}
	c.value = ptr.Val(ret) // copies the value so we don't keep the reference
	return ret, nil
}

func resolveOptions(opts []Option) Options {
	o := Options{}
	for _, opt := range opts {
		if opt != nil {
			o = opt(o)
		}
	}
	return o
}
