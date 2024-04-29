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

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/samber/lo"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers"
	"github.com/aws/karpenter-provider-aws/pkg/operator"
	"github.com/aws/karpenter-provider-aws/pkg/webhooks"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/metrics"
	corecontrollers "sigs.k8s.io/karpenter/pkg/controllers"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	coreoperator "sigs.k8s.io/karpenter/pkg/operator"
	corewebhooks "sigs.k8s.io/karpenter/pkg/webhooks"
)

func main() {
	ctx, op := operator.NewOperator(coreoperator.NewOperator())
	awsCloudProvider := cloudprovider.New(
		op.InstanceTypesProvider,
		op.InstanceProvider,
		op.EventRecorder,
		op.GetClient(),
		op.AMIProvider,
		op.SecurityGroupProvider,
		op.SubnetProvider,
	)
	lo.Must0(op.AddHealthzCheck("cloud-provider", awsCloudProvider.LivenessProbe))
	cloudProvider := metrics.Decorate(awsCloudProvider)

	if err := validateEC2NodeClasses(ctx, op.GetAPIReader()); err != nil {
		logging.FromContext(ctx).Fatalf("validating EC2NodeClasses, %s", err)
	}

	op.
		WithControllers(ctx, corecontrollers.NewControllers(
			op.Clock,
			op.GetClient(),
			state.NewCluster(op.Clock, op.GetClient(), cloudProvider),
			op.EventRecorder,
			cloudProvider,
		)...).
		WithWebhooks(ctx, corewebhooks.NewWebhooks()...).
		WithControllers(ctx, controllers.NewControllers(
			ctx,
			op.Session,
			op.Clock,
			op.GetClient(),
			op.EventRecorder,
			op.UnavailableOfferingsCache,
			cloudProvider,
			op.SubnetProvider,
			op.SecurityGroupProvider,
			op.InstanceProfileProvider,
			op.InstanceProvider,
			op.PricingProvider,
			op.AMIProvider,
			op.LaunchTemplateProvider,
			op.InstanceTypesProvider,
		)...).
		WithWebhooks(ctx, webhooks.NewWebhooks()...).
		Start(ctx)
}

// validateEC2NodeClasses ensures all EC2NodeClasses specify AMISelectorTerms (required as of v0.37.0)
func validateEC2NodeClasses(ctx context.Context, reader client.Reader) error {
	nodeClassList := v1beta1.EC2NodeClassList{}
	err := reader.List(ctx, &nodeClassList)
	if err != nil {
		return fmt.Errorf("listing EC2NodeClasses on startup, %s", err.Error())
	}
	if invalidNodeClasses := lo.FilterMap(nodeClassList.Items, func(nc v1beta1.EC2NodeClass, _ int) (string, bool) {
		return nc.Name, len(nc.Spec.AMISelectorTerms) == 0
	}); len(invalidNodeClasses) != 0 {
		return fmt.Errorf("detected EC2NodeClasses with un-set AMISelectorTerms (%s), refer to the 0.37.0+ upgrade guide: https://karpenter.sh/upgrading/upgrade-guide/#upgrading-to-0370", strings.Join(invalidNodeClasses, ","))
	}
	return nil
}
