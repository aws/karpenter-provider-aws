package amifamily

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/stretchr/testify/assert"
)

var (
	defaultOwners = []*string{aws.String("self"), aws.String("amazon")}
)

func TestNoOwnersNoPrefix(t *testing.T) {
	amiSelector := map[string]string{
		"Name": "my-ami",
	}
	filters, owners := getFiltersAndOwners(amiSelector)
	assert.ElementsMatch(t, owners, defaultOwners)
	assert.ElementsMatch(t, filters, []*ec2.Filter{
		{
			Name:   aws.String("tag:Name"),
			Values: aws.StringSlice([]string{"my-ami"}),
		},
	})
}

func TestNoOwnersPrefixedName(t *testing.T) {
	amiSelector := map[string]string{
		"aws::name": "my-ami",
	}
	filters, owners := getFiltersAndOwners(amiSelector)
	assert.ElementsMatch(t, owners, defaultOwners)
	assert.ElementsMatch(t, filters, []*ec2.Filter{
		{
			Name:   aws.String("name"),
			Values: aws.StringSlice([]string{"my-ami"}),
		},
	})
}

func TestLegacyIdsPrefixedName(t *testing.T) {
	amiSelector := map[string]string{
		"aws::name": "my-ami",
		"aws-ids":   "ami-abcd1234,ami-cafeaced",
	}
	filters, owners := getFiltersAndOwners(amiSelector)
	assert.ElementsMatch(t, owners, nil)
	assert.ElementsMatch(t, filters, []*ec2.Filter{
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
	})
}

func TestIdsPrefixedName(t *testing.T) {
	amiSelector := map[string]string{
		"aws::name": "my-ami",
		"aws::ids":  "ami-abcd1234,ami-cafeaced",
	}
	filters, owners := getFiltersAndOwners(amiSelector)
	assert.ElementsMatch(t, owners, nil)
	assert.ElementsMatch(t, filters, []*ec2.Filter{
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
	})
}

func TestIdsPrefixedNameWithOwners(t *testing.T) {
	amiSelector := map[string]string{
		"aws::name":   "my-ami",
		"aws::ids":    "ami-abcd1234,ami-cafeaced",
		"aws::owners": "self,amazon",
	}
	filters, owners := getFiltersAndOwners(amiSelector)
	assert.ElementsMatch(t, owners, defaultOwners)
	assert.ElementsMatch(t, filters, []*ec2.Filter{
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
	})
}

func TestOwnersPrefixedName(t *testing.T) {
	amiSelector := map[string]string{
		"aws::name":   "my-ami",
		"aws::owners": "0123456789,self",
	}
	filters, owners := getFiltersAndOwners(amiSelector)
	assert.ElementsMatch(t, owners, []*string{
		aws.String("0123456789"),
		aws.String("self"),
	})
	assert.ElementsMatch(t, filters, []*ec2.Filter{
		{
			Name:   aws.String("name"),
			Values: aws.StringSlice([]string{"my-ami"}),
		},
	})
}
