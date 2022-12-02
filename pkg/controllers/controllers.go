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

	"github.com/aws/karpenter-core/pkg/operator/controller"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/utils/project"
)

func NewControllers(ctx awscontext.Context) []controller.Controller {
	logging.FromContext(ctx).With("version", project.Version).Debugf("discovered version")
	return []controller.Controller{
		interruption.NewController(ctx.KubeClient, ctx.Clock, ctx.EventRecorder, interruption.NewSQSProvider(sqs.New(ctx.Session)), ctx.UnavailableOfferingsCache),
	}
}
