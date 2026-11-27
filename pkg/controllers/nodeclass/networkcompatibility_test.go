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

package nodeclass_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/awslabs/operatorpkg/status"
	"github.com/patrickmn/go-cache"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/events"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"

	. "github.com/onsi/gomega"
)

// These tests use the real discovery providers without starting envtest.
type networkFixture struct {
	api                     *fake.EC2API
	clk                     *clocktesting.FakeClock
	nodeClass               *v1.EC2NodeClass
	reconciler              *nodeclass.NetworkCompatibility
	recorder                *coretest.EventRecorder
	subnets                 *subnet.DefaultProvider
	groups                  *securitygroup.DefaultProvider
	subnetCache, groupCache *cache.Cache
}

func newNetworkFixture(t *testing.T) *networkFixture {
	t.Helper()
	f := &networkFixture{
		api: fake.NewEC2API(),
		clk: clocktesting.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		nodeClass: &v1.EC2NodeClass{
			ObjectMeta: metav1.ObjectMeta{Name: "network-test", UID: types.UID("network-test-uid")},
			Spec: v1.EC2NodeClassSpec{
				SubnetSelectorTerms:        []v1.SubnetSelectorTerm{{Tags: map[string]string{"foo": "bar"}}},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"foo": "bar"}}},
			},
		},
		recorder:    coretest.NewEventRecorder(),
		subnetCache: cache.New(cache.NoExpiration, 0),
		groupCache:  cache.New(cache.NoExpiration, 0),
	}
	f.subnets = subnet.NewDefaultProvider(f.api, f.subnetCache, cache.New(cache.NoExpiration, 0))
	f.groups = securitygroup.NewDefaultProvider(f.api, f.groupCache)
	f.reconciler = nodeclass.NewNetworkCompatibilityReconciler(f.clk, f.recorder, f.subnets, f.groups, f.api)
	f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
	for _, condition := range []string{
		v1.ConditionTypeAMIsReady, v1.ConditionTypeSubnetsReady, v1.ConditionTypeSecurityGroupsReady,
		v1.ConditionTypeInstanceProfileReady, v1.ConditionTypeValidationSucceeded,
		v1.ConditionTypePlacementGroupReady, v1.ConditionTypeCapacityReservationsReady,
	} {
		f.nodeClass.StatusConditions(status.WithClock(f.clk)).SetTrue(condition)
	}
	return f
}

func networkSubnet(id, vpc string) ec2types.Subnet {
	return ec2types.Subnet{SubnetId: aws.String(id), VpcId: aws.String(vpc), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(100)}
}

func networkGroup(id, vpc string) ec2types.SecurityGroup {
	return ec2types.SecurityGroup{GroupId: aws.String(id), GroupName: aws.String(id), VpcId: aws.String(vpc)}
}

func networkAssociation(group, vpc string, state ec2types.SecurityGroupVpcAssociationState) ec2types.SecurityGroupVpcAssociation {
	return ec2types.SecurityGroupVpcAssociation{GroupId: aws.String(group), VpcId: aws.String(vpc), State: state}
}

func (f *networkFixture) resources(subnets []ec2types.Subnet, groups []ec2types.SecurityGroup) {
	f.subnetCache.Flush()
	f.groupCache.Flush()
	f.api.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: subnets})
	f.api.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: groups})
	f.nodeClass.Status.Subnets = nil
	f.nodeClass.Status.SecurityGroups = nil
	for _, s := range subnets {
		f.nodeClass.Status.Subnets = append(f.nodeClass.Status.Subnets, v1.Subnet{ID: aws.ToString(s.SubnetId)})
	}
	for _, sg := range groups {
		f.nodeClass.Status.SecurityGroups = append(f.nodeClass.Status.SecurityGroups, v1.SecurityGroup{ID: aws.ToString(sg.GroupId)})
	}
}

func (f *networkFixture) reconcile(t *testing.T) reconcile.Result {
	t.Helper()
	result, err := f.reconciler.Reconcile(context.Background(), f.nodeClass)
	NewWithT(t).Expect(err).NotTo(HaveOccurred())
	return result
}

