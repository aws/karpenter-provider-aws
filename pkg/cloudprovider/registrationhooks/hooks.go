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

package registrationhooks

import (
	"context"

	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
)

// AWSRegistrationHook serializes AWS label updates because core evaluates registration hooks in parallel.
// Placement and reservation hooks both mutate the NodeClaim's label map, so they must share a single core hook.
// The first pending hook determines the retry interval; later hooks run after it completes.
type AWSRegistrationHook struct {
	hooks []cloudprovider.NodeLifecycleHook
}

func NewAWSRegistrationHook(provider instance.Provider) *AWSRegistrationHook {
	return &AWSRegistrationHook{hooks: []cloudprovider.NodeLifecycleHook{
		NewPlacementGroupRegistrationHook(provider),
		NewCapacityReservationGroupRegistrationHook(provider),
	}}
}

func (*AWSRegistrationHook) Name() string { return "AWSRegistrationHook" }

func (h *AWSRegistrationHook) Registered(ctx context.Context, nc *karpv1.NodeClaim) (cloudprovider.NodeLifecycleHookResult, error) {
	for _, hook := range h.hooks {
		result, err := hook.Registered(ctx, nc)
		if err != nil || !lo.IsEmpty(result) {
			return result, err
		}
	}
	return cloudprovider.NodeLifecycleHookResult{}, nil
}
