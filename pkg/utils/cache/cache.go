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

package cache

import (
	"fmt"
	"sync"
)

type Option func(Options) Options

type Options struct {
	ignoreCache bool
}

func IgnoreCacheOption(o Options) Options {
	o.ignoreCache = true
	return o
}

// TryGetStringWithFallback attempts to get non-nil string value from field. If field is nil, the function
// will attempt to resolve the value by calling fallback, setting the value stored in field in-place if found.
func TryGetStringWithFallback(mu *sync.RWMutex, field *string, fallback func() (string, error), opts ...Option) (string, error) {
	o := resolveOptions(opts)
	mu.RLock()
	if field != nil && !o.ignoreCache {
		ret := *field
		mu.RUnlock()
		return ret, nil
	}
	mu.RUnlock()
	mu.Lock()
	defer mu.Unlock()
	// We have to check if the field is set again here in case multiple threads make it past the read-locked section
	if field != nil {
		return *field, nil
	}
	ret, err := fallback()
	if err != nil {
		return "", err
	}
	if ret == "" {
		return "", fmt.Errorf("return value didn't resolve to non-nil value")
	}
	*field = ret
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
