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

package controllers

import (
	"github.com/aws/aws-sdk-go/service/sqs"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/cloudprovider"

	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/utils/project"

	"github.com/aws/karpenter-core/pkg/operator/controller"
)

func NewControllers(ctx awscontext.Context, cloudProvider *cloudprovider.CloudProvider) (controllers []controller.Controller) {
	logging.FromContext(ctx).With("version", project.Version).Debugf("discovered version")

	if settings.FromContext(ctx).InterruptionQueueName != "" {
		controllers = append(controllers, interruption.NewController(ctx.KubeClient, ctx.Clock, ctx.EventRecorder, interruption.NewSQSProvider(sqs.New(ctx.Session)), ctx.UnavailableOfferingsCache))
	}
	controllers = append(controllers, nodetemplate.NewController(ctx.KubeClient, ctx.SubnetProvider, ctx.SecurityGroupProvider))
	return controllers
}
