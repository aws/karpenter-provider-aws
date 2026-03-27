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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
)

type Query struct {
	ID   string
	Name string
}

func (q *Query) CacheKey() string {
	return fmt.Sprintf("%d", lo.Must(hashstructure.Hash(q, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets: true,
	})))
}

func (q *Query) DescribePlacementGroupsInput() *ec2.DescribePlacementGroupsInput {
	input := &ec2.DescribePlacementGroupsInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("state"),
				Values: []string{string(ec2types.PlacementGroupStateAvailable)},
			},
		},
	}
	if q.ID != "" {
		input.GroupIds = []string{q.ID}
	} else if q.Name != "" {
		input.GroupNames = []string{q.Name}
	}
	return input
}
