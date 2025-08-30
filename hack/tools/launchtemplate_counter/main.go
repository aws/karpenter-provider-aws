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
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

func main() {
	lo.Must0(os.Setenv("AWS_SDK_LOAD_CONFIG", "true"))

	ctx := coreoptions.ToContext(context.Background(), coretest.Options(coretest.OptionsFields{
		FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(false)},
	}))
	ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
		ClusterName:     lo.ToPtr("docs-gen"),
		ClusterEndpoint: lo.ToPtr("https://docs-gen.aws"),
	}))

	region := "us-west-2"
	cfg := lo.Must(config.LoadDefaultConfig(ctx, config.WithRegion(region)))
	ec2api := ec2.NewFromConfig(cfg)
	subnetProvider := subnet.NewDefaultProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AvailableIPAddressTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AssociatePublicIPAddressTTL, awscache.DefaultCleanupInterval))
	instanceTypeProvider := instancetype.NewDefaultProvider(
		cache.New(awscache.InstanceTypesZonesAndOfferingsTTL, awscache.DefaultCleanupInterval),
		cache.New(awscache.InstanceTypesZonesAndOfferingsTTL, awscache.DefaultCleanupInterval),
		cache.New(awscache.DiscoveredCapacityCacheTTL, awscache.DefaultCleanupInterval),
		ec2api,
		subnetProvider,
		pricing.NewDefaultProvider(
			pricing.NewAPI(cfg),
			ec2api,
			cfg.Region,
			true,
		),
		nil,
		awscache.NewUnavailableOfferings(),
		instancetype.NewDefaultResolver(
			region,
		),
	)
	if err := instanceTypeProvider.UpdateInstanceTypes(ctx); err != nil {
		log.Fatalf("updating instance types, %s", err)
	}
	if err := instanceTypeProvider.UpdateInstanceTypeOfferings(ctx); err != nil {
		log.Fatalf("updating instance types offerings, %s", err)
	}
	// Fake a NodeClass, so we can use it to get InstanceTypes
	nodeClass := &v1.EC2NodeClass{
		Spec: v1.EC2NodeClassSpec{
			AMISelectorTerms: []v1.AMISelectorTerm{{
				Alias: "al2023@latest",
			}},
			SubnetSelectorTerms: []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"*": "*",
					},
				},
			},
		},
	}
	subnets, err := subnetProvider.List(ctx, nodeClass)
	if err != nil {
		log.Fatalf("listing subnets, %s", err)
	}
	nodeClass.Status.Subnets = lo.Map(subnets, func(ec2subnet ec2types.Subnet, _ int) v1.Subnet {
		return v1.Subnet{
			ID:   *ec2subnet.SubnetId,
			Zone: *ec2subnet.AvailabilityZone,
		}
	})
	nodeClass.Status.AMIs = []v1.AMI{
		{
			ID:   coretest.RandomName(),
			Name: coretest.RandomName(),
			Requirements: []corev1.NodeSelectorRequirement{
				{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.ArchitectureAmd64},
				},
			},
		},
		{
			ID:   coretest.RandomName(),
			Name: coretest.RandomName(),
			Requirements: []corev1.NodeSelectorRequirement{
				{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.ArchitectureArm64},
				},
			},
		},
	}
	instanceTypes := lo.Must(instanceTypeProvider.List(ctx, nodeClass))

	// See how many launch templates we get by constraining our instance types to just be "c", "m", and "r"
	reqs := scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelInstanceCategory, corev1.NodeSelectorOpIn, "c", "m", "r"))
	instanceTypes = lo.Filter(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		return it.Requirements.Compatible(reqs) == nil
	})
	fmt.Printf("Got %d instance types after filtering\n", len(instanceTypes))

	resolver := amifamily.NewDefaultResolver(region)
	launchTemplates, err := resolver.Resolve(nodeClass, &karpv1.NodeClaim{}, lo.Slice(instanceTypes, 0, 60), karpv1.CapacityTypeOnDemand, &amifamily.Options{InstanceStorePolicy: lo.ToPtr(v1.InstanceStorePolicyRAID0)})

	if err != nil {
		log.Fatalf("resolving launchTemplates, %s", err)
	}
	fmt.Printf("Got %d launch templates back from the resolver\n", len(launchTemplates))
}
