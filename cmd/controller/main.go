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
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	coreapis "github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/operator"
	"github.com/aws/karpenter/pkg/webhooks"

	"github.com/aws/karpenter-core/pkg/cloudprovider/metrics"
	corecontrollers "github.com/aws/karpenter-core/pkg/controllers"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	coreoperator "github.com/aws/karpenter-core/pkg/operator"
	corewebhooks "github.com/aws/karpenter-core/pkg/webhooks"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
	v1alpha5.NormalizedLabels = lo.Assign(v1alpha5.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
	corev1beta1.NormalizedLabels = lo.Assign(corev1beta1.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
	coreapis.Settings = append(coreapis.Settings, apis.Settings...)
}

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

	op.
		WithControllers(ctx, corecontrollers.NewControllers(
			op.Clock,
			op.GetClient(),
			op.KubernetesInterface,
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
			awsCloudProvider,
			op.SubnetProvider,
			op.SecurityGroupProvider,
			op.InstanceProfileProvider,
			op.InstanceProvider,
			op.PricingProvider,
			op.AMIProvider,
		)...).
		WithWebhooks(ctx, webhooks.NewWebhooks()...).
		Start(ctx)
}
