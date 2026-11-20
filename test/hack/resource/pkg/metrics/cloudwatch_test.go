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

package metrics

import (
	"context"
	"errors"
	"testing"
)

type fakeClient struct {
	err   error
	fired []string
}

func (f *fakeClient) FireMetric(_ context.Context, _ string, name string, _ float64, _ string) error {
	f.fired = append(f.fired, name)
	return f.err
}

func TestMultiFiresToEveryClientWhenOneFails(t *testing.T) {
	failing, working := &fakeClient{err: errors.New("AccessDenied")}, &fakeClient{}
	err := Multi{failing, working}.FireMetric(context.Background(), "resourceCount", "VPCs", 3, "us-east-2")
	if !errors.Is(err, failing.err) {
		t.Errorf("FireMetric() error = %v, want %v", err, failing.err)
	}
	if len(failing.fired) != 1 || len(working.fired) != 1 {
		t.Errorf("fired %v and %v, want VPCs once to each client", failing.fired, working.fired)
	}
}
