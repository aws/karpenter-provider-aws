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

package capacityreservation

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type Query struct {
	ids     []string
	ownerID string
	tags    map[string]string
}

func QueriesFromSelectorTerms(terms ...v1.CapacityReservationSelectorTerm) []*Query {
	queries := []*Query{}
	ids := []string{}
	for i := range terms {
		if terms[i].ID != "" {
			ids = append(ids, terms[i].ID)
		}
		queries = append(queries, &Query{
			ownerID: terms[i].OwnerID,
			tags:    terms[i].Tags,
		})
	}
	if len(ids) != 0 {
		queries = append(queries, &Query{ids: ids})
	}
	return queries
}

func (q *Query) CacheKey() string {
	return fmt.Sprintf("%d", lo.Must(hashstructure.Hash(q, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets: true,
	})))
}

func (q *Query) DescribeCapacityReservationsInput() *ec2.DescribeCapacityReservationsInput {
	if len(q.ids) != 0 {
		return &ec2.DescribeCapacityReservationsInput{
			Filters:                []ec2types.Filter{lo.Must(q.stateFilter())[0]},
			CapacityReservationIds: q.ids,
		}
	}
	type filterProvider func() ([]ec2types.Filter, bool)
	return &ec2.DescribeCapacityReservationsInput{
		Filters: lo.Flatten(lo.FilterMap([]filterProvider{
			q.stateFilter,
			q.ownerIDFilter,
			q.tagsFilter,
		}, func(f filterProvider, _ int) ([]ec2types.Filter, bool) {
			return f()
		})),
	}
}

func (q *Query) stateFilter() ([]ec2types.Filter, bool) {
	return []ec2types.Filter{{
		Name:   lo.ToPtr("state"),
		Values: []string{string(ec2types.CapacityReservationStateActive)},
	}}, true
}

func (q *Query) ownerIDFilter() ([]ec2types.Filter, bool) {
	return []ec2types.Filter{{
		Name:   lo.ToPtr("owner-id"),
		Values: []string{q.ownerID},
	}}, q.ownerID != ""
}

func (q *Query) tagsFilter() ([]ec2types.Filter, bool) {
	return lo.MapToSlice(q.tags, func(k, v string) ec2types.Filter {
		if v == "*" {
			return ec2types.Filter{
				Name:   lo.ToPtr("tag-key"),
				Values: []string{k},
			}
		}
		return ec2types.Filter{
			Name:   lo.ToPtr(fmt.Sprintf("tag:%s", k)),
			Values: []string{v},
		}
	}), len(q.tags) != 0

}
