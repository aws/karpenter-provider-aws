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

// Validation caches the results of EC2 node class validation
type Validation struct {
	// key: hash of nodeclass + tags, value: string
	validationCache *cache.Cache
}

func NewValidation() *Validation {
	return &Validation{
		validationCache: cache.New(ValidationTTL, DefaultCleanupInterval),
	}
}

// SetSuccess marks validation as successful (i.e. value in cache as empty string)
func (v *Validation) SetSuccess(nodeClass *v1.EC2NodeClass, tags map[string]string) {
	v.validationCache.SetDefault(v.key(nodeClass, tags), "")
}

// SetFailure marks validation as a failure (i.e. value in cache not a empty string)
func (v *Validation) SetFailure(nodeClass *v1.EC2NodeClass, tags map[string]string, failureReason string) {
	v.validationCache.SetDefault(v.key(nodeClass, tags), failureReason)
}

func (v *Validation) Get(nodeClass *v1.EC2NodeClass, tags map[string]string) (interface{}, bool) {
	return v.validationCache.Get(v.key(nodeClass, tags))
}

// ClearCacheEntries removes all cache entries associated with the given nodeclass from the validation cache
func (v *Validation) ClearCacheEntries(nodeClass *v1.EC2NodeClass) {
	var toDelete []string
	for key := range v.validationCache.Items() {
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
		v.validationCache.Delete(key)
	}
}

func (v *Validation) Items() map[string]cache.Item {
	return v.validationCache.Items()
}

func (v *Validation) Flush() {
	v.validationCache.Flush()
}

func (v *Validation) key(nodeClass *v1.EC2NodeClass, tags map[string]string) string {
	// we omit the nodepool tag as this is expected to differ between dry-run validation and CreateFleet calls
	// the nodepool does not impact the validation of an node class
	filteredTags := lo.OmitByKeys(tags, []string{v1.NodePoolTagKey})
	hash := lo.Must(hashstructure.Hash([]interface{}{
		nodeClass.Status.Subnets,
		nodeClass.Status.SecurityGroups,
		nodeClass.Status.AMIs,
		nodeClass.Status.InstanceProfile,
		nodeClass.Spec.MetadataOptions,
		nodeClass.Spec.BlockDeviceMappings,
		filteredTags,
	}, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
	return fmt.Sprintf("%s:%016x", nodeClass.Name, hash)
}
