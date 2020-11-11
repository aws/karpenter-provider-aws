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

package aws

import (
	"testing"
)

func TestNormalizeID(t *testing.T) {
	var idTests = []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"foo", "foo"},
		{"arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:autoScalingGroupName/asg-name", "asg-name"},
		{"arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:utoScalingGroupName/asg-name", "arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:utoScalingGroupName/asg-name"},
	}
	for _, idTest := range idTests {
		normalized := normalizeID(idTest.input)
		if normalized != idTest.expected {
			t.Errorf("expected: %s, got: %s", idTest.expected, normalized)
		}
	}
}
