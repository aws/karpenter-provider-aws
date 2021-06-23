package predicates

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// HasNameTag returns a func that returns true if name tag matches name
func HasNameTag(name string) func([]*ec2.Tag) bool {
	return func(tags []*ec2.Tag) bool {
		for _, tag := range tags {
			if aws.StringValue(tag.Key) == "Name" {
				return aws.StringValue(tag.Value) == name
			}
		}
		return false
	}
}

// HasNameTag returns a func that returns true if tag exists with tagKey
func HasTagKey(tagKey string) func([]*ec2.Tag) bool {
	return func(tags []*ec2.Tag) bool {
		for _, tag := range tags {
			if aws.StringValue(tag.Key) == tagKey {
				return true
			}
		}
		return false
	}
}
