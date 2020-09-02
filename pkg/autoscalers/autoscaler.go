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

package autoscalers

import (
	"github.com/ellistarn/karpenter/pkg/metrics"
)

// Autoscaler defines an interface for autoscaler all implementations
// These algorithms are black boxes and may use different strategies to compute desired replicas.
type Autoscaler interface {
	GetDesiredReplicas(metrics []metrics.ObservedMetric, replicas int32) int32
}
