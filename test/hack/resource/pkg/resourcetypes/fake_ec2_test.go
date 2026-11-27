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
	"slices"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
)

type fakeEC2 struct {
	mu    sync.Mutex
	enis  []ec2types.NetworkInterface
	calls []string
}

func matchesFilter(ni ec2types.NetworkInterface, f ec2types.Filter) bool {
	name := lo.FromPtr(f.Name)
	switch {
	case name == "description":
		return slices.Contains(f.Values, lo.FromPtr(ni.Description))
	case name == "status":
		return slices.Contains(f.Values, string(ni.Status))
	case name == "vpc-id":
		return slices.Contains(f.Values, lo.FromPtr(ni.VpcId))
	case name == "tag-key":
		return lo.ContainsBy(ni.TagSet, func(t ec2types.Tag) bool { return slices.Contains(f.Values, lo.FromPtr(t.Key)) })
	case strings.HasPrefix(name, "tag:"):
		return lo.ContainsBy(ni.TagSet, func(t ec2types.Tag) bool {
			return lo.FromPtr(t.Key) == strings.TrimPrefix(name, "tag:") && slices.Contains(f.Values, lo.FromPtr(t.Value))
		})
	}
	panic(fmt.Sprintf("unsupported filter %s", name))
}

func (f *fakeEC2) DescribeNetworkInterfaces(_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: lo.Filter(f.enis, func(ni ec2types.NetworkInterface, _ int) bool {
		return lo.EveryBy(in.Filters, func(filter ec2types.Filter) bool { return matchesFilter(ni, filter) })
	})}, nil
}

func (f *fakeEC2) DeleteNetworkInterface(_ context.Context, in *ec2.DeleteNetworkInterfaceInput, _ ...func(*ec2.Options)) (*ec2.DeleteNetworkInterfaceOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "DeleteNetworkInterface "+lo.FromPtr(in.NetworkInterfaceId))
	f.enis = lo.Reject(f.enis, func(ni ec2types.NetworkInterface, _ int) bool {
		return lo.FromPtr(ni.NetworkInterfaceId) == lo.FromPtr(in.NetworkInterfaceId)
	})
	return &ec2.DeleteNetworkInterfaceOutput{}, nil
}

// branchENI builds a network interface the way the VPC resource controller tags branch ENIs
func branchENI(id, vpc, cluster string, status ec2types.NetworkInterfaceStatus) ec2types.NetworkInterface {
	return ec2types.NetworkInterface{
		NetworkInterfaceId: lo.ToPtr(id),
		VpcId:              lo.ToPtr(vpc),
		Description:        lo.ToPtr(branchENIDescription),
		Status:             status,
		TagSet: []ec2types.Tag{
			{Key: lo.ToPtr(vpcResourceControllerOwnerTag), Value: lo.ToPtr(vpcResourceControllerOwner)},
			{Key: lo.ToPtr(clusterOwnedTagPrefix + cluster), Value: lo.ToPtr("owned")},
			{Key: lo.ToPtr("vpcresources.k8s.aws/trunk-eni-id"), Value: lo.ToPtr("eni-trunk")},
		},
	}
}