func TestNetworkCompatibilityConditions(t *testing.T) {
	cases := []struct {
		name             string
		setup            func(*networkFixture)
		condition        metav1.ConditionStatus
		reason           string
		associationCalls int
	}{
		{name: "same VPC needs no association permission", condition: metav1.ConditionTrue, reason: "Compatible"},
		{name: "only status resources are evaluated", setup: func(f *networkFixture) {
			f.api.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{networkSubnet("subnet-a", "vpc-a"), networkSubnet("subnet-extra", "vpc-b")}})
			f.api.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a"), networkGroup("sg-extra", "vpc-c")}})
		}, condition: metav1.ConditionTrue, reason: "Compatible"},
		{name: "partial mismatch", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-b", "vpc-b"), networkSubnet("subnet-a", "vpc-a")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
		}, condition: metav1.ConditionFalse, reason: "PartiallyIncompatible", associationCalls: 1},
		{name: "all subnets mismatch", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-b"), networkSubnet("subnet-b", "vpc-c")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
		}, condition: metav1.ConditionFalse, reason: "NoCompatibleSubnet", associationCalls: 1},
		{name: "each subnet must support the entire SG set", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a"), networkSubnet("subnet-b", "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a"), networkGroup("sg-b", "vpc-b")})
		}, condition: metav1.ConditionFalse, reason: "NoCompatibleSubnet", associationCalls: 1},
		{name: "active association plus own VPC", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a"), networkSubnet("subnet-b", "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
			f.api.DescribeSecurityGroupVpcAssociationsBehavior.Output.Set(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{networkAssociation("sg-a", "vpc-b", ec2types.SecurityGroupVpcAssociationStateAssociated)}})
		}, condition: metav1.ConditionTrue, reason: "Compatible", associationCalls: 1},
		{name: "one associated SG does not cover the other SGs", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-c")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a"), networkGroup("sg-b", "vpc-b")})
			f.api.DescribeSecurityGroupVpcAssociationsBehavior.Output.Set(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{networkAssociation("sg-a", "vpc-c", ec2types.SecurityGroupVpcAssociationStateAssociated)}})
		}, condition: metav1.ConditionFalse, reason: "NoCompatibleSubnet", associationCalls: 1},
		{name: "no selectors and no resolved SGs stays unknown", setup: func(f *networkFixture) {
			f.nodeClass.Spec.SecurityGroupSelectorTerms = nil
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a")}, nil)
		}, condition: metav1.ConditionUnknown, reason: "DependenciesNotReady"},
		{name: "selected SGs but empty status", setup: func(f *networkFixture) { f.nodeClass.Status.SecurityGroups = nil }, condition: metav1.ConditionUnknown, reason: "DependenciesNotReady"},
		{name: "empty subnet status", setup: func(f *networkFixture) { f.nodeClass.Status.Subnets = nil }, condition: metav1.ConditionUnknown, reason: "DependenciesNotReady"},
		{name: "discovery not ready", setup: func(f *networkFixture) {
			f.nodeClass.StatusConditions().SetFalse(v1.ConditionTypeSubnetsReady, "NotFound", "not found")
		}, condition: metav1.ConditionUnknown, reason: "DependenciesNotReady"},
		{name: "discovery unknown", setup: func(f *networkFixture) {
			f.nodeClass.StatusConditions().SetUnknown(v1.ConditionTypeSecurityGroupsReady)
		}, condition: metav1.ConditionUnknown, reason: "DependenciesNotReady"},
		{name: "missing subnet VPC", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "missing SG VPC", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a")}, []ec2types.SecurityGroup{networkGroup("sg-a", "")})
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "missing subnet ID", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("", "vpc-a")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "missing SG ID", setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a")}, []ec2types.SecurityGroup{networkGroup("", "vpc-a")})
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "missing status subnet in provider", setup: func(f *networkFixture) {
			f.nodeClass.Status.Subnets = append(f.nodeClass.Status.Subnets, v1.Subnet{ID: "subnet-missing"})
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "missing status SG in provider", setup: func(f *networkFixture) {
			f.nodeClass.Status.SecurityGroups = append(f.nodeClass.Status.SecurityGroups, v1.SecurityGroup{ID: "sg-missing"})
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "provider returns no status subnet", setup: func(f *networkFixture) { f.api.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{}) }, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "provider returns no status SG", setup: func(f *networkFixture) {
			f.api.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{})
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "subnet API failure", setup: func(f *networkFixture) { f.api.DescribeSubnetsBehavior.Error.Set(errors.New("subnet lookup failed")) }, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
		{name: "SG API failure", setup: func(f *networkFixture) {
			f.api.DescribeSecurityGroupsBehavior.Error.Set(errors.New("SG lookup failed"))
		}, condition: metav1.ConditionUnknown, reason: "CompatibilityCheckFailed"},
	}
	for _, state := range (ec2types.SecurityGroupVpcAssociationState("")).Values() {
		if state == ec2types.SecurityGroupVpcAssociationStateAssociated {
			continue
		}
		cases = append(cases, struct {
			name             string
			setup            func(*networkFixture)
			condition        metav1.ConditionStatus
			reason           string
			associationCalls int
		}{name: string(state), setup: func(f *networkFixture) {
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
			f.api.DescribeSecurityGroupVpcAssociationsBehavior.Output.Set(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{networkAssociation("sg-a", "vpc-b", state)}})
		}, condition: metav1.ConditionFalse, reason: "NoCompatibleSubnet", associationCalls: 1})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			f := newNetworkFixture(t)
			if tc.setup != nil {
				tc.setup(f)
			}
			before := f.nodeClass.DeepCopy()
			g.Expect(f.reconcile(t).RequeueAfter).To(Equal(awscache.DefaultTTL))
			condition := f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible)
			g.Expect(condition.Status).To(Equal(tc.condition))
			g.Expect(condition.Reason).To(Equal(tc.reason))
			g.Expect(f.api.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(tc.associationCalls))
			g.Expect(f.nodeClass.Spec).To(Equal(before.Spec))
			g.Expect(f.nodeClass.Status.Subnets).To(Equal(before.Status.Subnets))
			g.Expect(f.nodeClass.Status.SecurityGroups).To(Equal(before.Status.SecurityGroups))
			g.Expect(f.nodeClass.StatusConditions().Get(status.ConditionReady)).To(Equal(before.StatusConditions().Get(status.ConditionReady)))
			g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded)).To(Equal(before.StatusConditions().Get(v1.ConditionTypeValidationSucceeded)))
			if tc.condition == metav1.ConditionFalse {
				g.Expect(f.recorder.Calls("IncompatibleSubnetSecurityGroups")).To(Equal(1))
				g.Expect(condition.Message).To(ContainSubstring("subnetSelectorTerms"))
			}
		})
	}
}

