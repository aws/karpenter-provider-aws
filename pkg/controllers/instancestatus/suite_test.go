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

package instancestatus_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/instancestatus"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	instancestatusprovider "github.com/aws/karpenter-provider-aws/pkg/providers/instancestatus"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

// forbiddenStatusClient rejects every status patch as if Karpenter lacked the nodes/status permission.
type forbiddenStatusClient struct {
	client.Client
	patches *atomic.Int32
}

func (f forbiddenStatusClient) Status() client.SubResourceWriter {
	return forbiddenStatusWriter{SubResourceWriter: f.Client.Status(), patches: f.patches}
}

type forbiddenStatusWriter struct {
	client.SubResourceWriter
	patches *atomic.Int32
}

func (f forbiddenStatusWriter) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
	f.patches.Add(1)
	return apierrors.NewForbidden(schema.GroupResource{Resource: "nodes/status"}, obj.GetName(), fmt.Errorf("forbidden"))
}

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var controller *instancestatus.Controller
var cloudProvider *cloudprovider.CloudProvider

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceStatus")
}

var _ = BeforeSuite(func() {
	ctx = options.ToContext(ctx, test.Options())
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...), coretest.WithFieldIndexers(coretest.NodeProviderIDFieldIndexer(ctx)))
	awsEnv = test.NewEnvironment(ctx, env)
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider, awsEnv.PlacementGroupProvider, awsEnv.InstanceTypeStore, lo.ToPtr(""))
	controller = instancestatus.NewController(env.Client, awsEnv.Clock, cloudProvider, awsEnv.InstanceStatusProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv.Reset()
	awsEnv.Clock.SetTime(time.Date(2026, time.October, 12, 12, 0, 0, 0, time.UTC))
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InstanceStatus", func() {
	var nodeClaim *karpv1.NodeClaim
	var node *corev1.Node
	var instanceID string
	BeforeEach(func() {
		nodeClaim, node = coretest.NodeClaimAndNode(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{Group: apis.Group, Kind: "EC2NodeClass", Name: "default"},
			},
			Status: karpv1.NodeClaimStatus{ProviderID: fake.RandomProviderID()},
		})
		nodeClaim.StatusConditions().SetTrue(karpv1.ConditionTypeRegistered)
		node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady"}}
		instanceID = lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))
	})
	impair := func(instanceStatus, systemStatus *time.Time) {
		summary := func(impairedSince *time.Time) *ec2types.InstanceStatusSummary {
			if impairedSince == nil {
				return &ec2types.InstanceStatusSummary{
					Status:  ec2types.SummaryStatusOk,
					Details: []ec2types.InstanceStatusDetails{{Name: ec2types.StatusNameReachability, Status: ec2types.StatusTypePassed}},
				}
			}
			return &ec2types.InstanceStatusSummary{
				Status:  ec2types.SummaryStatusImpaired,
				Details: []ec2types.InstanceStatusDetails{{Name: ec2types.StatusNameReachability, Status: ec2types.StatusTypeFailed, ImpairedSince: impairedSince}},
			}
		}
		awsEnv.EC2API.DescribeInstanceStatusOutput.Set(&ec2.DescribeInstanceStatusOutput{
			InstanceStatuses: []ec2types.InstanceStatus{{
				InstanceId:     lo.ToPtr(instanceID),
				InstanceStatus: summary(instanceStatus),
				SystemStatus:   summary(systemStatus),
			}},
		})
	}
	expectCondition := func(status corev1.ConditionStatus, reason string, lastTransitionTime time.Time) {
		GinkgoHelper()
		condition := nodeutils.GetCondition(ExpectExists(ctx, env.Client, node), instancestatusprovider.ConditionTypeEC2StatusReady)
		Expect(condition.Status).To(Equal(status))
		Expect(condition.Reason).To(Equal(reason))
		Expect(condition.LastTransitionTime.Time).To(BeTemporally("==", lastTransitionTime))
	}

	DescribeTable("should set EC2StatusReady=False from the time EC2 reports a failed reachability check",
		func(instanceStatus, systemStatus *time.Time, reason string, lastTransitionTime time.Time, message string) {
			impair(instanceStatus, systemStatus)
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectSingletonReconciled(ctx, controller)
			expectCondition(corev1.ConditionFalse, reason, lastTransitionTime)
			Expect(nodeutils.GetCondition(ExpectExists(ctx, env.Client, node), instancestatusprovider.ConditionTypeEC2StatusReady).Message).To(Equal(message))
		},
		Entry("instance status", lo.ToPtr(time.Date(2026, time.October, 12, 11, 59, 0, 0, time.UTC)), nil, instancestatusprovider.ReasonInstanceReachabilityFailed,
			time.Date(2026, time.October, 12, 11, 59, 0, 0, time.UTC), "EC2 reports failing reachability checks: InstanceStatus"),
		Entry("system status", nil, lo.ToPtr(time.Date(2026, time.October, 12, 11, 58, 0, 0, time.UTC)), instancestatusprovider.ReasonSystemReachabilityFailed,
			time.Date(2026, time.October, 12, 11, 58, 0, 0, time.UTC), "EC2 reports failing reachability checks: SystemStatus"),
		Entry("both, using the system reason and the earliest impairment", lo.ToPtr(time.Date(2026, time.October, 12, 11, 59, 0, 0, time.UTC)), lo.ToPtr(time.Date(2026, time.October, 12, 11, 58, 0, 0, time.UTC)), instancestatusprovider.ReasonSystemReachabilityFailed,
			time.Date(2026, time.October, 12, 11, 58, 0, 0, time.UTC), "EC2 reports failing reachability checks: InstanceStatus, SystemStatus"),
	)
	It("should use the observation time when EC2 does not report when the impairment began", func() {
		awsEnv.EC2API.DescribeInstanceStatusOutput.Set(&ec2.DescribeInstanceStatusOutput{
			InstanceStatuses: []ec2types.InstanceStatus{{
				InstanceId: lo.ToPtr(instanceID),
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status:  ec2types.SummaryStatusImpaired,
					Details: []ec2types.InstanceStatusDetails{{Name: ec2types.StatusNameReachability, Status: ec2types.StatusTypeFailed}},
				},
			}},
		})
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectSingletonReconciled(ctx, controller)
		expectCondition(corev1.ConditionFalse, instancestatusprovider.ReasonInstanceReachabilityFailed, awsEnv.Clock.Now())
	})
	It("should keep the transition time when the failing check changes and set EC2StatusReady=True when the impairment clears", func() {
		impairedSince := awsEnv.Clock.Now().Add(-time.Minute)
		impair(&impairedSince, nil)
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectSingletonReconciled(ctx, controller)
		resourceVersion := ExpectExists(ctx, env.Client, node).ResourceVersion

		awsEnv.Clock.Step(time.Minute)
		impair(nil, lo.ToPtr(impairedSince.Add(30*time.Second)))
		ExpectSingletonReconciled(ctx, controller)
		expectCondition(corev1.ConditionFalse, instancestatusprovider.ReasonSystemReachabilityFailed, impairedSince)
		Expect(nodeutils.GetCondition(ExpectExists(ctx, env.Client, node), instancestatusprovider.ConditionTypeEC2StatusReady).Message).To(Equal("EC2 reports failing reachability checks: SystemStatus"))

		awsEnv.Clock.Step(time.Minute)
		impair(nil, nil)
		ExpectSingletonReconciled(ctx, controller)
		expectCondition(corev1.ConditionTrue, instancestatusprovider.ReasonNoImpairmentReported, awsEnv.Clock.Now())
		Expect(ExpectExists(ctx, env.Client, node).ResourceVersion).ToNot(Equal(resourceVersion))
	})
	It("should set EC2StatusReady=True when EC2 reports no impairment and not patch the Node again while unchanged", func() {
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectSingletonReconciled(ctx, controller)
		expectCondition(corev1.ConditionTrue, instancestatusprovider.ReasonNoImpairmentReported, awsEnv.Clock.Now())
		Expect(nodeutils.GetCondition(ExpectExists(ctx, env.Client, node), corev1.NodeReady).Status).To(Equal(corev1.ConditionTrue))
		resourceVersion := ExpectExists(ctx, env.Client, node).ResourceVersion
		awsEnv.Clock.Step(time.Minute)
		ExpectSingletonReconciled(ctx, controller)
		Expect(ExpectExists(ctx, env.Client, node).ResourceVersion).To(Equal(resourceVersion))
	})
	It("should leave the condition unchanged when DescribeInstanceStatus fails", func() {
		impairedSince := awsEnv.Clock.Now()
		impair(&impairedSince, nil)
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectSingletonReconciled(ctx, controller)

		impair(nil, nil)
		awsEnv.EC2API.NextError.Set(fmt.Errorf("throttled"))
		_ = ExpectSingletonReconcileFailed(ctx, controller)
		expectCondition(corev1.ConditionFalse, instancestatusprovider.ReasonInstanceReachabilityFailed, impairedSince)
	})
	It("should not publish health for a Node whose NodeClaim is rebooting", func() {
		impair(lo.ToPtr(awsEnv.Clock.Now().Add(-5*time.Minute)), nil)
		nodeClaim.StatusConditions().SetTrueWithReason(karpv1.ConditionTypeRebooting, karpv1.RebootReasonIssued, "")
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectSingletonReconciled(ctx, controller)
		Expect(nodeutils.GetCondition(ExpectExists(ctx, env.Client, node), instancestatusprovider.ConditionTypeEC2StatusReady).Type).To(BeEmpty())
	})
	It("should stop patching Nodes once a status patch is forbidden", func() {
		for range 11 {
			nc, n := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				Spec:   karpv1.NodeClaimSpec{NodeClassRef: nodeClaim.Spec.NodeClassRef},
				Status: karpv1.NodeClaimStatus{ProviderID: fake.RandomProviderID()},
			})
			nc.StatusConditions().SetTrue(karpv1.ConditionTypeRegistered)
			ExpectApplied(ctx, env.Client, nc, n)
		}
		patches := &atomic.Int32{}
		forbidden := instancestatus.NewController(forbiddenStatusClient{Client: env.Client, patches: patches}, awsEnv.Clock, cloudProvider, awsEnv.InstanceStatusProvider)
		Expect(ExpectSingletonReconcileFailed(ctx, forbidden)).To(MatchError(ContainSubstring("forbidden")))
		Expect(patches.Load()).To(BeNumerically("<=", 10))
	})
	It("should not publish health for a Node without a registered NodeClaim", func() {
		nodeClaim.StatusConditions().SetUnknown(karpv1.ConditionTypeRegistered)
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectSingletonReconciled(ctx, controller)
		Expect(nodeutils.GetCondition(ExpectExists(ctx, env.Client, node), instancestatusprovider.ConditionTypeEC2StatusReady).Type).To(BeEmpty())
	})
})
