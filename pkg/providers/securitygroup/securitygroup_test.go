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

package securitygroup

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

func TestGetFilterSets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SecurityGroup Provider Suite")
}

var _ = Describe("getFilterSets", func() {
	It("should create filters for security group by ID", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{ID: "sg-123"},
			{ID: "sg-456"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(2))
		Expect(filterSets[0]).To(ContainElement(ec2types.Filter{
			Name:   aws.String("group-id"),
			Values: []string{"sg-123"},
		}))
		Expect(filterSets[1]).To(ContainElement(ec2types.Filter{
			Name:   aws.String("group-id"),
			Values: []string{"sg-456"},
		}))
	})

	It("should create filters for security group by name", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{Name: "my-sg"},
			{Name: "other-sg"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(1))
		Expect(filterSets[0]).To(ContainElement(ec2types.Filter{
			Name:   aws.String("group-name"),
			Values: []string{"my-sg", "other-sg"},
		}))
	})

	It("should create separate filter sets for names with different VPCs", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{Name: "my-sg", VPCID: "vpc-111"},
			{Name: "my-sg", VPCID: "vpc-222"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(2))

		// Each filter set should have both group-name and vpc-id filters
		for _, filterSet := range filterSets {
			Expect(filterSet).To(HaveLen(2))
			hasNameFilter := false
			hasVPCFilter := false
			for _, filter := range filterSet {
				if *filter.Name == "group-name" {
					hasNameFilter = true
					Expect(filter.Values).To(ContainElement("my-sg"))
				}
				if *filter.Name == "vpc-id" {
					hasVPCFilter = true
					Expect(filter.Values).To(Or(
						ContainElement("vpc-111"),
						ContainElement("vpc-222"),
					))
				}
			}
			Expect(hasNameFilter).To(BeTrue())
			Expect(hasVPCFilter).To(BeTrue())
		}
	})

	It("should create filters for security group by name with VPC ID", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{Name: "my-sg", VPCID: "vpc-123"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(1))
		Expect(filterSets[0]).To(ContainElements(
			ec2types.Filter{
				Name:   aws.String("group-name"),
				Values: []string{"my-sg"},
			},
			ec2types.Filter{
				Name:   aws.String("vpc-id"),
				Values: []string{"vpc-123"},
			},
		))
	})

	It("should group names by VPC ID", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{Name: "sg-1", VPCID: "vpc-aaa"},
			{Name: "sg-2", VPCID: "vpc-aaa"},
			{Name: "sg-3", VPCID: "vpc-bbb"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(2))

		// Find the filter set for vpc-aaa
		var vpcAaaFilterSet []ec2types.Filter
		var vpcBbbFilterSet []ec2types.Filter
		for _, filterSet := range filterSets {
			for _, filter := range filterSet {
				if *filter.Name == "vpc-id" {
					switch filter.Values[0] {
					case "vpc-aaa":
						vpcAaaFilterSet = filterSet
					case "vpc-bbb":
						vpcBbbFilterSet = filterSet
					}
				}
			}
		}

		Expect(vpcAaaFilterSet).ToNot(BeNil())
		Expect(vpcBbbFilterSet).ToNot(BeNil())

		// vpc-aaa should have sg-1 and sg-2
		for _, filter := range vpcAaaFilterSet {
			if *filter.Name == "group-name" {
				Expect(filter.Values).To(ConsistOf("sg-1", "sg-2"))
			}
		}

		// vpc-bbb should have sg-3
		for _, filter := range vpcBbbFilterSet {
			if *filter.Name == "group-name" {
				Expect(filter.Values).To(ConsistOf("sg-3"))
			}
		}
	})

	It("should create filters for security group by ID with VPC ID", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{ID: "sg-123", VPCID: "vpc-456"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(1))
		Expect(filterSets[0]).To(ContainElements(
			ec2types.Filter{
				Name:   aws.String("group-id"),
				Values: []string{"sg-123"},
			},
			ec2types.Filter{
				Name:   aws.String("vpc-id"),
				Values: []string{"vpc-456"},
			},
		))
	})

	It("should create filters for security group by tags", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{
					"Name":        "my-sg",
					"Environment": "prod",
				},
			},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(1))
		Expect(filterSets[0]).To(HaveLen(2))
		Expect(filterSets[0]).To(ContainElements(
			ec2types.Filter{
				Name:   aws.String("tag:Name"),
				Values: []string{"my-sg"},
			},
			ec2types.Filter{
				Name:   aws.String("tag:Environment"),
				Values: []string{"prod"},
			},
		))
	})

	It("should create filters for security group by tags with VPC ID", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{
					"Name": "my-sg",
				},
				VPCID: "vpc-789",
			},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(1))
		Expect(filterSets[0]).To(ContainElements(
			ec2types.Filter{
				Name:   aws.String("tag:Name"),
				Values: []string{"my-sg"},
			},
			ec2types.Filter{
				Name:   aws.String("vpc-id"),
				Values: []string{"vpc-789"},
			},
		))
	})

	It("should handle wildcard tags", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{
					"Name": "*",
				},
			},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(1))
		Expect(filterSets[0]).To(ContainElement(ec2types.Filter{
			Name:   aws.String("tag-key"),
			Values: []string{"Name"},
		}))
	})

	It("should handle mixed selector terms", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{ID: "sg-111"},
			{Name: "my-sg"},
			{
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(3))
	})

	It("should create filters for names without VPC ID", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{Name: "sg-1"},
			{Name: "sg-2"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(1))
		Expect(filterSets[0]).To(HaveLen(1))
		Expect(filterSets[0][0]).To(Equal(ec2types.Filter{
			Name:   aws.String("group-name"),
			Values: []string{"sg-1", "sg-2"},
		}))
	})

	It("should handle mix of names with and without VPC ID", func() {
		terms := []v1.SecurityGroupSelectorTerm{
			{Name: "sg-1"},
			{Name: "sg-2", VPCID: "vpc-123"},
			{Name: "sg-3"},
		}
		filterSets := getFilterSets(terms)
		Expect(filterSets).To(HaveLen(2))

		// One filter set for names without VPC (sg-1, sg-3)
		// One filter set for names with vpc-123 (sg-2)
		foundNoVPC := false
		foundWithVPC := false

		for _, filterSet := range filterSets {
			hasVPCFilter := false
			for _, filter := range filterSet {
				if *filter.Name == "vpc-id" {
					hasVPCFilter = true
					Expect(filter.Values).To(Equal([]string{"vpc-123"}))
				}
				if *filter.Name == "group-name" {
					if hasVPCFilter {
						foundWithVPC = true
						Expect(filter.Values).To(Equal([]string{"sg-2"}))
					}
				}
			}
			if !hasVPCFilter {
				foundNoVPC = true
				for _, filter := range filterSet {
					if *filter.Name == "group-name" {
						Expect(filter.Values).To(ConsistOf("sg-1", "sg-3"))
					}
				}
			}
		}

		Expect(foundNoVPC).To(BeTrue())
		Expect(foundWithVPC).To(BeTrue())
	})
})
