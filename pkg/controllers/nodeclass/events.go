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

package nodeclass

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/utils"
)

func WaitingOnInstanceTerminationEvent(nodeClass *v1beta1.EC2NodeClass, profileName string, ids []string) events.Event {
	return events.Event{
		InvolvedObject: nodeClass,
		Type:           v1.EventTypeNormal,
		Reason:         "WaitingOnInstanceTermination",
		Message:        fmt.Sprintf("Waiting on instance termination for instances %s using profile %q", utils.PrettySlice(ids, 5), profileName),
		DedupeValues:   []string{string(nodeClass.UID)},
	}
}
