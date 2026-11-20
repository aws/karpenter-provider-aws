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

package nodeclass

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"

	. "github.com/onsi/gomega"
)

func TestNetworkCompatibilityFinalizationClearsState(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clk := clocktesting.NewFakeClock(time.Now())
	api := fake.NewEC2API()
	api.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{{SubnetId: aws.String("subnet-test1"), VpcId: aws.String("vpc-other")}}})
	subnets := subnet.NewDefaultProvider(api, cache.New(cache.NoExpiration, 0), cache.New(cache.NoExpiration, 0))
	groups := securitygroup.NewDefaultProvider(api, cache.New(cache.NoExpiration, 0))
	recorder := coretest.NewEventRecorder()
	c := NewController(clk, nil, nil, recorder, "test-region", subnets, groups,
		nil, nil, nil, nil, nil, nil, api, nil, nil, nil, nil, false)
	var network *NetworkCompatibility
	for _, reconciler := range c.reconcilers {
		if r, ok := reconciler.(*NetworkCompatibility); ok {
			network = r
		}
	}
	g.Expect(network).NotTo(BeNil())
	nodeClass := &v1.EC2NodeClass{
		ObjectMeta: metav1.ObjectMeta{Name: "deleted", UID: "deleted-uid"},
		Spec: v1.EC2NodeClassSpec{
			SubnetSelectorTerms:        []v1.SubnetSelectorTerm{{ID: "subnet-test1"}},
			SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{{ID: "sg-test1"}},
		},
		Status: v1.EC2NodeClassStatus{
			Subnets:        []v1.Subnet{{ID: "subnet-test1"}},
			SecurityGroups: []v1.SecurityGroup{{ID: "sg-test1"}},
		},
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeSubnetsReady)
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeSecurityGroupsReady)
	_, err := network.Reconcile(ctx, nodeClass)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).IsFalse()).To(BeTrue())
	other := nodeClass.DeepCopy()
	other.Name, other.UID = "live", "live-uid"
	_, err = network.Reconcile(ctx, other)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(network.eventLimiters.Len()).To(Equal(2))

	// Retained mismatch history must also be cleaned up if the last diagnostic was Unknown.
	nodeClass.StatusConditions().SetUnknown(v1.ConditionTypeSubnetsReady)
	_, err = network.Reconcile(ctx, nodeClass)
	g.Expect(err).NotTo(HaveOccurred())
	nodeClass.DeletionTimestamp = &metav1.Time{Time: clk.Now()}
	for range 2 { // cleanup is idempotent, including after the finalizer has been removed
		_, err = c.Reconcile(ctx, nodeClass)
		g.Expect(err).NotTo(HaveOccurred())
	}
	var remaining []string
	network.incompatibilities.Range(func(key, _ any) bool {
		remaining = append(remaining, key.(string))
		return true
	})
	g.Expect(remaining).To(ConsistOf(fmt.Sprintf("%s/%s", other.Name, other.UID)))
	g.Expect(network.eventLimiters.Len()).To(Equal(1))
	g.Expect(api.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(1))
	g.Expect(recorder.Calls("NetworkCompatibilityRestored")).To(BeZero())
}