func TestNetworkCompatibilityAssociationPaginationCacheAndRefresh(t *testing.T) {
	g := NewWithT(t)
	f := newNetworkFixture(t)
	f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-b"), networkSubnet("subnet-b", "vpc-c")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
	f.api.DescribeSecurityGroupVpcAssociationsBehavior.OutputPages.Add(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{networkAssociation("sg-a", "vpc-b", ec2types.SecurityGroupVpcAssociationStateAssociated)}})
	f.api.DescribeSecurityGroupVpcAssociationsBehavior.OutputPages.Add(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{networkAssociation("sg-a", "vpc-c", ec2types.SecurityGroupVpcAssociationStateAssociated)}})
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).IsTrue()).To(BeTrue())
	g.Expect(f.api.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(2))
	g.Expect(f.api.DescribeSecurityGroupVpcAssociationsBehavior.CalledWithInput.At(0).Filters).To(ConsistOf(
		ec2types.Filter{Name: aws.String("group-id"), Values: []string{"sg-a"}},
		ec2types.Filter{Name: aws.String("state"), Values: []string{"associated"}},
	))
	g.Expect(aws.ToString(f.api.DescribeSecurityGroupVpcAssociationsBehavior.CalledWithInput.At(1).NextToken)).NotTo(BeEmpty())
	f.reconcile(t)
	g.Expect(f.api.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(2))

	// Even an unfiltered/malformed response must not make an unrelated association count.
	f.api.DescribeSecurityGroupVpcAssociationsBehavior.Output.Set(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{
		networkAssociation("sg-other", "vpc-b", ec2types.SecurityGroupVpcAssociationStateAssociated),
		networkAssociation("sg-other", "vpc-c", ec2types.SecurityGroupVpcAssociationStateAssociated),
		networkAssociation("sg-a", "", ec2types.SecurityGroupVpcAssociationStateAssociated),
	}})
	f.clk.Step(awscache.DefaultTTL)
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("NoCompatibleSubnet"))
	g.Expect(f.api.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(3))

	// Empty association results are cached too; new associations are noticed at the next refresh.
	f.api.DescribeSecurityGroupVpcAssociationsBehavior.Output.Set(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{
		networkAssociation("sg-a", "vpc-b", ec2types.SecurityGroupVpcAssociationStateAssociated),
		networkAssociation("sg-a", "vpc-c", ec2types.SecurityGroupVpcAssociationStateAssociated),
	}})
	f.reconcile(t)
	g.Expect(f.api.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(3))
	f.clk.Step(awscache.DefaultTTL)
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).IsTrue()).To(BeTrue())
	g.Expect(f.recorder.Calls("NetworkCompatibilityRestored")).To(Equal(1))
}

