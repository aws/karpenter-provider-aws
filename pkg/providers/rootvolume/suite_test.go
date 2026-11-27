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

package rootvolume_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/rootvolume"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment

const instanceID = "i-0123456789abcdef0"

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "RootVolumeProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
	rootvolume.TasksTotal.Reset()
})

func storeTask(id, imageID string, state ec2types.ReplaceRootVolumeTaskState, start time.Time) {
	awsEnv.EC2API.ReplaceRootVolumeTasks.Store(id, ec2types.ReplaceRootVolumeTask{
		ReplaceRootVolumeTaskId: lo.ToPtr(id),
		InstanceId:              lo.ToPtr(instanceID),
		ImageId:                 lo.ToPtr(imageID),
		TaskState:               state,
		StartTime:               lo.ToPtr(start.UTC().Format(time.RFC3339)),
	})
}

var _ = Describe("RootVolumeProvider", func() {
	Context("InPlaceUpgrades", func() {
		It("should support AMIs without product codes whose root snapshot is readable", func() {
			upgrades, err := awsEnv.RootVolumeProvider.InPlaceUpgrades(ctx, []amifamily.AMI{{AmiID: "ami-1", RootSnapshotID: "snap-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(upgrades).To(Equal(map[string]*v1.InPlaceUpgrade{"ami-1": {Supported: true}}))
		})
		It("should reject AMIs with product codes", func() {
			upgrades, err := awsEnv.RootVolumeProvider.InPlaceUpgrades(ctx, []amifamily.AMI{{AmiID: "ami-1", RootSnapshotID: "snap-1", HasProductCodes: true}})
			Expect(err).ToNot(HaveOccurred())
			Expect(upgrades).To(Equal(map[string]*v1.InPlaceUpgrade{"ami-1": {Reason: v1.InPlaceUpgradeReasonProductCodes}}))
		})
		It("should reject AMIs whose root snapshot isn't readable", func() {
			awsEnv.EC2API.DescribeSnapshotsBehavior.Output.Set(&ec2.DescribeSnapshotsOutput{
				Snapshots: []ec2types.Snapshot{{SnapshotId: lo.ToPtr("snap-1")}},
			})
			upgrades, err := awsEnv.RootVolumeProvider.InPlaceUpgrades(ctx, []amifamily.AMI{
				{AmiID: "ami-1", RootSnapshotID: "snap-1"},
				{AmiID: "ami-2", RootSnapshotID: "snap-2"},
				{AmiID: "ami-3"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(upgrades).To(Equal(map[string]*v1.InPlaceUpgrade{
				"ami-1": {Supported: true},
				"ami-2": {Reason: v1.InPlaceUpgradeReasonSnapshotNotReadable},
				"ami-3": {Reason: v1.InPlaceUpgradeReasonSnapshotNotReadable},
			}))
		})
		It("should describe each root snapshot once", func() {
			_, err := awsEnv.RootVolumeProvider.InPlaceUpgrades(ctx, []amifamily.AMI{
				{AmiID: "ami-1", RootSnapshotID: "snap-1"},
				{AmiID: "ami-1", RootSnapshotID: "snap-1"},
				{AmiID: "ami-2", RootSnapshotID: "snap-2"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(awsEnv.EC2API.DescribeSnapshotsBehavior.CalledWithInput.Len()).To(Equal(1))
			input := awsEnv.EC2API.DescribeSnapshotsBehavior.CalledWithInput.Pop()
			Expect(input.SnapshotIds).To(BeEmpty())
			Expect(input.Filters).To(HaveLen(1))
			Expect(lo.FromPtr(input.Filters[0].Name)).To(Equal("snapshot-id"))
			Expect(input.Filters[0].Values).To(ConsistOf("snap-1", "snap-2"))
		})
		It("should return an error when snapshots can't be described", func() {
			awsEnv.EC2API.DescribeSnapshotsBehavior.Error.Set(&smithy.GenericAPIError{Code: "UnauthorizedOperation"})
			_, err := awsEnv.RootVolumeProvider.InPlaceUpgrades(ctx, []amifamily.AMI{{AmiID: "ami-1", RootSnapshotID: "snap-1"}})
			Expect(err).To(HaveOccurred())
		})
	})
	Context("UnsupportedNodeReason", func() {
		var node *corev1.Node
		var inst *instance.Instance
		var ami amifamily.AMI
		BeforeEach(func() {
			node = coretest.Node()
			node.Status.NodeInfo.KubeletVersion = fmt.Sprintf("v%s.4-eks-1234", awsEnv.VersionProvider.Get(ctx))
			inst = &instance.Instance{
				ID:                      instanceID,
				State:                   ec2types.InstanceStateNameRunning,
				Type:                    "m5.large",
				EnaSupport:              true,
				CurrentInstanceBootMode: ec2types.InstanceBootModeValuesLegacyBios,
			}
			ami = amifamily.AMI{AmiID: "ami-1", EnaSupport: true}
		})
		It("should pass a running node on the cluster's Kubernetes minor version", func() {
			Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(BeEmpty())
		})
		It("should reject a missing node", func() {
			Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, nil, inst, ami)).To(Equal(rootvolume.ReasonNodeNotFound))
		})
		It("should reject an instance that isn't running", func() {
			inst.State = ec2types.InstanceStateNameStopping
			Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(Equal(rootvolume.ReasonInstanceNotRunning))
		})
		It("should reject a node on another Kubernetes minor version", func() {
			node.Status.NodeInfo.KubeletVersion = "v1.20.4-eks-1234"
			Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(Equal(rootvolume.ReasonKubernetesVersionChanged))
		})
		DescribeTable("should check the instance type isn't bare metal",
			func(instanceType ec2types.InstanceType, reason string) {
				inst.Type = instanceType
				Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(Equal(reason))
			},
			Entry("virtualized", ec2types.InstanceType("m5.large"), ""),
			Entry("bare metal", ec2types.InstanceType("m5.metal"), rootvolume.ReasonBareMetal),
			Entry("bare metal with a size suffix", ec2types.InstanceType("c7i.metal-24xl"), rootvolume.ReasonBareMetal),
		)
		DescribeTable("should check the AMI supports ENA when the instance has it enabled",
			func(instanceEnaSupport, amiEnaSupport bool, reason string) {
				inst.EnaSupport = instanceEnaSupport
				ami.EnaSupport = amiEnaSupport
				Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(Equal(reason))
			},
			Entry("both support it", true, true, ""),
			Entry("only the AMI supports it", false, true, ""),
			Entry("only the instance has it enabled", true, false, rootvolume.ReasonEnaSupport),
		)
		DescribeTable("should check the AMI supports sriov-net when the instance has it enabled",
			func(instanceSriovNetSupport, amiSriovNetSupport bool, reason string) {
				inst.SriovNetSupport = instanceSriovNetSupport
				ami.SriovNetSupport = amiSriovNetSupport
				Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(Equal(reason))
			},
			Entry("both support it", true, true, ""),
			Entry("only the AMI supports it", false, true, ""),
			Entry("only the instance has it enabled", true, false, rootvolume.ReasonSriovNetSupport),
		)
		DescribeTable("should check the AMI supports NitroTPM when the instance has it enabled",
			func(instanceTpmSupport, amiTpmSupport bool, reason string) {
				inst.TpmSupport = instanceTpmSupport
				ami.TpmSupport = amiTpmSupport
				Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(Equal(reason))
			},
			Entry("both support it", true, true, ""),
			Entry("only the AMI supports it", false, true, ""),
			Entry("only the instance has it enabled", true, false, rootvolume.ReasonTpmSupport),
		)
		DescribeTable("should check the AMI boot mode matches the instance boot mode",
			func(instanceBootMode ec2types.InstanceBootModeValues, amiBootMode ec2types.BootModeValues, reason string) {
				inst.CurrentInstanceBootMode = instanceBootMode
				ami.BootMode = amiBootMode
				Expect(awsEnv.RootVolumeProvider.UnsupportedNodeReason(ctx, node, inst, ami)).To(Equal(reason))
			},
			Entry("both uefi", ec2types.InstanceBootModeValuesUefi, ec2types.BootModeValuesUefi, ""),
			Entry("both legacy-bios", ec2types.InstanceBootModeValuesLegacyBios, ec2types.BootModeValuesLegacyBios, ""),
			Entry("uefi-preferred AMI", ec2types.InstanceBootModeValuesLegacyBios, ec2types.BootModeValuesUefiPreferred, ""),
			Entry("AMI without a boot mode", ec2types.InstanceBootModeValuesUefi, ec2types.BootModeValues(""), ""),
			Entry("uefi AMI on a legacy-bios instance", ec2types.InstanceBootModeValuesLegacyBios, ec2types.BootModeValuesUefi, rootvolume.ReasonBootMode),
			Entry("legacy-bios AMI on a uefi instance", ec2types.InstanceBootModeValuesUefi, ec2types.BootModeValuesLegacyBios, rootvolume.ReasonBootMode),
		)
	})
	Context("Replace", func() {
		It("should create a task that restores the root volume from the AMI and deletes the replaced volume", func() {
			task, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(task.ImageID).To(Equal("ami-new"))
			Expect(task.Finished()).To(BeFalse())
			input := awsEnv.EC2API.CreateReplaceRootVolumeTaskBehavior.CalledWithInput.Pop()
			Expect(lo.FromPtr(input.InstanceId)).To(Equal(instanceID))
			Expect(lo.FromPtr(input.ImageId)).To(Equal("ami-new"))
			Expect(lo.FromPtr(input.DeleteReplacedRootVolume)).To(BeTrue())
		})
		It("should pass the SHA-256 of the operation ID as the client token", func() {
			operationID := "4a1b6d2e-7c3f-4e5a-9b8c-0d1e2f3a4b5c-1763596800000000000-0c8f6a2b-5d4e-4f3a-8b7c-6d5e4f3a2b1c"
			_, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", operationID)
			Expect(err).ToNot(HaveOccurred())
			sum := sha256.Sum256([]byte(operationID))
			token := lo.FromPtr(awsEnv.EC2API.CreateReplaceRootVolumeTaskBehavior.CalledWithInput.Pop().ClientToken)
			Expect(token).To(Equal(hex.EncodeToString(sum[:])))
			Expect(token).To(HaveLen(64))
		})
		DescribeTable("should wait for an unfinished task instead of creating another",
			func(state ec2types.ReplaceRootVolumeTaskState) {
				storeTask("replacevol-running", "ami-old", state, time.Now())
				task, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
				Expect(err).ToNot(HaveOccurred())
				Expect(task.ID).To(Equal("replacevol-running"))
				Expect(awsEnv.EC2API.CreateReplaceRootVolumeTaskBehavior.Calls()).To(Equal(0))
			},
			Entry("pending", ec2types.ReplaceRootVolumeTaskStatePending),
			Entry("in-progress", ec2types.ReplaceRootVolumeTaskStateInProgress),
			Entry("failing", ec2types.ReplaceRootVolumeTaskStateFailing),
		)
		It("should create a task when the latest task finished", func() {
			storeTask("replacevol-done", "ami-old", ec2types.ReplaceRootVolumeTaskStateSucceeded, time.Now())
			task, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(task.ID).ToNot(Equal("replacevol-done"))
			Expect(awsEnv.EC2API.CreateReplaceRootVolumeTaskBehavior.Calls()).To(Equal(1))
		})
		It("should fail the reboot when EC2 rejects the task", func() {
			awsEnv.EC2API.CreateReplaceRootVolumeTaskBehavior.Error.Set(&smithy.GenericAPIError{Code: "IncorrectState"})
			_, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
			Expect(cloudprovider.IsNodeRebootFailedError(err)).To(BeTrue())
		})
		It("should return a NodeClaimNotFoundError when the instance is gone", func() {
			awsEnv.EC2API.CreateReplaceRootVolumeTaskBehavior.Error.Set(&smithy.GenericAPIError{Code: "InvalidInstanceID.NotFound"})
			_, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
			Expect(cloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
		})
		It("should return a retryable error when EC2 throttles the call", func() {
			awsEnv.EC2API.CreateReplaceRootVolumeTaskBehavior.Error.Set(&smithy.GenericAPIError{Code: "RequestLimitExceeded"})
			_, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
			Expect(err).To(HaveOccurred())
			Expect(cloudprovider.IsNodeRebootFailedError(err)).To(BeFalse())
		})
	})
	Context("LatestTask", func() {
		It("should return nil when the instance has no task", func() {
			task, err := awsEnv.RootVolumeProvider.LatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(task).To(BeNil())
		})
		It("should return the most recently started task", func() {
			storeTask("replacevol-old", "ami-old", ec2types.ReplaceRootVolumeTaskStateFailed, time.Now().Add(-time.Hour))
			storeTask("replacevol-new", "ami-new", ec2types.ReplaceRootVolumeTaskStateSucceeded, time.Now())
			task, err := awsEnv.RootVolumeProvider.LatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(task.ID).To(Equal("replacevol-new"))
			Expect(task.Succeeded()).To(BeTrue())
		})
		It("should read an unfinished task from the cache", func() {
			_, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
			Expect(err).ToNot(HaveOccurred())
			describeCalls := awsEnv.EC2API.DescribeReplaceRootVolumeTasksBehavior.Calls()
			task, err := awsEnv.RootVolumeProvider.LatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(task.Finished()).To(BeFalse())
			Expect(awsEnv.EC2API.DescribeReplaceRootVolumeTasksBehavior.Calls()).To(Equal(describeCalls))
		})
	})
	Context("SyncLatestTask", func() {
		It("should read a finished task from the cache", func() {
			storeTask("replacevol-done", "ami-new", ec2types.ReplaceRootVolumeTaskStateSucceeded, time.Now())
			_, err := awsEnv.RootVolumeProvider.SyncLatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			_, err = awsEnv.RootVolumeProvider.SyncLatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(awsEnv.EC2API.DescribeReplaceRootVolumeTasksBehavior.Calls()).To(Equal(1))
		})
		It("should describe an unfinished task again and count it once it finishes", func() {
			task, err := awsEnv.RootVolumeProvider.Replace(ctx, instanceID, "ami-new", "op-1")
			Expect(err).ToNot(HaveOccurred())
			storeTask(task.ID, "ami-new", ec2types.ReplaceRootVolumeTaskStateInProgress, task.StartTime)
			task, err = awsEnv.RootVolumeProvider.SyncLatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(task.State).To(Equal(ec2types.ReplaceRootVolumeTaskStateInProgress))

			storeTask(task.ID, "ami-new", ec2types.ReplaceRootVolumeTaskStateFailedDetached, task.StartTime)
			task, err = awsEnv.RootVolumeProvider.SyncLatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(task.Failed()).To(BeTrue())
			ExpectMetricCounterValue(rootvolume.TasksTotal, 1, map[string]string{"state": "failed-detached"})

			_, err = awsEnv.RootVolumeProvider.SyncLatestTask(ctx, instanceID)
			Expect(err).ToNot(HaveOccurred())
			ExpectMetricCounterValue(rootvolume.TasksTotal, 1, map[string]string{"state": "failed-detached"})
		})
	})
})
