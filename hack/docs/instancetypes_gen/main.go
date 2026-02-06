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
	"log"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

// FakeManager is a manager that takes all the utilized calls from the operator setup
type FakeManager struct {
	manager.Manager
}

func (m *FakeManager) GetClient() client.Client {
	return fake.NewClientBuilder().Build()
}

func (m *FakeManager) GetConfig() *rest.Config {
	return &rest.Config{}
}

func (m *FakeManager) GetFieldIndexer() client.FieldIndexer {
	return &FakeFieldIndexer{}
}

func (m *FakeManager) Elected() <-chan struct{} {
	return make(chan struct{}, 1)
}

type FakeFieldIndexer struct{}

func (f *FakeFieldIndexer) IndexField(_ context.Context, _ client.Object, _ string, _ client.IndexerFunc) error {
	return nil
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalf("Usage: %s path/to/markdown.md", os.Args[0])
	}

	lo.Must0(os.Setenv("SYSTEM_NAMESPACE", "karpenter"))
	lo.Must0(os.Setenv("AWS_SDK_LOAD_CONFIG", "true"))

	ctx := coreoptions.ToContext(context.Background(), coretest.Options(coretest.OptionsFields{
		FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(false)},
	}))
	ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
		ClusterName:     lo.ToPtr("docs-gen"),
		ClusterEndpoint: lo.ToPtr("https://docs-gen.aws"),
	}))

	outputFileName := flag.Arg(0)
	f, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("error creating output file %s, %s", outputFileName, err)
	}

	log.Println("writing output to", outputFileName)
	fmt.Fprintf(f, `---
title: "Instance Types"
linkTitle: "Instance Types"
weight: 100

description: >
  Evaluate Instance Type Resources
---
`)
	fmt.Fprintln(f, "<!-- this document is generated from hack/docs/instancetypes_gen/main.go -->")
	fmt.Fprintln(f, `AWS instance types offer varying resources and can be selected by labels. The values provided
below are the resources available with some assumptions and after the instance overhead has been subtracted:
- `+"`blockDeviceMappings` are not configured"+`
- `+"`amiFamily` is set to `AL2023`")

	// generate a map of family -> map[instance type name]instance types along with some other sorted lists.  The sorted lists ensure we
	// generate consistent docs every run.
	families := map[string]map[string]*cloudprovider.InstanceType{}
	labelNameMap := sets.New[string]()
	resourceNameMap := sets.New[string]()

	// Iterate through regions and take the union of instance types we discover across both
	for _, region := range []string{"us-east-1", "us-east-2", "us-west-2"} {
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
				pricing.NewAPI(cfg, ""),
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
		if err = instanceTypeProvider.UpdateInstanceTypes(ctx); err != nil {
			log.Fatalf("updating instance types, %s", err)
		}
		if err = instanceTypeProvider.UpdateInstanceTypeOfferings(ctx); err != nil {
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
		for _, it := range instanceTypes {
			familyName := strings.Split(string(it.Name), ".")[0]
			if _, ok := families[familyName]; !ok {
				families[familyName] = map[string]*cloudprovider.InstanceType{}
			}
			families[familyName][string(it.Name)] = it
			for labelName := range it.Requirements {
				labelNameMap.Insert(labelName)
			}
			for resourceName := range it.Capacity {
				resourceNameMap.Insert(string(resourceName))
			}
		}
	}

	familyNames := lo.Keys(families)
	sort.Strings(familyNames)

	// we don't want to show a few labels that will vary amongst regions
	delete(labelNameMap, corev1.LabelTopologyZone)
	delete(labelNameMap, v1.LabelTopologyZoneID)
	delete(labelNameMap, karpv1.CapacityTypeLabelKey)

	labelNames := lo.Keys(labelNameMap)

	sort.Strings(labelNames)
	resourceNames := lo.Keys(resourceNameMap)
	sort.Strings(resourceNames)

	for _, familyName := range familyNames {
		fmt.Fprintf(f, "## %s Family\n", familyName)

		instanceTypes := lo.MapToSlice(families[familyName], func(_ string, it *cloudprovider.InstanceType) *cloudprovider.InstanceType { return it })
		// sort the instance types within the family, we sort by CPU and memory which should be a pretty good ordering
		sort.Slice(instanceTypes, func(a, b int) bool {
			lhs := instanceTypes[a]
			rhs := instanceTypes[b]
			lhsResources := lhs.Capacity
			rhsResources := rhs.Capacity
			if cpuCmp := resources.Cmp(*lhsResources.Cpu(), *rhsResources.Cpu()); cpuCmp != 0 {
				return cpuCmp < 0
			}
			if memCmp := resources.Cmp(*lhsResources.Memory(), *rhsResources.Memory()); memCmp != 0 {
				return memCmp < 0
			}
			return lhs.Name < rhs.Name
		})

		for _, it := range instanceTypes {
			fmt.Fprintf(f, "### `%s`\n", it.Name)
			minusOverhead := resources.Subtract(it.Capacity, it.Overhead.Total())
			fmt.Fprintln(f, "#### Labels")
			fmt.Fprintln(f, " | Label | Value |")
			fmt.Fprintln(f, " |--|--|")
			for _, label := range labelNames {
				req, ok := it.Requirements[label]
				if !ok {
					continue
				}
				if req.Key == corev1.LabelTopologyRegion {
					continue
				}
				if len(req.Values()) == 1 {
					fmt.Fprintf(f, " |%s|%s|\n", label, req.Values()[0])
				}
			}
			fmt.Fprintln(f, "#### Resources")
			fmt.Fprintln(f, " | Resource | Quantity |")
			fmt.Fprintln(f, " |--|--|")
			for _, resourceName := range resourceNames {
				quantity := minusOverhead[corev1.ResourceName(resourceName)]
				if quantity.IsZero() {
					continue
				}
				if corev1.ResourceName(resourceName) == corev1.ResourceEphemeralStorage {
					i64, _ := quantity.AsInt64()
					quantity = *resource.NewQuantity(i64, resource.BinarySI)
				}
				fmt.Fprintf(f, " |%s|%s|\n", resourceName, quantity.String())
			}
		}
	}
}
