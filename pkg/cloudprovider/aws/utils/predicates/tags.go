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

// HasTagKey returns a func that returns true if tag exists with tagKey
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
