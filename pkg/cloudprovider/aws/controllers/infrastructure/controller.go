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

package infrastructure

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/aggregatedparser"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/state"
)

// Controller is the consolidation controller.  It is not a standard controller-runtime controller in that it doesn't
// have a reconcile method.
type Controller struct {
	kubeClient  client.Client
	provisioner *provisioning.Provisioner
	cluster     *state.Cluster
	recorder    controllers.Recorder
	clock       clock.Clock
	parser      event.Parser
}

// pollingPeriod that we go to the SQS queue to check if there are any new events
const pollingPeriod = 2 * time.Second

func NewController(ctx context.Context, clk clock.Clock, kubeClient client.Client, recorder controllers.Recorder,
	cluster *state.Cluster, startAsync <-chan struct{}) *Controller {
	c := &Controller{
		kubeClient: kubeClient,
		cluster:    cluster,
		recorder:   recorder,
		clock:      clk,
		parser:     aggregatedparser.NewAggregatedParser(aggregatedparser.DefaultParsers...),
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-startAsync:
			c.run(ctx)
		}
	}()

	return c
}

func (c *Controller) run(ctx context.Context) {
	logger := logging.FromContext(ctx).Named("notification")
	ctx = logging.WithLogger(ctx, logger)
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Shutting down")
			return
		case <-time.After(pollingPeriod):
			logging.FromContext(ctx).Infof("polled after the polling period")
		}
	}
}
