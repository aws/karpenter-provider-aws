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

package status

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Tags struct {
	ec2API sdk.EC2API
}

func (ip *Tags) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.Spec.Role != "" {
		createLaunchTemplateInput := &ec2.CreateLaunchTemplateInput{
			DryRun: aws.Bool(true),
			TagSpecifications: []ec2types.TagSpecification{
				{ResourceType: ec2types.ResourceTypeInstance, Tags: utils.MergeTags(nodeClass.Spec.Tags)},
				{ResourceType: ec2types.ResourceTypeVolume, Tags: utils.MergeTags(nodeClass.Spec.Tags)},
				{ResourceType: ec2types.ResourceTypeFleet, Tags: utils.MergeTags(nodeClass.Spec.Tags)},
			},
		}
		_, err := ip.ec2API.CreateLaunchTemplate(ctx, createLaunchTemplateInput)
		var APIErr smithy.APIError
		if err != nil && errors.As(err, &APIErr) && APIErr.ErrorCode() == "UnauthorizedOperation" {
			nodeClass.StatusConditions().SetFalse(v1.ConditionTypeTagsReady, "UnauthorizedOperation", fmt.Sprintf("role does not contain required permissions: %v", err.Error()))
			return reconcile.Result{}, fmt.Errorf("reconciling Tags, %w", err)
		}
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeTagsReady)
	return reconcile.Result{RequeueAfter: time.Minute}, nil
}
