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
	"strings"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

// ValidationEntry stores a validation result. An empty Reason indicates a successful validation. For failures, both
// the condition reason and the enriched message derived from the AWS error are stored so cached results can
// reproduce the same condition.
type ValidationEntry struct {
	Reason  string
	Message string
}

// Validation caches the results of EC2NodeClass validation. Results can be populated either by the dry-run
// validation performed by the nodeclass controller or by successful CreateFleet calls made by the instance provider.
type Validation struct {
	// key: <nodeclass name>:<hash of nodeclass + tags>, value: ValidationEntry
	cache *cache.Cache
}

// NewValidation returns a Validation cache backed by the provided go-cache. The backing cache's default TTL is used
// for all entries.
func NewValidation(c *cache.Cache) *Validation {
	return &Validation{cache: c}
}

// SetSuccess marks validation as successful for the given nodeclass and tags
func (v *Validation) SetSuccess(nodeClass *v1.EC2NodeClass, tags map[string]string) {
	v.cache.SetDefault(v.key(nodeClass, tags), ValidationEntry{})
}

// SetFailure marks validation as failed for the given nodeclass and tags
func (v *Validation) SetFailure(nodeClass *v1.EC2NodeClass, tags map[string]string, reason, message string) {
	v.cache.SetDefault(v.key(nodeClass, tags), ValidationEntry{Reason: reason, Message: message})
}

// Get returns the cached validation result for the given nodeclass and tags, if one exists
func (v *Validation) Get(nodeClass *v1.EC2NodeClass, tags map[string]string) (ValidationEntry, bool) {
	val, ok := v.cache.Get(v.key(nodeClass, tags))
	if !ok {
		return ValidationEntry{}, false
	}
	return val.(ValidationEntry), true
}

// ClearCacheEntries removes all cache entries associated with the given nodeclass from the validation cache
func (v *Validation) ClearCacheEntries(nodeClass *v1.EC2NodeClass) {
	var toDelete []string
	for key := range v.cache.Items() {
		parts := strings.Split(key, ":")
		// NOTE: should never occur, indicates malformed cache key
		if len(parts) != 2 {
			continue
		}
		if parts[0] == nodeClass.Name {
			toDelete = append(toDelete, key)
		}
	}
	for _, key := range toDelete {
		v.cache.Delete(key)
	}
}

func (v *Validation) Items() map[string]cache.Item {
	return v.cache.Items()
}

func (v *Validation) Flush() {
	v.cache.Flush()
}

func (v *Validation) key(nodeClass *v1.EC2NodeClass, tags map[string]string) string {
	// We omit the nodepool tag as this is expected to differ between dry-run validation and CreateFleet calls.
	// The nodepool does not impact the validation of a nodeclass.
	filteredTags := lo.OmitByKeys(tags, []string{v1.NodePoolTagKey})
	hash := lo.Must(hashstructure.Hash([]any{
		nodeClass.Status.Subnets,
		nodeClass.Status.SecurityGroups,
		nodeClass.Status.AMIs,
		nodeClass.Status.InstanceProfile,
		nodeClass.Spec,
		nodeClass.Annotations,
		filteredTags,
		nodeClass.Spec.Kubelet.String(),
	}, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
	return fmt.Sprintf("%s:%016x", nodeClass.Name, hash)
}