// Override only this operation to exercise failure after a successful pagination page.
type networkAssociationAPI struct {
	*fake.EC2API
	describe func(*ec2.DescribeSecurityGroupVpcAssociationsInput) (*ec2.DescribeSecurityGroupVpcAssociationsOutput, error)
}

func (a *networkAssociationAPI) DescribeSecurityGroupVpcAssociations(_ context.Context, input *ec2.DescribeSecurityGroupVpcAssociationsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupVpcAssociationsOutput, error) {
	return a.describe(input)
}

func TestNetworkCompatibilityPaginatedFailureIsNotCached(t *testing.T) {
	g := NewWithT(t)
	f := newNetworkFixture(t)
	f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
	calls := 0
	api := &networkAssociationAPI{EC2API: f.api, describe: func(input *ec2.DescribeSecurityGroupVpcAssociationsInput) (*ec2.DescribeSecurityGroupVpcAssociationsOutput, error) {
		calls++
		switch calls {
		case 1:
			return &ec2.DescribeSecurityGroupVpcAssociationsOutput{NextToken: aws.String("page-2"), SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{networkAssociation("sg-a", "vpc-b", ec2types.SecurityGroupVpcAssociationStateAssociated)}}, nil
		case 2:
			g.Expect(aws.ToString(input.NextToken)).To(Equal("page-2"))
			return nil, errors.New("second page unavailable")
		default:
			g.Expect(input.NextToken).To(BeNil()) // retry the complete lookup
			return &ec2.DescribeSecurityGroupVpcAssociationsOutput{}, nil
		}
	}}
	f.reconciler = nodeclass.NewNetworkCompatibilityReconciler(f.clk, f.recorder, f.subnets, f.groups, api)
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("CompatibilityCheckFailed"))
	f.reconcile(t)
	g.Expect(calls).To(Equal(3))
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("NoCompatibleSubnet"))
}

func TestNetworkCompatibilityAssociationErrorsRetryAndRecovery(t *testing.T) {
	for _, err := range []error{errors.New("temporary EC2 failure"), &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "access denied"}} {
		t.Run(err.Error(), func(t *testing.T) {
			g := NewWithT(t)
			f := newNetworkFixture(t)
			f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
			f.reconcile(t) // confirmed mismatch
			f.clk.Step(awscache.DefaultTTL)
			f.api.DescribeSecurityGroupVpcAssociationsBehavior.Error.Set(err)
			f.reconcile(t)
			condition := f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible)
			g.Expect(condition.Status).To(Equal(metav1.ConditionUnknown))
			g.Expect(condition.Reason).To(Equal("CompatibilityCheckFailed"))
			if strings.Contains(err.Error(), "UnauthorizedOperation") {
				g.Expect(condition.Message).To(ContainSubstring("ec2:DescribeSecurityGroupVpcAssociations"))
			}
			f.api.DescribeSecurityGroupVpcAssociationsBehavior.Error.Reset()
			f.api.DescribeSecurityGroupVpcAssociationsBehavior.Output.Set(&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{networkAssociation("sg-a", "vpc-b", ec2types.SecurityGroupVpcAssociationStateAssociated)}})
			f.reconcile(t) // failure was not cached, so no clock advance is needed
			g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).IsTrue()).To(BeTrue())
			g.Expect(f.recorder.Calls("NetworkCompatibilityRestored")).To(Equal(1))
			f.reconcile(t)
			g.Expect(f.recorder.Calls("NetworkCompatibilityRestored")).To(Equal(1))
		})
	}
}

func TestNetworkCompatibilityUnknownIsNotAConfirmedMismatch(t *testing.T) {
	g := NewWithT(t)
	f := newNetworkFixture(t)
	f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
	f.api.DescribeSecurityGroupVpcAssociationsBehavior.Error.Set(errors.New(strings.Repeat("failure ", 1000)))
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("CompatibilityCheckFailed"))
	g.Expect(len(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Message)).To(BeNumerically("<", 2048))
	f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).IsTrue()).To(BeTrue())
	g.Expect(f.recorder.Calls("NetworkCompatibilityRestored")).To(BeZero())
}

