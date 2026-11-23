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

package rootvolume

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sversion "k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
)

// Node-level reasons a node's root volume can't be replaced in place. They are reported in events, not in status.
const (
	ReasonNodeNotFound             = "NodeNotFound"
	ReasonInstanceNotRunning       = "InstanceNotRunning"
	ReasonBareMetal                = "BareMetal"
	ReasonKubernetesVersionChanged = "KubernetesVersionChanged"
	ReasonEnaSupport               = "EnaSupport"
	ReasonSriovNetSupport          = "SriovNetSupport"
	ReasonTpmSupport               = "TpmSupport"
	ReasonBootMode                 = "BootMode"
	ReasonTaskFailed               = "RootVolumeReplacementFailed"
)

// retryableErrorCodes are CreateReplaceRootVolumeTask errors that don't reject the task, so the call is retried.
var retryableErrorCodes = sets.New(
	awserrors.RateLimitingErrorCode,
	"InternalError",
	"Unavailable",
	"ServiceUnavailable",
)

type Task struct {
	ID         string
	InstanceID string
	ImageID    string
	State      ec2types.ReplaceRootVolumeTaskState
	StartTime  time.Time
}

func (t *Task) Finished() bool {
	return t.Succeeded() || t.Failed()
}

func (t *Task) Succeeded() bool {
	return t.State == ec2types.ReplaceRootVolumeTaskStateSucceeded
}

func (t *Task) Failed() bool {
	return t.State == ec2types.ReplaceRootVolumeTaskStateFailed || t.State == ec2types.ReplaceRootVolumeTaskStateFailedDetached
}

type Provider interface {
	// InPlaceUpgrades returns the result of the AMI-level checks for each AMI, keyed by AMI ID.
	InPlaceUpgrades(context.Context, []amifamily.AMI) (map[string]*v1.InPlaceUpgrade, error)
	// UnsupportedNodeReason returns the node-level check the node fails, or "" when it passes every check.
	UnsupportedNodeReason(context.Context, *corev1.Node, *instance.Instance, amifamily.AMI) string
	// Replace starts a task that replaces the instance's root volume with one restored from the AMI, or returns the
	// instance's unfinished task so the caller waits for it instead.
	Replace(ctx context.Context, instanceID, imageID, operationID string) (*Task, error)
	// LatestTask returns the instance's most recent task, or nil when it has none. It only calls EC2 when the instance's
	// tasks aren't cached, so it is cheap enough for every drift check.
	LatestTask(ctx context.Context, instanceID string) (*Task, error)
	// SyncLatestTask is LatestTask, but it also describes the instance's tasks again while the cached task is unfinished.
	SyncLatestTask(ctx context.Context, instanceID string) (*Task, error)
}

type DefaultProvider struct {
	ec2api          sdk.EC2API
	versionProvider version.Provider
	// tasks holds the latest task per instance ID, or a nil *Task when the instance has none.
	tasks *cache.Cache
}

func NewDefaultProvider(ec2api sdk.EC2API, versionProvider version.Provider, tasks *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api:          ec2api,
		versionProvider: versionProvider,
		tasks:           tasks,
	}
}

func (p *DefaultProvider) InPlaceUpgrades(ctx context.Context, amis []amifamily.AMI) (map[string]*v1.InPlaceUpgrade, error) {
	snapshotIDs := lo.Uniq(lo.FilterMap(amis, func(ami amifamily.AMI, _ int) (string, bool) {
		return ami.RootSnapshotID, ami.RootSnapshotID != ""
	}))
	readable := sets.New[string]()
	if len(snapshotIDs) > 0 {
		// A snapshot-id filter leaves out snapshots the account can't see. Passing SnapshotIds instead fails the whole
		// call with InvalidSnapshot.NotFound when any one of them isn't shared with the account.
		input := &ec2.DescribeSnapshotsInput{Filters: []ec2types.Filter{{Name: lo.ToPtr("snapshot-id"), Values: snapshotIDs}}}
		for {
			out, err := p.ec2api.DescribeSnapshots(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("describing snapshots, %w", err)
			}
			for _, snapshot := range out.Snapshots {
				readable.Insert(lo.FromPtr(snapshot.SnapshotId))
			}
			if lo.FromPtr(out.NextToken) == "" {
				break
			}
			input.NextToken = out.NextToken
		}
	}
	return lo.SliceToMap(amis, func(ami amifamily.AMI) (string, *v1.InPlaceUpgrade) {
		switch {
		case ami.HasProductCodes:
			return ami.AmiID, &v1.InPlaceUpgrade{Reason: v1.InPlaceUpgradeReasonProductCodes}
		case !readable.Has(ami.RootSnapshotID):
			return ami.AmiID, &v1.InPlaceUpgrade{Reason: v1.InPlaceUpgradeReasonSnapshotNotReadable}
		default:
			return ami.AmiID, &v1.InPlaceUpgrade{Supported: true}
		}
	}), nil
}

// UnsupportedNodeReason checks what the AMI-level checks can't: the instance must be running and not bare metal, its
// kubelet must match the Kubernetes minor version the EC2NodeClass resolves AMIs for, and the AMI must support the
// instance's ENA, sriov-net, NitroTPM and boot mode settings. The AMI family and variant need no check here: a family
// change drifts the EC2NodeClass hash, and the AMI for the node's instance type keeps its variant.
func (p *DefaultProvider) UnsupportedNodeReason(ctx context.Context, node *corev1.Node, instance *instance.Instance, ami amifamily.AMI) string {
	if node == nil {
		return ReasonNodeNotFound
	}
	if instance.State != ec2types.InstanceStateNameRunning {
		return ReasonInstanceNotRunning
	}
	if strings.Contains(string(instance.Type), ".metal") {
		return ReasonBareMetal
	}
	kubeletVersion, err := k8sversion.ParseGeneric(node.Status.NodeInfo.KubeletVersion)
	if err != nil {
		return ReasonKubernetesVersionChanged
	}
	clusterVersion, err := k8sversion.ParseGeneric(p.versionProvider.Get(ctx))
	if err != nil || kubeletVersion.Major() != clusterVersion.Major() || kubeletVersion.Minor() != clusterVersion.Minor() {
		return ReasonKubernetesVersionChanged
	}
	return unsupportedAMIReason(instance, ami)
}

