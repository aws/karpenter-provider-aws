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

package resourcetypes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/samber/lo"
)

// A nil or failed LiveClusters snapshot reports every cluster as live, so the sweeps that rely on it fail closed
type LiveClusters struct {
	names map[string]struct{}
	err   error
}

type ClusterLister func(ctx context.Context, region string) ([]string, error)

type regionsAPI interface {
	DescribeRegions(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
}

func EKSClusterLister(cfg aws.Config) ClusterLister {
	return func(ctx context.Context, region string) ([]string, error) {
		client := eks.NewFromConfig(cfg, func(o *eks.Options) { o.Region = region })
		var names []string
		paginator := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
		for paginator.HasMorePages() {
			out, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			names = append(names, out.Clusters...)
		}
		return names, nil
	}
}

// SnapshotLiveClusters lists clusters in every enabled region
func SnapshotLiveClusters(ctx context.Context, api regionsAPI, lister ClusterLister, currentRegion string) *LiveClusters {
	// Without AllRegions, DescribeRegions only returns the regions enabled in the account
	out, err := api.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return &LiveClusters{err: fmt.Errorf("describing regions, %w", err)}
	}
	regions := lo.FilterMap(out.Regions, func(r ec2types.Region, _ int) (string, bool) {
		return lo.FromPtr(r.RegionName), lo.FromPtr(r.RegionName) != ""
	})
	if currentRegion != "" {
		regions = append(regions, currentRegion)
	}
	return ListLiveClusters(ctx, lister, lo.Uniq(regions))
}

// ListLiveClusters fails the whole snapshot if any region can't be listed
func ListLiveClusters(ctx context.Context, lister ClusterLister, regions []string) *LiveClusters {
	live := &LiveClusters{names: map[string]struct{}{}}
	if len(regions) == 0 {
		live.err = fmt.Errorf("no regions to list clusters in")
		return live
	}
	for _, region := range regions {
		names, err := lister(ctx, region)
		if err != nil {
			live.err = fmt.Errorf("listing eks clusters in %s, %w", region, err)
			return live
		}
		for _, name := range names {
			live.names[name] = struct{}{}
		}
	}
	return live
}

func (l *LiveClusters) Err() error {
	if l == nil {
		return fmt.Errorf("live clusters were not listed")
	}
	return l.err
}

func (l *LiveClusters) Gone(clusterName string) bool {
	if l.Err() != nil || clusterName == "" {
		return false
	}
	_, found := l.names[clusterName]
	return !found
}
