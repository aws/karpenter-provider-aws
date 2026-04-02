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

type Strategy string

const (
	StrategyCluster   Strategy = "cluster"
	StrategyPartition Strategy = "partition"
	StrategySpread    Strategy = "spread"
)

type SpreadLevel string

const (
	SpreadLevelRack SpreadLevel = "rack"
	SpreadLevelHost SpreadLevel = "host"
)

// PlacementGroup represents a resolved EC2 placement group stored in-memory by the provider.
type PlacementGroup struct {
	// ID is the placement group ID (e.g., "pg-0123456789abcdef0")
	ID string
	// Name is the placement group name
	Name string
	// PartitionCount is the number of partitions for partition placement groups
	PartitionCount int32
	// SpreadLevel is the spread level for spread placement groups
	SpreadLevel SpreadLevel
	// Strategy is the placement group strategy
	Strategy Strategy
}

// PlacementGroupFromEC2 converts an EC2 PlacementGroup to the provider's PlacementGroup type.
func PlacementGroupFromEC2(pg *ec2types.PlacementGroup) *PlacementGroup {
	return &PlacementGroup{
		ID:             lo.FromPtr(pg.GroupId),
		Name:           lo.FromPtr(pg.GroupName),
		PartitionCount: lo.FromPtr(pg.PartitionCount),
		SpreadLevel:    SpreadLevel(pg.SpreadLevel),
		Strategy:       Strategy(pg.Strategy),
	}
}

// Query represents a placement group lookup query by name or ID.
type Query struct {
	ID   *string
	Name *string
}

func (q *Query) CacheKey() string {
	return fmt.Sprintf("%d", lo.Must(hashstructure.Hash(q, hashstructure.FormatV2, &hashstructure.HashOptions{})))
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
	if id := lo.FromPtr(q.ID); id != "" {
		input.GroupIds = []string{id}
	} else if name := lo.FromPtr(q.Name); name != "" {
		input.GroupNames = []string{name}
	}
	return input
}