func TestNetworkCompatibilityEventsAndBoundedMessages(t *testing.T) {
	g := NewWithT(t)
	f := newNetworkFixture(t)
	f.reconcile(t)
	g.Expect(f.recorder.Calls("NetworkCompatibilityRestored")).To(BeZero())
	var subnets []ec2types.Subnet
	var groups []ec2types.SecurityGroup
	for i := 0; i < 10; i++ {
		subnets = append(subnets, networkSubnet(fmt.Sprintf("subnet-%02d", i), "vpc-b"))
		groups = append(groups, networkGroup(fmt.Sprintf("sg-%02d", i), "vpc-a"))
	}
	f.resources(subnets, groups)
	f.reconcile(t)
	firstMessage := f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Message
	g.Expect(firstMessage).To(ContainSubstring("subnet-00"))
	g.Expect(firstMessage).NotTo(ContainSubstring("subnet-09"))
	g.Expect(firstMessage).NotTo(ContainSubstring("sg-09"))
	g.Expect(len(firstMessage)).To(BeNumerically("<", 4096))
	f.reconcile(t)
	g.Expect(f.recorder.Calls("IncompatibleSubnetSecurityGroups")).To(Equal(1))

	// Resource changes outside either truncation must still emit a new event.
	subnets[9] = networkSubnet("subnet-99", "vpc-b")
	f.resources(subnets, groups)
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Message).To(Equal(firstMessage))
	g.Expect(f.recorder.Calls("IncompatibleSubnetSecurityGroups")).To(Equal(2))
	groups[9] = networkGroup("sg-99", "vpc-a")
	f.resources(subnets, groups)
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Message).To(Equal(firstMessage))
	g.Expect(f.recorder.Calls("IncompatibleSubnetSecurityGroups")).To(Equal(3))

	// Order is immaterial to both message and deduplication.
	subnets[0], subnets[9] = subnets[9], subnets[0]
	groups[0], groups[9] = groups[9], groups[0]
	f.resources(subnets, groups)
	f.reconcile(t)
	g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Message).To(Equal(firstMessage))
	g.Expect(f.recorder.Calls("IncompatibleSubnetSecurityGroups")).To(Equal(3))

	f.nodeClass.StatusConditions().SetUnknown(v1.ConditionTypeSubnetsReady)
	f.reconcile(t)
	f.nodeClass.StatusConditions().SetTrue(v1.ConditionTypeSubnetsReady)
	f.reconcile(t)
	g.Expect(f.recorder.Calls("IncompatibleSubnetSecurityGroups")).To(Equal(3))
	f.resources([]ec2types.Subnet{networkSubnet("subnet-a", "vpc-a")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
	f.reconcile(t)
	f.reconcile(t)
	g.Expect(f.recorder.Calls("NetworkCompatibilityRestored")).To(Equal(1))
	for _, event := range f.recorder.Events() {
		if event.Reason == "IncompatibleSubnetSecurityGroups" {
			g.Expect(event.Type).To(Equal(corev1.EventTypeWarning))
		}
		if event.Reason == "NetworkCompatibilityRestored" {
			g.Expect(event.Type).To(Equal(corev1.EventTypeNormal))
		}
	}
}

func TestNetworkCompatibilitySelectorsRemainPlainFilters(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		subnets                 []v1.SubnetSelectorTerm
		groups                  []v1.SecurityGroupSelectorTerm
		wantSubnets, wantGroups []string
	}{
		{name: "IDs", subnets: []v1.SubnetSelectorTerm{{ID: "subnet-test1"}}, groups: []v1.SecurityGroupSelectorTerm{{ID: "sg-test1"}}, wantSubnets: []string{"subnet-test1"}, wantGroups: []string{"sg-test1"}},
		{name: "tags", subnets: []v1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-1"}}}, groups: []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"Name": "test-security-group-1"}}}, wantSubnets: []string{"subnet-test1"}, wantGroups: []string{"sg-test1"}},
		{name: "SG name", subnets: []v1.SubnetSelectorTerm{{ID: "subnet-test1"}}, groups: []v1.SecurityGroupSelectorTerm{{Name: "securityGroup-test1"}}, wantSubnets: []string{"subnet-test1"}, wantGroups: []string{"sg-test1"}},
		{name: "OR across ID tag and name", subnets: []v1.SubnetSelectorTerm{{ID: "subnet-test1"}, {Tags: map[string]string{"Name": "test-subnet-2"}}}, groups: []v1.SecurityGroupSelectorTerm{{ID: "sg-test1"}, {Tags: map[string]string{"Name": "test-security-group-2"}}, {Name: "securityGroup-test3"}}, wantSubnets: []string{"subnet-test1", "subnet-test2"}, wantGroups: []string{"sg-test1", "sg-test2", "sg-test3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			f := newNetworkFixture(t)
			f.api.DescribeSubnetsBehavior.Output.Reset()
			f.api.DescribeSecurityGroupsBehavior.Output.Reset()
			// Default fake SGs belong to vpc-test1. Supply selected subnets from a different VPC.
			for i := 1; i <= 2; i++ {
				s := networkSubnet(fmt.Sprintf("subnet-test%d", i), "vpc-other")
				s.Tags = []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("test-subnet-%d", i))}}
				f.api.Subnets.Store(aws.ToString(s.SubnetId), s)
			}
			f.nodeClass.Spec.SubnetSelectorTerms, f.nodeClass.Spec.SecurityGroupSelectorTerms = tc.subnets, tc.groups
			_, err := nodeclass.NewSubnetReconciler(f.clk, f.subnets).Reconcile(context.Background(), f.nodeClass)
			g.Expect(err).NotTo(HaveOccurred())
			_, err = nodeclass.NewSecurityGroupReconciler(f.clk, f.groups).Reconcile(context.Background(), f.nodeClass)
			g.Expect(err).NotTo(HaveOccurred())
			before := f.nodeClass.DeepCopy()
			f.reconcile(t)
			g.Expect(f.nodeClass.StatusConditions().Get(v1.ConditionTypeNetworkCompatible).Reason).To(Equal("NoCompatibleSubnet"))
			g.Expect(f.nodeClass.Spec).To(Equal(before.Spec))
			g.Expect(f.nodeClass.Status.Subnets).To(Equal(before.Status.Subnets))
			g.Expect(f.nodeClass.Status.SecurityGroups).To(Equal(before.Status.SecurityGroups))
			var subnetIDs, groupIDs []string
			for _, s := range f.nodeClass.Status.Subnets {
				subnetIDs = append(subnetIDs, s.ID)
			}
			for _, sg := range f.nodeClass.Status.SecurityGroups {
				groupIDs = append(groupIDs, sg.ID)
			}
			g.Expect(subnetIDs).To(ConsistOf(tc.wantSubnets))
			g.Expect(groupIDs).To(ConsistOf(tc.wantGroups))
			f.api.DescribeSubnetsBehavior.CalledWithInput.ForEach(func(input *ec2.DescribeSubnetsInput) {
				for _, filter := range input.Filters {
					g.Expect(aws.ToString(filter.Name)).NotTo(Equal("vpc-id"))
				}
			})
			f.api.DescribeSecurityGroupsBehavior.CalledWithInput.ForEach(func(input *ec2.DescribeSecurityGroupsInput) {
				for _, filter := range input.Filters {
					g.Expect(aws.ToString(filter.Name)).NotTo(Equal("vpc-id"))
				}
			})
		})
	}
}

