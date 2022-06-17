package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Printf("Usage: %s path/to/markdown.md", os.Args[0])
		os.Exit(1)
	}

	os.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("CLUSTER_NAME", "docs-gen")
	os.Setenv("CLUSTER_ENDPOINT", "https://docs-gen.aws")
	ctx := injection.WithOptions(context.Background(), options.New().MustParse())

	cp := aws.NewCloudProvider(ctx, cloudprovider.Options{
		ClientSet:  nil,
		KubeClient: nil,
	})
	provider := v1alpha1.AWS{SubnetSelector: map[string]string{
		"*": "*",
	}}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(provider); err != nil {
		log.Fatalf("encoding provider, %s", err)
	}
	instanceTypes, err := cp.GetInstanceTypes(ctx, &v1alpha5.Provider{
		Raw:    buf.Bytes(),
		Object: nil,
	})
	if err != nil {
		log.Fatalf("listing instance types, %s", err)
	}

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
	fmt.Fprintln(f, "<!-- this document is generated from hack/docs/instancetypes_gen_docs.go -->")
	fmt.Fprintln(f, `AWS instance types offer varying resources and can be selected by labels. The values provided 
below are the resources available with some assumptions and after the instance overhead has been subtracted:
- `+"`blockDeviceMappings` are not configured"+`
- `+"`aws-eni-limited-pod-density` is assumed to be `true`"+`
- `+"`amiFamily` is set to the default of `AL2`")

	// generate a map of family -> instance types along with some other sorted lists.  The sorted lists ensure we
	// generate consistent docs every run.
	families := map[string][]cloudprovider.InstanceType{}
	labelNameMap := sets.String{}
	resourceNameMap := sets.String{}
	for _, it := range instanceTypes {
		familyName := strings.Split(it.Name(), ".")[0]
		families[familyName] = append(families[familyName], it)
		for labelName := range it.Requirements() {
			labelNameMap.Insert(labelName)
		}
		for resourceName := range it.Resources() {
			resourceNameMap.Insert(string(resourceName))
		}
	}
	familyNames := lo.Keys(families)
	sort.Strings(familyNames)

	// we don't want to show the zone label that was applied based on our credentials
	delete(labelNameMap, v1.LabelTopologyZone)
	labelNames := lo.Keys(labelNameMap)

	sort.Strings(labelNames)
	resourceNames := lo.Keys(resourceNameMap)
	sort.Strings(resourceNames)

	for _, familyName := range familyNames {
		fmt.Fprintf(f, "## %s Family\n", familyName)

		// sort the instance types within the family, we sort by CPU and memory which should be a pretty good ordering
		sort.Slice(families[familyName], func(a, b int) bool {
			lhs := families[familyName][a]
			rhs := families[familyName][b]
			lhsResources := lhs.Resources()
			rhsResources := rhs.Resources()
			if cpuCmp := resources.Cmp(*lhsResources.Cpu(), *rhsResources.Cpu()); cpuCmp != 0 {
				return cpuCmp < 0
			}
			if memCmp := resources.Cmp(*lhsResources.Memory(), *rhsResources.Memory()); memCmp != 0 {
				return memCmp < 0
			}
			return lhs.Name() < rhs.Name()
		})

		for _, it := range families[familyName] {
			fmt.Fprintf(f, "### `%s`\n", it.Name())
			minusOverhead := v1.ResourceList{}
			for k, v := range it.Resources() {
				if v.IsZero() {
					continue
				}
				cp := v.DeepCopy()
				cp.Sub(it.Overhead()[k])
				minusOverhead[k] = cp
			}
			fmt.Fprintln(f, "#### Labels")
			fmt.Fprintln(f, " | Label | Value |")
			fmt.Fprintln(f, " |--|--|")
			for _, label := range labelNames {
				req, ok := it.Requirements()[label]
				if !ok {
					continue
				}
				if req.Values().Len() == 1 {
					fmt.Fprintf(f, " |%s|%s|\n", label, req.Values().List()[0])
				}
			}
			fmt.Fprintln(f, "#### Resources")
			fmt.Fprintln(f, " | Resource | Quantity |")
			fmt.Fprintln(f, " |--|--|")
			for _, resourceName := range resourceNames {
				quantity := minusOverhead[v1.ResourceName(resourceName)]
				if quantity.IsZero() {
					continue
				}
				if v1.ResourceName(resourceName) == v1.ResourceEphemeralStorage {
					i64, _ := quantity.AsInt64()
					quantity = *resource.NewQuantity(i64, resource.BinarySI)
				}
				fmt.Fprintf(f, " |%s|%s|\n", resourceName, quantity.String())
			}
		}
	}
}
