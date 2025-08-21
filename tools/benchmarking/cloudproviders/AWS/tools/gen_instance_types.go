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
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

type InstanceOffering struct {
	Requirements []corev1.NodeSelectorRequirement `json:"requirements"`
	Price        float64                          `json:"price"`
	Available    bool                             `json:"available"`
}

type InstanceTypeOption struct {
	Name      string             `json:"name"`
	Offerings []InstanceOffering `json:"offerings"`
}

func main() {
	lo.Must0(os.Setenv("SYSTEM_NAMESPACE", "karpenter"))
	lo.Must0(os.Setenv("AWS_SDK_LOAD_CONFIG", "true"))

	ctx := coreoptions.ToContext(context.Background(), coretest.Options(coretest.OptionsFields{
		FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(false)},
	}))
	ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
		ClusterName:     lo.ToPtr("docs-gen"),
		ClusterEndpoint: lo.ToPtr("https://docs-gen.aws"),
	}))

	for _, region := range []string{"us-west-2"} {
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
		// Fake a NodeClass so we can use it to get InstanceTypes
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
		instanceTypes, err := instanceTypeProvider.List(ctx, nodeClass)
		if err != nil {
			log.Fatalf("listing instance types, %s", err)
		}
		// convert instance types to json readable for capacity type and availability zone
		instanceTypeOpts := []InstanceTypeOption{}
		for _, it := range instanceTypes {
			instanceTypeOpts = append(instanceTypeOpts, InstanceTypeOption{
				Name: it.Name,
				Offerings: lo.Map(it.Offerings, func(offering *cloudprovider.Offering, _ int) InstanceOffering {
					requirements := []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelTopologyZone,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{offering.Zone()},
						},
						{
							Key:      karpv1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{offering.CapacityType()},
						},
					}
					return InstanceOffering{
						Requirements: requirements,
						// CapacityType:     offering.CapacityType(),
						// AvailabilityZone: offering.Zone(),
						Price:     offering.Price,
						Available: offering.Available,
					}
				}),
			})
		}
		outputFile := "../instance-types.json"
		f, err := os.Create(outputFile)
		if err != nil {
			log.Fatalf("error creating output file %s: %v", outputFile, err)
		}
		defer f.Close()

		// Marshal the instance types to JSON
		jsonData, err := json.MarshalIndent(instanceTypeOpts, "", "  ")
		if err != nil {
			log.Fatalf("error marshaling instance types to JSON: %v", err)
		}

		// Write the JSON data to the file
		if _, err := f.Write(jsonData); err != nil {
			log.Fatalf("error writing JSON data to file: %v", err)
		}

		fmt.Printf("Successfully wrote %d instance types to %s\n", len(instanceTypes), outputFile)
	}
}
