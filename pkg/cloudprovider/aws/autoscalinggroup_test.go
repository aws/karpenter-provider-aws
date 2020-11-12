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
	const noErr = false
	const shouldError = true
	var idTests = []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"", "", false},
		{"foo", "foo", false},
		{"arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:autoScalingGroupName/asg-name", "asg-name", noErr},
		{"arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:autoScalingGroupName/", "", noErr},
		{"arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:autoScalingGroupName", "arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:autoScalingGroupName", shouldError},
		{"arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:utoScalingGroupName/asg-name", "arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:utoScalingGroupName/asg-name", shouldError},
		{"arn:aws:autoscalin:region:123456789012:autoScalingGroup:uuid:utoScalingGroupName/asg-name", "arn:aws:autoscalin:region:123456789012:autoScalingGroup:uuid:utoScalingGroupName/asg-name", shouldError},
	}
	for _, idTest := range idTests {
		normalized, err := normalizeID(idTest.input)
		if normalized != idTest.expected {
			t.Errorf("expected: %s, got: %s", idTest.expected, normalized)
		}
		if (err != nil) != idTest.wantErr {
			t.Errorf("expected error: %t, got: %v (input: %s)", idTest.wantErr, err, idTest.input)
		}
	}
}
