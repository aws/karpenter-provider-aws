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

package fake

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// TestSSMAPIConcurrentGetParameterAndReset exercises the default-parameter caching path of GetParameter concurrently
// with Reset. Before the fix, SSMAPI had no mutex: GetParameter performed an unsynchronized check-then-act on the
// defaultParameters map while Reset reassigned it, which is a data race and can trigger a "concurrent map read and map
// write" fatal runtime panic. Run with -race to observe the data race.
func TestSSMAPIConcurrentGetParameterAndReset(t *testing.T) {
	api := NewSSMAPI()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range 200 {
		// Reuse a small set of names so multiple goroutines contend on the same defaultParameters entries,
		// maximizing the chance of a concurrent read/write against the shared map.
		name := fmt.Sprintf("param-%d", i%4)
		wg.Add(3)
		go func() {
			defer wg.Done()
			_, _ = api.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(name)})
		}()
		go func() {
			defer wg.Done()
			_, _ = api.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(name)})
		}()
		go func() {
			defer wg.Done()
			api.Reset()
		}()
	}
	wg.Wait()
}
