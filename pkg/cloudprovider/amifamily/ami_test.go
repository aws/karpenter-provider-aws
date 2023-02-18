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
			"aws::name": "my-ami",
			"aws-ids":   "ami-abcd1234,ami-cafeaced",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(BeNil())
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
	It("should not set owners when prefixed ids are passed", func() {
		amiSelector := map[string]string{
			"aws::name": "my-ami",
			"aws::ids":  "ami-abcd1234,ami-cafeaced",
		}
		filters, owners := getFiltersAndOwners(amiSelector)
		Expect(owners).Should(BeNil())
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
	It("should have work with prefixed name and owners", func() {
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
