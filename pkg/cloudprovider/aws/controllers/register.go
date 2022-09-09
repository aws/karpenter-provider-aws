package controllers

import (
	"context"

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/controllers"
)

func Register(ctx context.Context, provider *aws.CloudProvider, opts *controllers.ControllerOptions) {
	rec := events.NewRecorder(opts.Recorder)
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("aws"))

	// Injecting the controllers that will start when opts.StartAsync is triggered
	notification.NewController(ctx, opts.Clock, opts.KubeClient, provider.SQSProvider(), rec, opts.Provisioner, opts.Cluster, opts.StartAsync)
	infrastructure.NewController(ctx, opts.Clock, rec, provider.SQSProvider(), provider.EventBridgeProvider(), opts.StartAsync)
}
