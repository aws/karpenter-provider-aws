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

package events

import (
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/events"
)

func ProvisionerFailedToResolveNodeTemplate(provisioner *v1alpha5.Provisioner) events.Event {
	return events.Event{
		InvolvedObject: provisioner,
		Type:           v1.EventTypeWarning,
		Message:        "Failed to resolve AWSNodeTemplate",
		DedupeValues:   []string{provisioner.Name},
	}
}

func MachineFailedToResolveNodeTemplate(machine *v1alpha5.Machine) events.Event {
	return events.Event{
		InvolvedObject: machine,
		Type:           v1.EventTypeWarning,
		Message:        "Failed to resolve AWSNodeTemplate",
		DedupeValues:   []string{machine.Name},
	}
}
