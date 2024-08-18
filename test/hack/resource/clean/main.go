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
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/aws/karpenter-provider-aws/test/hack/resource/pkg/metrics"
	"github.com/aws/karpenter-provider-aws/test/hack/resource/pkg/resourcetypes"
)

const sweeperCleanedResourcesTableName = "sweeperCleanedResources"

var excludedClusters = []string{}

func main() {
	expiration := flag.String("expiration", "12h", "define the expirationTTL of the resources")
	clusterName := flag.String("cluster-name", "", "define cluster name to cleanup")
	flag.Parse()

	ctx := context.Background()
	cfg := lo.Must(config.LoadDefaultConfig(ctx))
	cfg.RetryMaxAttempts = 10

	logger := lo.Must(zap.NewProduction()).Sugar()

	expirationTTL, err := time.ParseDuration(lo.FromPtr(expiration))
	if err != nil {
		logger.Fatalln("need a valid expiration duration", err)
	}
	expirationTime := time.Now().Add(-expirationTTL)

	logger.With("expiration-time", expirationTime.String()).Infof("resolved expiration time for all resourceTypes")

	ec2Client := ec2.NewFromConfig(cfg)
	cloudFormationClient := cloudformation.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)

	metricsClient := metrics.Client(metrics.NewTimeStream(cfg))

	// These resources are intentionally ordered so that instances that are using ENIs
	// will be cleaned before ENIs are attempted to be cleaned up. Likewise, instances and ENIs
	// are cleaned up before security groups are cleaned up to ensure that everything is detached and doesn't
	// prevent deletion
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
		var ids []string
		var err error
		// If there's no cluster defined, clean up all expired resources. otherwise, only cleanup the resources associated with the cluster
		if lo.FromPtr(clusterName) == "" {
			ids, err = resourceTypes[i].GetExpired(ctx, expirationTime, excludedClusters)
		} else if !slices.Contains(excludedClusters, lo.FromPtr(clusterName)) {
			ids, err = resourceTypes[i].Get(ctx, lo.FromPtr(clusterName))
		}
		if err != nil {
			resourceLogger.Errorf("%v", err)
		}
		cleaned := []string{}
		resourceLogger.With("ids", ids, "count", len(ids)).Infof("discovered resourceTypes")
		if len(ids) > 0 {
			cleaned, err = resourceTypes[i].Cleanup(ctx, ids)
			if err != nil {
				resourceLogger.Errorf("%v", err)
			}
			resourceLogger.With("ids", cleaned, "count", len(cleaned)).Infof("deleted resourceTypes")
		}
		// Should only fire metrics if the resource have expired
		if lo.FromPtr(clusterName) == "" {
			if err = metricsClient.FireMetric(ctx, sweeperCleanedResourcesTableName, fmt.Sprintf("%sDeleted", resourceTypes[i].String()), float64(len(cleaned)), lo.Ternary(resourceTypes[i].Global(), "global", cfg.Region)); err != nil {
				resourceLogger.Errorf("%v", err)
			}
		}
	}
}
