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
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/aws/karpenter-provider-aws/test/hack/resource/pkg/metrics"
	"github.com/aws/karpenter-provider-aws/test/hack/resource/pkg/resourcetypes"
)

const (
	resourceCountTableName = "resourceCount"
	globalRegion           = "global"
)

func main() {
	global := flag.Bool("global", false, "also count account-wide resources like IAM roles. Only one region needs to.")
	flag.Parse()

	ctx := context.Background()
	cfg := lo.Must(config.LoadDefaultConfig(ctx))
	cfg.RetryMaxAttempts = 10

	logger := lo.Must(zap.NewProduction()).Sugar()

	ec2Client := ec2.NewFromConfig(cfg)
	metricsClient := metrics.Multi{metrics.NewTimeStream(cfg), metrics.NewCloudWatch(cfg)}

	// Stacks, ENIs, OIDC providers and instance profiles are counted by Capacity, which lists them once
	resourceTypes := []resourcetypes.Type{
		resourcetypes.NewInstance(ec2Client),
		resourcetypes.NewVPCEndpoint(ec2Client),
		resourcetypes.NewSecurityGroup(ec2Client),
		resourcetypes.NewLaunchTemplate(ec2Client),
		resourcetypes.NewVPCPeeringConnection(ec2Client),
	}
	var errs error
	for i := range resourceTypes {
		resourceLogger := logger.With("type", resourceTypes[i].String())
		resourceCount, err := resourceTypes[i].CountAll(ctx)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("counting %s, %w", resourceTypes[i].String(), err))
			continue
		}
		if err = metricsClient.FireMetric(ctx, resourceCountTableName, resourceTypes[i].String(), float64(resourceCount), lo.Ternary(resourceTypes[i].Global(), globalRegion, cfg.Region)); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("writing %s, %w", resourceTypes[i].String(), err))
		}
		resourceLogger.With("count", resourceCount).Infof("counted resourceTypes")
	}

	capacity := resourcetypes.NewCapacity(ec2Client, cloudformation.NewFromConfig(cfg), eks.NewFromConfig(cfg), iam.NewFromConfig(cfg))
	samples, err := capacity.Regional(ctx)
	errs = multierr.Append(errs, err)
	if *global {
		globalSamples, err := capacity.Global(ctx)
		samples = append(samples, globalSamples...)
		errs = multierr.Append(errs, err)
	}
	for _, s := range samples {
		region := lo.Ternary(s.Global, globalRegion, cfg.Region)
		sampleLogger := logger.With("type", s.Name, "region", region)
		if err := metricsClient.FireMetric(ctx, resourceCountTableName, s.Name, s.Value, region); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("writing %s, %w", s.Name, err))
		}
		sampleLogger.With("count", s.Value).Infof("counted capacity")
	}
	for _, err := range multierr.Errors(errs) {
		logger.Errorf("%v", err)
	}
	// A count or write that fails without failing the job would look like a resource that stopped leaking
	if errs != nil {
		os.Exit(1)
	}
}