// unsupportedAMIReason returns the instance setting the AMI doesn't support, or "" when it supports all of them.
func unsupportedAMIReason(instance *instance.Instance, ami amifamily.AMI) string {
	switch {
	case instance.EnaSupport && !ami.EnaSupport:
		return ReasonEnaSupport
	case instance.SriovNetSupport && !ami.SriovNetSupport:
		return ReasonSriovNetSupport
	case instance.TpmSupport && !ami.TpmSupport:
		return ReasonTpmSupport
	case (ami.BootMode == ec2types.BootModeValuesUefi || ami.BootMode == ec2types.BootModeValuesLegacyBios) &&
		string(ami.BootMode) != string(instance.CurrentInstanceBootMode):
		return ReasonBootMode
	}
	return ""
}

func (p *DefaultProvider) Replace(ctx context.Context, instanceID, imageID, operationID string) (*Task, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("image-id", imageID))
	// The ClientToken alone can't find a running task: EC2 doesn't always reject a reused token whose AMI changed.
	task, err := p.describe(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if task != nil && !task.Finished() {
		log.FromContext(ctx).WithValues("task-id", task.ID).V(1).Info("waiting for running root volume replacement task")
		return task, nil
	}
	out, err := p.ec2api.CreateReplaceRootVolumeTask(ctx, &ec2.CreateReplaceRootVolumeTaskInput{
		InstanceId:               aws.String(instanceID),
		ImageId:                  aws.String(imageID),
		DeleteReplacedRootVolume: aws.Bool(true),
		ClientToken:              aws.String(clientToken(operationID)),
	})
	if err != nil {
		if awserrors.IsNotFound(err) {
			return nil, cloudprovider.NewNodeClaimNotFoundError(fmt.Errorf("creating root volume replacement task, %w", err))
		}
		if apiErr, ok := lo.ErrorsAs[smithy.APIError](err); ok && apiErr.ErrorFault() != smithy.FaultServer && !retryableErrorCodes.Has(apiErr.ErrorCode()) {
			return nil, cloudprovider.NewNodeRebootFailedError(fmt.Errorf("creating root volume replacement task, %w", err))
		}
		return nil, fmt.Errorf("creating root volume replacement task, %w", err)
	}
	task = newTask(out.ReplaceRootVolumeTask)
	p.tasks.SetDefault(instanceID, task)
	log.FromContext(ctx).WithValues("task-id", task.ID).Info("created root volume replacement task")
	return task, nil
}

func (p *DefaultProvider) LatestTask(ctx context.Context, instanceID string) (*Task, error) {
	if cached, ok := p.tasks.Get(instanceID); ok {
		return cached.(*Task), nil
	}
	return p.describe(ctx, instanceID)
}

// SyncLatestTask reads a finished task from the cache, since it can't change, and describes the instance's tasks
// otherwise.
func (p *DefaultProvider) SyncLatestTask(ctx context.Context, instanceID string) (*Task, error) {
	if cached, ok := p.tasks.Get(instanceID); ok {
		if task := cached.(*Task); task == nil || task.Finished() {
			return task, nil
		}
	}
	return p.describe(ctx, instanceID)
}

func (p *DefaultProvider) describe(ctx context.Context, instanceID string) (*Task, error) {
	var latest *Task
	pager := ec2.NewDescribeReplaceRootVolumeTasksPaginator(p.ec2api, &ec2.DescribeReplaceRootVolumeTasksInput{
		Filters: []ec2types.Filter{{Name: aws.String("instance-id"), Values: []string{instanceID}}},
	})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing root volume replacement tasks, %w", err)
		}
		for i := range out.ReplaceRootVolumeTasks {
			if task := newTask(&out.ReplaceRootVolumeTasks[i]); latest == nil || task.StartTime.After(latest.StartTime) {
				latest = task
			}
		}
	}
	p.recordFinished(instanceID, latest)
	p.tasks.SetDefault(instanceID, latest)
	return latest, nil
}

// recordFinished counts a task once, when it is first seen finished after being seen unfinished.
func (p *DefaultProvider) recordFinished(instanceID string, latest *Task) {
	if latest == nil || !latest.Finished() {
		return
	}
	if cached, ok := p.tasks.Get(instanceID); ok {
		if previous := cached.(*Task); previous != nil && previous.ID == latest.ID && !previous.Finished() {
			TasksTotal.Inc(map[string]string{stateLabel: string(latest.State)})
		}
	}
}

func newTask(task *ec2types.ReplaceRootVolumeTask) *Task {
	startTime, _ := time.Parse(time.RFC3339, lo.FromPtr(task.StartTime))
	return &Task{
		ID:         lo.FromPtr(task.ReplaceRootVolumeTaskId),
		InstanceID: lo.FromPtr(task.InstanceId),
		ImageID:    lo.FromPtr(task.ImageId),
		State:      task.TaskState,
		StartTime:  startTime,
	}
}

// clientToken fits the operation ID into the 64 characters EC2 allows, staying the same on every retry.
func clientToken(operationID string) string {
	sum := sha256.Sum256([]byte(operationID))
	return hex.EncodeToString(sum[:])
}
