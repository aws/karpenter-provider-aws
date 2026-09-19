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
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// TestIAMAPICreateInstanceProfileReturnsCopy ensures CreateInstanceProfile returns an object that is independent from
// the one retained in the fake's internal map. Before the fix, the same pointer was returned, so a caller mutating the
// returned profile (as the instanceprofile provider does) silently mutated the fake's internal state.
func TestIAMAPICreateInstanceProfileReturnsCopy(t *testing.T) {
	api := NewIAMAPI()
	const name = "test-profile"

	out, err := api.CreateInstanceProfile(context.Background(), &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(name),
	})
	if err != nil {
		t.Fatalf("CreateInstanceProfile: unexpected error: %v", err)
	}

	// Mutate the returned object the way the provider's Create does.
	out.InstanceProfile.Roles = []iamtypes.Role{{RoleName: aws.String("some-role")}}

	if got := len(api.InstanceProfiles[name].Roles); got != 0 {
		t.Fatalf("mutating the returned instance profile leaked into internal state: internal Roles len = %d, want 0", got)
	}
}

// TestIAMAPIGetInstanceProfileReturnsCopy ensures GetInstanceProfile returns an object independent from the internally
// stored one.
func TestIAMAPIGetInstanceProfileReturnsCopy(t *testing.T) {
	api := NewIAMAPI()
	const name = "test-profile"

	if _, err := api.CreateInstanceProfile(context.Background(), &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(name),
	}); err != nil {
		t.Fatalf("CreateInstanceProfile: unexpected error: %v", err)
	}

	out, err := api.GetInstanceProfile(context.Background(), &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(name),
	})
	if err != nil {
		t.Fatalf("GetInstanceProfile: unexpected error: %v", err)
	}

	out.InstanceProfile.Roles = append(out.InstanceProfile.Roles, iamtypes.Role{RoleName: aws.String("some-role")})

	if got := len(api.InstanceProfiles[name].Roles); got != 0 {
		t.Fatalf("mutating the returned instance profile leaked into internal state: internal Roles len = %d, want 0", got)
	}
}

// TestIAMAPIConcurrentResetAndAccess exercises Reset concurrently with the locked API methods. Before the fix, Reset
// reassigned the InstanceProfiles/Roles maps without holding the mutex, racing with (and potentially panicking against)
// the concurrent map access performed under lock by the other methods. Run with -race to observe the data race.
func TestIAMAPIConcurrentResetAndAccess(t *testing.T) {
	api := NewIAMAPI()
	ctx := context.Background()
	const name = "p"

	var wg sync.WaitGroup
	for range 200 {
		wg.Add(5)
		go func() { defer wg.Done(); api.Reset() }()
		go func() {
			defer wg.Done()
			_, _ = api.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{InstanceProfileName: aws.String(name)})
		}()
		go func() {
			defer wg.Done()
			_, _ = api.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: aws.String(name)})
		}()
		go func() {
			defer wg.Done()
			_, _ = api.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
				InstanceProfileName: aws.String(name),
				RoleName:            aws.String("r"),
			})
		}()
		go func() {
			defer wg.Done()
			_, _ = api.ListInstanceProfiles(ctx, &iam.ListInstanceProfilesInput{PathPrefix: aws.String("/")})
		}()
	}
	wg.Wait()
}
