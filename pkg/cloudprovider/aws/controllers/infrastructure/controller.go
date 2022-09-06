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

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers"
)

// Controller is the AWS infrastructure controller.  It is not a standard controller-runtime controller in that it doesn't
// have a reconcile method.
type Controller struct {
	sqsProvider *aws.SQSProvider
	recorder    controllers.Recorder
	clock       clock.Clock
}

// pollingPeriod that we go to AWS APIs to ensure that the appropriate AWS infrastructure is provisioned
const pollingPeriod = 15 * time.Minute

func NewController(ctx context.Context, clk clock.Clock, recorder controllers.Recorder,
	sqsProvider *aws.SQSProvider, startAsync <-chan struct{}) *Controller {
	c := &Controller{
		recorder:    recorder,
		clock:       clk,
		sqsProvider: sqsProvider,
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
	logger := logging.FromContext(ctx).Named("infrastructure")
	ctx = logging.WithLogger(ctx, logger)
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Shutting down")
			return
		case <-time.After(pollingPeriod):
			c.ensureInfrastructure(ctx)
		}
	}
}

func (c *Controller) ensureInfrastructure(ctx context.Context) error {
	return nil
}
