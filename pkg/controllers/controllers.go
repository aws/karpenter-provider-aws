package controllers

import (
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/drift"

	"github.com/aws/karpenter-core/pkg/operator/controller"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/controllers/providers"
)

func NewControllers(ctx awscontext.Context, cloudProvider *cloudprovider.CloudProvider) []controller.Controller {
	sqsProvider := providers.NewSQS(sqs.New(ctx.Session))
	eventBridgeProvider := providers.NewEventBridge(eventbridge.New(ctx.Session), sqsProvider)

	return []controller.Controller{
		nodetemplate.NewController(ctx.KubeClient, sqsProvider, eventBridgeProvider),
		interruption.NewController(ctx.KubeClient, ctx.Clock, ctx.EventRecorder, sqsProvider, ctx.UnavailableOfferingsCache),
		drift.NewController(ctx.KubeClient, cloudProvider),
	}
}
