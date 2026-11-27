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

package imageid_test

import (
	"context"
	"testing"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclaim/imageid"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var awsEnv *test.Environment
var env *coretest.Environment
var controller *imageid.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "ImageIDController")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
	cloudProvider := cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider, awsEnv.PlacementGroupProvider, awsEnv.RootVolumeProvider, awsEnv.InstanceTypeStore, lo.ToPtr(""))
	controller = imageid.NewController(env.Client, cloudProvider, awsEnv.RootVolumeProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("ImageIDController", func() {
	var instanceID string
	var nodeClaim *karpv1.NodeClaim

	storeTask := func(state ec2types.ReplaceRootVolumeTaskState) {
		awsEnv.EC2API.ReplaceRootVolumeTasks.Store("replacevol-1", ec2types.ReplaceRootVolumeTask{
			ReplaceRootVolumeTaskId: lo.ToPtr("replacevol-1"),
			InstanceId:              lo.ToPtr(instanceID),
			ImageId:                 lo.ToPtr("ami-new"),
			TaskState:               state,
			StartTime:               lo.ToPtr(time.Now().UTC().Format(time.RFC3339)),
		})
	}

	BeforeEach(func() {
		instanceID = fake.InstanceID()
		nodeClaim = coretest.NodeClaim(karpv1.NodeClaim{
			Status: karpv1.NodeClaimStatus{
				ProviderID: fake.ProviderID(instanceID),
				ImageID:    "ami-old",
			},
		})
		nodeClaim.StatusConditions().SetTrueWithReason(karpv1.ConditionTypeRebooting, karpv1.RebootReasonIssued, "")
	})

	It("should update the image ID when the root volume replacement task succeeds", func() {
		storeTask(ec2types.ReplaceRootVolumeTaskStateSucceeded)
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClaim)
		Expect(ExpectExists(ctx, env.Client, nodeClaim).Status.ImageID).To(Equal("ami-new"))
	})
	It("should update the image ID after the reboot finished", func() {
		storeTask(ec2types.ReplaceRootVolumeTaskStateSucceeded)
		nodeClaim.StatusConditions().SetFalse(karpv1.ConditionTypeRebooting, karpv1.RebootReasonSucceeded, "")
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClaim)
		Expect(ExpectExists(ctx, env.Client, nodeClaim).Status.ImageID).To(Equal("ami-new"))
	})
	It("should wait for an unfinished task", func() {
		storeTask(ec2types.ReplaceRootVolumeTaskStateInProgress)
		ExpectApplied(ctx, env.Client, nodeClaim)
		result := ExpectObjectReconciled(ctx, env.Client, controller, nodeClaim)
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		Expect(ExpectExists(ctx, env.Client, nodeClaim).Status.ImageID).To(Equal("ami-old"))

		storeTask(ec2types.ReplaceRootVolumeTaskStateSucceeded)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClaim)
		Expect(ExpectExists(ctx, env.Client, nodeClaim).Status.ImageID).To(Equal("ami-new"))
	})
	DescribeTable("should keep the image ID when the task failed",
		func(state ec2types.ReplaceRootVolumeTaskState) {
			storeTask(state)
			ExpectApplied(ctx, env.Client, nodeClaim)
			result := ExpectObjectReconciled(ctx, env.Client, controller, nodeClaim)
			Expect(result.RequeueAfter).To(BeZero())
			Expect(ExpectExists(ctx, env.Client, nodeClaim).Status.ImageID).To(Equal("ami-old"))
		},
		Entry("failed", ec2types.ReplaceRootVolumeTaskStateFailed),
		Entry("failed-detached", ec2types.ReplaceRootVolumeTaskStateFailedDetached),
	)
	It("should keep the image ID when the instance has no task", func() {
		ExpectApplied(ctx, env.Client, nodeClaim)
		result := ExpectObjectReconciled(ctx, env.Client, controller, nodeClaim)
		Expect(result.RequeueAfter).To(BeZero())
		Expect(ExpectExists(ctx, env.Client, nodeClaim).Status.ImageID).To(Equal("ami-old"))
	})
	It("should not look up tasks for a NodeClaim that never rebooted", func() {
		storeTask(ec2types.ReplaceRootVolumeTaskStateSucceeded)
		nodeClaim.Status.Conditions = nil
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClaim)
		Expect(awsEnv.EC2API.DescribeReplaceRootVolumeTasksBehavior.Calls()).To(Equal(0))
		Expect(ExpectExists(ctx, env.Client, nodeClaim).Status.ImageID).To(Equal("ami-old"))
	})
})
