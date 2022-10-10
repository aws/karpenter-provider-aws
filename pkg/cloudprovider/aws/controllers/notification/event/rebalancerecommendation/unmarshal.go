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

package rebalancerecommendation

import (
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

// AWSEvent contains the properties defined by
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html#monitor-rebalance-recommendations
type AWSEvent struct {
	event.AWSMetadata

	Detail EC2InstanceRebalanceRecommendationDetail `json:"detail"`
}

type EC2InstanceRebalanceRecommendationDetail struct {
	InstanceID string `json:"instance-id"`
}
