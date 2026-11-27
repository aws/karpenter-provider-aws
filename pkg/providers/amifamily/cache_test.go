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
	corev1 "k8s.io/api/core/v1"

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

var _ = Describe("AMIProvider cache sharing", func() {
	amiIDs := func(amis amifamily.AMIs) []string {
		return lo.Map(amis, func(a amifamily.AMI, _ int) string { return a.AmiID })
	}
	image := func(id, arch, deprecationTime string) ec2types.Image {
		return ec2types.Image{
			Name: aws.String(id), ImageId: aws.String(id), Architecture: ec2types.ArchitectureValues(arch),
			CreationDate: aws.String("2022-01-01T00:00:00Z"), DeprecationTime: lo.EmptyableToPtr(deprecationTime), State: ec2types.ImageStateAvailable,
			Tags: []ec2types.Tag{{Key: aws.String("foo"), Value: aws.String("bar")}},
		}
	}
	It("should share a cache entry across NodeClasses with the same selector term", func() {
		// Distinct UIDs and Generations, so a per-NodeClass key would not share
		nc1 := test.EC2NodeClass()
		nc1.UID, nc1.Generation = "uid-1", 1
		nc1.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
		nc2 := test.EC2NodeClass()
		nc2.UID, nc2.Generation = "uid-2", 3
		nc2.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
		_, err := awsEnv.AMIProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		_, err = awsEnv.AMIProvider.List(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(awsEnv.AMICache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
	})
	It("should not share a cache entry across NodeClasses with different owners, and pass each owner to DescribeImages", func() {
		// Sharing here would surface an AMI from an account the second NodeClass never authorized
		nc1 := test.EC2NodeClass()
		nc1.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI, Owner: "111111111111"}}
		nc2 := test.EC2NodeClass()
		nc2.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI, Owner: "222222222222"}}
		_, err := awsEnv.AMIProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		_, err = awsEnv.AMIProvider.List(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(awsEnv.AMICache.Items()).To(HaveLen(2))
		owners := []string{}
		for awsEnv.EC2API.CalledWithDescribeImagesInput.Len() > 0 {
			owners = append(owners, awsEnv.EC2API.CalledWithDescribeImagesInput.Pop().Owners...)
		}
		Expect(owners).To(ConsistOf("111111111111", "222222222222"))
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
	Context("Known requirements", func() {
		var k8sVersion string
		windowsParam := func(version string) string {
			return fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-%s-English-Core-EKS_Optimized-%s/image_id", version, k8sVersion)
		}
		BeforeEach(func() {
			k8sVersion = awsEnv.VersionProvider.Get(ctx)
		})
		It("should share a cache entry between an alias and an ID term for the same AMI while each derives its own requirements", func() {
			awsEnv.SSMAPI.Parameters = map[string]string{windowsParam("2022"): amd64AMI}
			aliasNC := test.EC2NodeClass()
			aliasNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
			idNC := test.EC2NodeClass()
			idNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amd64AMI}}
			// ID, alias, ID: neither order lets one NodeClass's requirements leak into the other's
			for _, nc := range []*v1.EC2NodeClass{idNC, aliasNC, idNC} {
				amis, err := awsEnv.AMIProvider.List(ctx, nc)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis).To(HaveLen(1))
				if nc == aliasNC {
					Expect(amis[0].Requirements.Get(corev1.LabelOSStable).Has(string(corev1.Windows))).To(BeTrue())
					Expect(amis[0].Requirements.Get(corev1.LabelWindowsBuild).Has(v1.Windows2022Build)).To(BeTrue())
				} else {
					Expect(amis[0].Requirements.Has(corev1.LabelOSStable)).To(BeFalse())
				}
			}
			Expect(awsEnv.AMICache.Items()).To(HaveLen(1))
			Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
		})
		It("should resolve each alias's own requirements when two aliases resolve the same AMI", func() {
			// Both aliases send the same request and differ only in the values of their known requirements
			awsEnv.SSMAPI.Parameters = map[string]string{windowsParam("2022"): amd64AMI, windowsParam("2019"): amd64AMI}
			for alias, build := range map[string]string{"windows2022@latest": v1.Windows2022Build, "windows2019@latest": v1.Windows2019Build} {
				nc := test.EC2NodeClass()
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				amis, err := awsEnv.AMIProvider.List(ctx, nc)
				Expect(err).ToNot(HaveOccurred())
				Expect(amis).To(HaveLen(1))
				Expect(amis[0].Requirements.Get(corev1.LabelWindowsBuild).Values()).To(ConsistOf(build), alias)
			}
			Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
		})
	})
	It("should evaluate deprecation against the current time on a cache hit, inclusive of the deprecation time", func() {
		now := time.Now().Truncate(time.Second)
		awsEnv.Clock.SetTime(now)
		awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
			image(amd64AMI, "x86_64", now.Add(time.Hour).Format(time.RFC3339)),
		}})
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amd64AMI}}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
		Expect(amis[0].Deprecated).To(BeFalse())

		// The deprecation time arrives while the result is still cached
		awsEnv.Clock.Step(time.Hour)
		amis, err = awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis).To(HaveLen(1))
		Expect(amis[0].Deprecated).To(BeTrue())
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
	})
	It("should bypass the cache with SkipCache and write the fresh result through", func() {
		awsEnv.Clock.SetTime(time.Now())
		awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{image(amd64AMI, "x86_64", "")}})
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amd64AMI}}
		amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis[0].Deprecated).To(BeFalse())

		// The AMI is now deprecated at the API, but a cached List still returns the stale result
		awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
			image(amd64AMI, "x86_64", awsEnv.Clock.Now().Add(-time.Hour).Format(time.RFC3339)),
		}})
		amis, err = awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis[0].Deprecated).To(BeFalse())

		amis, err = awsEnv.AMIProvider.List(ctx, nodeClass, amifamily.SkipCache)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis[0].Deprecated).To(BeTrue())

		// The fresh result replaced the cached one
		calls := awsEnv.EC2API.CalledWithDescribeImagesInput.Len()
		amis, err = awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(amis[0].Deprecated).To(BeTrue())
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(calls))
	})
	It("should choose the best AMI across queries regardless of which query was cached by another NodeClass", func() {
		awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
			{Name: aws.String("img-old"), ImageId: aws.String("ami-old"), Architecture: "x86_64", CreationDate: aws.String("2020-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable},
			{Name: aws.String("img-new"), ImageId: aws.String("ami-new"), Architecture: "x86_64", CreationDate: aws.String("2022-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable},
		}})
		oldNC := test.EC2NodeClass()
		oldNC.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: "img-old"}}
		amis, err := awsEnv.AMIProvider.List(ctx, oldNC)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-old"))

		for _, terms := range [][]v1.AMISelectorTerm{
			{{Name: "img-old"}, {Name: "img-new"}},
			{{Name: "img-new"}, {Name: "img-old"}},
		} {
			nc := test.EC2NodeClass()
			nc.Spec.AMISelectorTerms = terms
			amis, err = awsEnv.AMIProvider.List(ctx, nc)
			Expect(err).ToNot(HaveOccurred())
			Expect(amiIDs(amis)).To(ConsistOf("ami-new"))
		}
		// oldNC is not polluted
		amis, err = awsEnv.AMIProvider.List(ctx, oldNC)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-old"))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(2))
	})
	It("should cache queries resolved before a failure in the same List, and not the failed one", func() {
		failing := &failingNameEC2API{EC2API: awsEnv.EC2API, failName: "fail-me"}
		amiCache := cache.New(time.Hour, time.Hour)
		provider := amifamily.NewDefaultProvider(awsEnv.Clock, awsEnv.VersionProvider, awsEnv.SSMProvider, failing, amiCache)
		nc := test.EC2NodeClass()
		nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}, {Name: "fail-me"}}
		_, err := provider.List(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(amiCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))

		// The query resolved before the failure is served from the cache
		other := test.EC2NodeClass()
		other.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Name: amd64AMI}}
		amis, err := provider.List(ctx, other)
		Expect(err).ToNot(HaveOccurred())
		Expect(amiIDs(amis)).To(ConsistOf("ami-id-amd64"))
		Expect(awsEnv.EC2API.CalledWithDescribeImagesInput.Len()).To(Equal(1))
	})
	It("should cache every page of a paginated DescribeImages response", func() {
		paged := &pagedImagesEC2API{EC2API: awsEnv.EC2API, pages: [][]ec2types.Image{
			{image("ami-page1", "x86_64", "")},
			{image("ami-page2", "arm64", "")},
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
	It("should drop images whose architecture Karpenter does not support", func() {
		awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
			image("ami-x86", "x86_64", ""),
			{Name: aws.String("img-i386"), ImageId: aws.String("ami-i386"), Architecture: "i386", CreationDate: aws.String("2023-01-01T00:00:00Z"), State: ec2types.ImageStateAvailable, Tags: []ec2types.Tag{{Key: aws.String("foo"), Value: aws.String("bar")}}},
		}})
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
		// Both the uncached and cached paths
		for range 2 {
			amis, err := awsEnv.AMIProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(amiIDs(amis)).To(ConsistOf("ami-x86"))
		}
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

var _ = Describe("DescribeImagesCacheKey", func() {
	f := func(name string, values ...string) ec2types.Filter {
		return ec2types.Filter{Name: aws.String(name), Values: values}
	}
	input := func(owners []string, filters ...ec2types.Filter) *ec2.DescribeImagesInput {
		return &ec2.DescribeImagesInput{Filters: filters, Owners: owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(1000)}
	}
	a, b, state := f("tag:a", "1"), f("tag:b", "2"), f("state", "available")
	base := input([]string{"self"}, a, state)

	DescribeTable("keys requests that select the same images alike",
		func(x, y *ec2.DescribeImagesInput) {
			Expect(amifamily.DescribeImagesCacheKey(x)).To(Equal(amifamily.DescribeImagesCacheKey(y)))
		},
		// Filters built from a tag map are emitted in map iteration order
		Entry("filter order", input(nil, a, b, state), input(nil, state, b, a)),
		Entry("owner order", input([]string{"self", "amazon"}, a), input([]string{"amazon", "self"}, a)),
		Entry("duplicate owners", input([]string{"self", "amazon", "self"}, a), input([]string{"amazon", "self"}, a)),
		Entry("nil and empty owners", input(nil, a), input([]string{}, a)),
	)
	DescribeTable("keys requests that select different images differently",
		func(other *ec2.DescribeImagesInput) {
			Expect(amifamily.DescribeImagesCacheKey(base)).ToNot(Equal(amifamily.DescribeImagesCacheKey(other)))
		},
		Entry("owner", input([]string{"amazon"}, a, state)),
		Entry("additional owner", input([]string{"self", "self", "amazon", "amazon"}, a, state)),
		Entry("no owner", input(nil, a, state)),
		Entry("filter value", input([]string{"self"}, f("tag:a", "2"), state)),
		Entry("extra filter", input([]string{"self"}, a, b, state)),
		Entry("different filter", input([]string{"self"}, b, state)),
		Entry("state filter value", input([]string{"self"}, a, f("state", "pending"))),
		// Every other request field is part of the key, including ones added later
		Entry("MaxResults", &ec2.DescribeImagesInput{Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(5)}),
		Entry("IncludeDeprecated", &ec2.DescribeImagesInput{Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(false), MaxResults: aws.Int32(1000)}),
		Entry("ImageIds", &ec2.DescribeImagesInput{Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(1000), ImageIds: []string{"ami-1"}}),
	)
})
