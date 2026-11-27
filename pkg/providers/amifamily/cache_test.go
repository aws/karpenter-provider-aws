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

package amifamily_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

// pagedImagesEC2API serves DescribeImages as fixed pages; the shared fake never paginates images.
type pagedImagesEC2API struct {
	*fake.EC2API
	pages [][]ec2types.Image
	calls int
}

func (p *pagedImagesEC2API) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	p.calls++
	idx := 0
	if in.NextToken != nil {
		idx = lo.Must(strconv.Atoi(*in.NextToken))
	}
	out := &ec2.DescribeImagesOutput{Images: p.pages[idx]}
	if idx+1 < len(p.pages) {
		out.NextToken = aws.String(strconv.Itoa(idx + 1))
	}
	return out, nil
}

// failingNameEC2API fails DescribeImages for one name filter value and delegates every other call.
type failingNameEC2API struct {
	*fake.EC2API
	failName string
}

func (f *failingNameEC2API) DescribeImages(ctx context.Context, in *ec2.DescribeImagesInput, opts ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	for _, flt := range in.Filters {
		if aws.ToString(flt.Name) == "name" && lo.Contains(flt.Values, f.failName) {
			return nil, fmt.Errorf("throttled")
		}
	}
	return f.EC2API.DescribeImages(ctx, in, opts...)
}

var _ = Describe("AMIProvider cache", func() {
	amiIDs := func(amis amifamily.AMIs) []string {
		return lo.Map(amis, func(a amifamily.AMI, _ int) string { return a.AmiID })
	}
	It("should cache every page of a paginated DescribeImages response", func() {
		paged := &pagedImagesEC2API{EC2API: awsEnv.EC2API, pages: [][]ec2types.Image{
			{{Name: aws.String("p1"), ImageId: aws.String("ami-page1"), Architecture: "x86_64", CreationDate: aws.String("2022-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable}},
			{{Name: aws.String("p2"), ImageId: aws.String("ami-page2"), Architecture: "arm64", CreationDate: aws.String("2022-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable}},
		}}
		provider := amifamily.NewDefaultProvider(awsEnv.Clock, awsEnv.VersionProvider, awsEnv.SSMProvider, paged, cache.New(time.Hour, time.Hour))
		nc := test.EC2NodeClass()
		nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
		amis, err := provider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-page1", "ami-page2"))
		Expect(paged.calls).To(Equal(2))

		// Served from the cache: both pages, no further calls
		amis, err = provider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-page1", "ami-page2"))
		Expect(paged.calls).To(Equal(2))

		// SkipCache re-reads every page
		amis, err = provider.List(ctx, nc, amifamily.SkipCache)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-page1", "ami-page2"))
		Expect(paged.calls).To(Equal(4))
	})
	It("should cache a query that succeeded earlier in the same List that later failed", func() {
		failing := &failingNameEC2API{EC2API: awsEnv.EC2API, failName: "fail-me"}
		amiCache := cache.New(time.Hour, time.Hour)
		provider := amifamily.NewDefaultProvider(awsEnv.Clock, awsEnv.VersionProvider, awsEnv.SSMProvider, failing, amiCache)
		nc := test.EC2NodeClass()
		nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}, {Name: "fail-me"}}
		_, err := provider.List(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(amiCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))

		other := test.EC2NodeClass()
		other.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}}
		amis, err := provider.List(ctx, other)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64"))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
	})
	It("should share a cache entry between an ID term and an SSM parameter term resolving to the same AMI", func() {
		awsEnv.SSMAPI.Parameters = map[string]string{"/custom/ami": "ami-id-amd64"}
		idNC := test.EC2NodeClass()
		idNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-id-amd64"}}
		ssmNC := test.EC2NodeClass()
		ssmNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{SSMParameter: "/custom/ami"}}
		_, err := awsEnv.AMIProvider.List(ctx, idNC)
		Expect(err).ToNot(HaveOccurred())
		amis, err := awsEnv.AMIProvider.List(ctx, ssmNC)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64"))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
	})
	// ID terms are batched into one image-id filter, so partially overlapping ID selectors issue
	// distinct requests and do not share entries (unlike name/tag terms, which are one query each)
	It("should not share a cache entry between ID selectors that only partially overlap", func() {
		nc1 := test.EC2NodeClass()
		nc1.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-id-amd64"}}
		nc2 := test.EC2NodeClass()
		nc2.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-id-amd64"}, {ID: "ami-id-arm64"}}
		_, err := awsEnv.AMIProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		amis, err := awsEnv.AMIProvider.List(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64", "ami-id-arm64"))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(2))
		Expect(awsEnv.AMICache.Items()).To(HaveLen(2))
	})
	It("should return the new selector's AMIs after a NodeClass changes its selector", func() {
		nc := test.EC2NodeClass()
		nc.UID, nc.Generation = "uid-1", 1
		nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}}
		amis, err := awsEnv.AMIProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64"))
		nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: arm64AMI}}
		amis, err = awsEnv.AMIProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-id-arm64"))
	})
})
