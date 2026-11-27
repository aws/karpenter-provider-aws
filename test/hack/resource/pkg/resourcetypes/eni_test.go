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
	"slices"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
)

func TestLeakedBranchENI(t *testing.T) {
	live := liveSnapshot("live-1")
	available, inUse := ec2types.NetworkInterfaceStatusAvailable, ec2types.NetworkInterfaceStatusInUse
	withDescription := func(ni ec2types.NetworkInterface, d string) ec2types.NetworkInterface {
		ni.Description = lo.ToPtr(d)
		return ni
	}
	withTags := func(ni ec2types.NetworkInterface, tags ...ec2types.Tag) ec2types.NetworkInterface {
		ni.TagSet = tags
		return ni
	}
	owner := ec2types.Tag{Key: lo.ToPtr(vpcResourceControllerOwnerTag), Value: lo.ToPtr(vpcResourceControllerOwner)}
	for _, tc := range []struct {
		name     string
		ni       ec2types.NetworkInterface
		excluded []string
		live     *LiveClusters
		want     bool
	}{
		{name: "leaked", ni: branchENI("eni-1", "vpc-1", "gone-1", available), want: true},
		{name: "attached", ni: branchENI("eni-1", "vpc-1", "gone-1", inUse)},
		{name: "live cluster", ni: branchENI("eni-1", "vpc-1", "live-1", available)},
		{name: "excluded cluster", ni: branchENI("eni-1", "vpc-1", "gone-1", available), excluded: []string{"gone-1"}},
		{name: "failed snapshot", ni: branchENI("eni-1", "vpc-1", "gone-1", available), live: failedSnapshot()},
		{name: "vpc cni eni", ni: withDescription(branchENI("eni-1", "vpc-1", "gone-1", available), "aws-K8S-i-0123")},
		{name: "trunk eni", ni: withDescription(branchENI("eni-1", "vpc-1", "gone-1", available), "aws-k8s-trunk-eni")},
		{name: "no owner tag", ni: withTags(branchENI("eni-1", "vpc-1", "gone-1", available), ec2types.Tag{Key: lo.ToPtr("kubernetes.io/cluster/gone-1"), Value: lo.ToPtr("owned")})},
		{name: "no cluster tag", ni: withTags(branchENI("eni-1", "vpc-1", "gone-1", available), owner)},
		{name: "two cluster tags", ni: withTags(branchENI("eni-1", "vpc-1", "gone-1", available), owner,
			ec2types.Tag{Key: lo.ToPtr("kubernetes.io/cluster/gone-1"), Value: lo.ToPtr("owned")},
			ec2types.Tag{Key: lo.ToPtr("kubernetes.io/cluster/gone-2"), Value: lo.ToPtr("owned")})},
		{name: "shared cluster tag", ni: withTags(branchENI("eni-1", "vpc-1", "gone-1", available), owner, ec2types.Tag{Key: lo.ToPtr("kubernetes.io/cluster/gone-1"), Value: lo.ToPtr("shared")})},
	} {
		l := live
		if tc.live != nil {
			l = tc.live
		}
		if got := leakedBranchENI(tc.ni, tc.excluded, l); got != tc.want {
			t.Errorf("%s: leakedBranchENI() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestENIGetExpiredIncludesLeakedBranchENIs(t *testing.T) {
	f := &fakeEC2{enis: []ec2types.NetworkInterface{
		branchENI("eni-leaked", "vpc-1", "gone-1", ec2types.NetworkInterfaceStatusAvailable),
		branchENI("eni-live", "vpc-2", "live-1", ec2types.NetworkInterfaceStatusAvailable),
		branchENI("eni-attached", "vpc-1", "gone-1", ec2types.NetworkInterfaceStatusInUse),
	}}
	ids, err := NewENI(f, liveSnapshot("live-1")).GetExpired(context.Background(), testNow, nil)
	if err != nil || !slices.Equal(ids, []string{"eni-leaked"}) {
		t.Errorf("GetExpired() = %v, %v, want [eni-leaked]", ids, err)
	}
	ids, err = NewENI(f, failedSnapshot()).GetExpired(context.Background(), testNow, nil)
	if err == nil || len(ids) != 0 {
		t.Errorf("GetExpired() = %v, %v, want an error and no ENIs", ids, err)
	}
}
