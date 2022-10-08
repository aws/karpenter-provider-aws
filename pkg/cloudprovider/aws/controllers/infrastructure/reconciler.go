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

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/controllers"
)

// Reconciler is the AWS infrastructure reconciler
// It plugs into the polling controller to periodically re-reconcile the expected Karpenter AWS infrastructure
type Reconciler struct {
	provider *Provider
}

// pollingPeriod is the period that we go to AWS APIs to ensure that the appropriate AWS infrastructure is provisioned
// This period can be reduced to a backoffPeriod if there is an error in ensuring the infrastructure
const pollingPeriod = time.Hour

func NewReconciler(provider *Provider) *Reconciler {
	return &Reconciler{
		provider: provider,
	}
}

func (r *Reconciler) Metadata() controllers.Metadata {
	return controllers.Metadata{
		Name:             "aws.infrastructure",
		MetricsSubsystem: "aws_infrastructure_controller",
	}
}

// Reconcile reconciles the SQS queue and the EventBridge rules with the expected
// configuration prescribed by Karpenter
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: pollingPeriod}, r.provider.CreateInfrastructure(ctx)
}
