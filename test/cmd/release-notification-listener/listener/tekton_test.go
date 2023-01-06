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

package listener

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/sqs"
	"strings"
	"testing"
)

func TestTknArgs(t *testing.T) {
	emptyJSON := "{}"
	msg, err := newNotificationMessage(&sqs.Message{Body: &emptyJSON})
	if err != nil {
		t.Fatalf("unexpected error. %s", err)
	}

	if msg.PrNumber != noPrNumber {
		t.Fatalf("want %s got %s", noPrNumber, msg.PrNumber)
	}
	msg.ReleaseIdentifier = "abcd"

	msg2, err := newNotificationMessage(&sqs.Message{Body: &emptyJSON})
	if err != nil {
		t.Fatalf("unexpected error. %s", err)
	}
	msg2.PrNumber = "123"
	msg2.ReleaseIdentifier = "abcd"

	var tests = []struct {
		msg               *notificationMessage
		pipelineName      string
		testFilter        string
		wantArgsToContain []string
	}{
		{msg, "foo", "bar", []string{"test-filter=bar", "--prefix-name=bar-abcd"}},
		{msg, "foo", "", []string{"test-filter=", "--prefix-name=foo-abcd"}},
		{msg2, "foo", "bar", []string{"test-filter=bar", "--prefix-name=bar-pr-123"}},
		{msg2, "foo", "", []string{"test-filter=", "--prefix-name=foo-pr-123"}},
	}

	for i, test := range tests {
		args, err := tknArgs(test.msg, test.pipelineName, test.testFilter)
		if err != nil {
			t.Fatalf("test #%d unexpected error. %s", i, err)
		}
		argsStr := fmt.Sprintf("%v", args)
		for _, want := range test.wantArgsToContain {
			if !strings.Contains(argsStr, want) {
				t.Fatalf("test #%d expected %s to contain %s", i, argsStr, want)
			}
		}
	}
}
