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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/aws/karpenter-provider-aws/test/hack/resource/pkg/metrics"
	"github.com/aws/karpenter-provider-aws/test/hack/resource/pkg/resourcetypes"
)

const resourceCountTableName = "resourceCount"

func main() {
	ctx := context.Background()
	cfg := lo.Must(config.LoadDefaultConfig(ctx))

	logger := lo.Must(zap.NewProduction()).Sugar()

	ec2Client := ec2.NewFromConfig(cfg)
	cloudFormationClient := cloudformation.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)
	metricsClient := metrics.Client(metrics.NewTimeStream(cfg))

	resourceTypes := []resourcetypes.Type{
		resourcetypes.NewInstance(ec2Client),
		resourcetypes.NewVPCEndpoint(ec2Client),
		resourcetypes.NewENI(ec2Client),
		resourcetypes.NewSecurityGroup(ec2Client),
		resourcetypes.NewLaunchTemplate(ec2Client),
		resourcetypes.NewOIDC(iamClient),
		resourcetypes.NewInstanceProfile(iamClient),
		resourcetypes.NewStack(cloudFormationClient),
		resourcetypes.NewVPCPeeringConnection(ec2Client),
	}

	for i := range resourceTypes {
		resourceLogger := logger.With("type", resourceTypes[i].String())
		resourceCount, err := resourceTypes[i].CountAll(ctx)
		if err != nil {
			resourceLogger.Errorf("%v", err)
		}

		if err = metricsClient.FireMetric(ctx, resourceCountTableName, resourceTypes[i].String(), float64(resourceCount), lo.Ternary(resourceTypes[i].Global(), "global", cfg.Region)); err != nil {
			resourceLogger.Errorf("%v", err)
		}
		resourceLogger.With("count", resourceCount).Infof("counted resourceTypes")
	}
}