func TestNetworkCompatibilityWarningRateLimits(t *testing.T) {
	g := NewWithT(t)
	f := newNetworkFixture(t)
	recorder := record.NewFakeRecorder(100)
	f.reconciler = nodeclass.NewNetworkCompatibilityReconciler(f.clk, events.NewRecorder(recorder), f.subnets, f.groups, f.api)
	for i := 0; i < 10; i++ {
		f.resources([]ec2types.Subnet{networkSubnet(fmt.Sprintf("subnet-%02d", i), "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
		f.reconcile(t)
	}
	g.Expect(len(recorder.Events)).To(Equal(5))
	f.clk.Step(time.Minute)
	f.resources([]ec2types.Subnet{networkSubnet("subnet-new", "vpc-b")}, []ec2types.SecurityGroup{networkGroup("sg-a", "vpc-a")})
	f.reconcile(t)
	g.Expect(len(recorder.Events)).To(Equal(6))

	f.clk.Step(awscache.DefaultTTL)
	for i := 0; i < 3; i++ {
		f.api.DescribeSecurityGroupVpcAssociationsBehavior.Error.Set(fmt.Errorf("changing failure %d", i), fake.MaxCalls(0))
		f.reconcile(t)
	}
	g.Expect(len(recorder.Events)).To(Equal(7)) // one failure warning regardless of changing error text
	other := f.nodeClass.DeepCopy()
	other.Name, other.UID = "other", "other-uid"
	_, err := f.reconciler.Reconcile(context.Background(), other)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(len(recorder.Events)).To(Equal(8)) // failure warning is per NodeClass
}
