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

package amifamily

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	clock "k8s.io/utils/clock/testing"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

func keyInput(owners []string, filters ...ec2types.Filter) *ec2.DescribeImagesInput {
	return &ec2.DescribeImagesInput{Filters: filters, Owners: owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(1000)}
}

func keyFilter(name string, values ...string) ec2types.Filter {
	return ec2types.Filter{Name: aws.String(name), Values: values}
}

func TestDescribeImagesCacheKey(t *testing.T) {
	a, b, state := keyFilter("tag:a", "1"), keyFilter("tag:b", "2"), keyFilter("state", "available")
	same := map[string][2]*ec2.DescribeImagesInput{
		// Filters built from a tag map are emitted in map iteration order
		"filter order":          {keyInput(nil, a, b, state), keyInput(nil, state, b, a)},
		"owner order":           {keyInput([]string{"self", "amazon"}, a), keyInput([]string{"amazon", "self"}, a)},
		"duplicate owners":      {keyInput([]string{"self", "self"}, a), keyInput([]string{"self"}, a)},
		"identical request":     {keyInput([]string{"self"}, a, state), keyInput([]string{"self"}, a, state)},
		"nil vs empty owners":   {keyInput(nil, a), keyInput([]string{}, a)},
		"duplicate owner order": {keyInput([]string{"self", "amazon", "self"}, a), keyInput([]string{"amazon", "self"}, a)},
	}
	for name, in := range same {
		if describeImagesCacheKey(in[0]) != describeImagesCacheKey(in[1]) {
			t.Errorf("%s: expected equal keys", name)
		}
	}
	base := keyInput([]string{"self"}, a, state)
	differ := map[string]*ec2.DescribeImagesInput{
		"owner":              keyInput([]string{"amazon"}, a, state),
		"duplicated owner":   keyInput([]string{"self", "self", "amazon", "amazon"}, a, state),
		"no owner":           keyInput(nil, a, state),
		"filter value":       keyInput([]string{"self"}, keyFilter("tag:a", "2"), state),
		"extra filter":       keyInput([]string{"self"}, a, b, state),
		"state filter value": keyInput([]string{"self"}, a, keyFilter("state", "pending")),
		"missing first":      keyInput([]string{"self"}, b, state),
		"max results":        {Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(5)},
		"include deprecated": {Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(false), MaxResults: aws.Int32(1000)},
		"image ids":          {Filters: base.Filters, Owners: base.Owners, IncludeDeprecated: aws.Bool(true), MaxResults: aws.Int32(1000), ImageIds: []string{"ami-1"}},
	}
	for name, in := range differ {
		if describeImagesCacheKey(base) == describeImagesCacheKey(in) {
			t.Errorf("%s: expected distinct keys", name)
		}
	}
}

// pagingEC2API serves DescribeImages in two pages; every other EC2 method is unimplemented
type pagingEC2API struct {
	sdk.EC2API
	calls int
}

func (p *pagingEC2API) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	p.calls++
	if in.NextToken == nil {
		return &ec2.DescribeImagesOutput{Images: []ec2types.Image{{ImageId: aws.String("ami-page-1")}}, NextToken: aws.String("page-2")}, nil
	}
	return &ec2.DescribeImagesOutput{Images: []ec2types.Image{{ImageId: aws.String("ami-page-2")}}}, nil
}

func TestDescribeImagesCachesEveryPage(t *testing.T) {
	api := &pagingEC2API{}
	p := &DefaultProvider{clk: clock.NewFakeClock(time.Now()), cache: cache.New(time.Hour, time.Hour), ec2api: api}
	input := keyInput([]string{"self"}, keyFilter("tag:a", "1"))
	for _, skip := range []bool{false, false, true} {
		images, err := p.describeImages(context.Background(), input, skip)
		if err != nil {
			t.Fatal(err)
		}
		if len(images) != 2 || aws.ToString(images[0].ImageId) != "ami-page-1" || aws.ToString(images[1].ImageId) != "ami-page-2" {
			t.Fatalf("skipCache=%v: expected images from both pages, got %v", skip, images)
		}
	}
	// two pages for the first call, served from cache for the second, two pages for the SkipCache call
	if api.calls != 4 {
		t.Fatalf("expected 4 DescribeImages calls, got %d", api.calls)
	}
}
