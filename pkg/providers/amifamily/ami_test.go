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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAWS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AMISelector")
}

var (
	defaultOwners = []*string{aws.String("self"), aws.String("amazon")}
)

var _ = Describe("AMI Selectors", func() {
	It("should have default owners and use tags when prefixes aren't set", func() {
		amiSelector := map[string]string{
			"Name": "my-ami",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(ConsistOf(defaultOwners))
		Expect(filters).Should(ConsistOf([]*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: aws.StringSlice([]string{"my-ami"}),
			},
		}))
	})
	It("should have default owners and use name when prefixed", func() {
		amiSelector := map[string]string{
			"aws::name": "my-ami",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(ConsistOf(defaultOwners))
		Expect(filters).Should(ConsistOf([]*ec2.Filter{
			{
				Name:   aws.String("name"),
				Values: aws.StringSlice([]string{"my-ami"}),
			},
		}))
	})
	It("should not set owners when legacy ids are passed", func() {
		amiSelector := map[string]string{
			"aws-ids": "ami-abcd1234,ami-cafeaced",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(BeNil())
		Expect(filters).Should(ConsistOf([]*ec2.Filter{
			{
				Name: aws.String("image-id"),
				Values: aws.StringSlice([]string{
					"ami-abcd1234",
					"ami-cafeaced",
				}),
			},
		}))
	})
	It("should not set owners when prefixed ids are passed", func() {
		amiSelector := map[string]string{
			"aws::ids": "ami-abcd1234,ami-cafeaced",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(BeNil())
		Expect(filters).Should(ConsistOf([]*ec2.Filter{
			{
				Name: aws.String("image-id"),
				Values: aws.StringSlice([]string{
					"ami-abcd1234",
					"ami-cafeaced",
				}),
			},
		}))
	})
	It("should allow only specifying owners", func() {
		amiSelector := map[string]string{
			"aws::owners": "abcdef,123456789012",
		}
		_, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(ConsistOf(
			[]*string{aws.String("abcdef"), aws.String("123456789012")},
		))
	})
	It("should allow prefixed id, prefixed name, and prefixed owners", func() {
		amiSelector := map[string]string{
			"aws::name":   "my-ami",
			"aws::ids":    "ami-abcd1234,ami-cafeaced",
			"aws::owners": "self,amazon",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(ConsistOf(defaultOwners))
		Expect(filters).Should(ConsistOf([]*ec2.Filter{
			{
				Name:   aws.String("name"),
				Values: aws.StringSlice([]string{"my-ami"}),
			},
			{
				Name: aws.String("image-id"),
				Values: aws.StringSlice([]string{
					"ami-abcd1234",
					"ami-cafeaced",
				}),
			},
		}))
	})
	It("should allow prefixed name and prefixed owners", func() {
		amiSelector := map[string]string{
			"aws::name":   "my-ami",
			"aws::owners": "0123456789,self",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(ConsistOf([]*string{
			aws.String("0123456789"),
			aws.String("self"),
		}))
		Expect(filters).Should(ConsistOf([]*ec2.Filter{
			{
				Name:   aws.String("name"),
				Values: aws.StringSlice([]string{"my-ami"}),
			},
		}))
	})
})
