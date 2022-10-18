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

package notification

import (
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/events"
)

type EC2SpotInterruptionWarning struct {
	n *v1.Node
}

func NewEC2SpotInterruptionWarning(n *v1.Node) EC2SpotInterruptionWarning {
	return EC2SpotInterruptionWarning{
		n: n,
	}
}

func (e EC2SpotInterruptionWarning) InvolvedObject() runtime.Object {
	return e.n
}

func (EC2SpotInterruptionWarning) Type() events.EventType {
	return events.WarningType
}

func (e EC2SpotInterruptionWarning) Reason() string {
	return reflect.TypeOf(e).Name()
}

func (e EC2SpotInterruptionWarning) Message() string {
	return fmt.Sprintf("Node %s event: EC2 triggered a spot interruption warning for the node", e.n.Name)
}

type EC2SpotRebalanceRecommendation struct {
	n *v1.Node
}

func NewEC2SpotRebalanceRecommendation(n *v1.Node) EC2SpotRebalanceRecommendation {
	return EC2SpotRebalanceRecommendation{
		n: n,
	}
}

func (e EC2SpotRebalanceRecommendation) InvolvedObject() runtime.Object {
	return e.n
}

func (EC2SpotRebalanceRecommendation) Type() events.EventType {
	return events.NormalType
}

func (e EC2SpotRebalanceRecommendation) Reason() string {
	return reflect.TypeOf(e).Name()
}

func (e EC2SpotRebalanceRecommendation) Message() string {
	return fmt.Sprintf("Node %s event: EC2 triggered a spot rebalance recommendation for the node", e.n.Name)
}

type EC2HealthWarning struct {
	n *v1.Node
}

func NewEC2HealthWarning(n *v1.Node) EC2HealthWarning {
	return EC2HealthWarning{
		n: n,
	}
}

func (e EC2HealthWarning) InvolvedObject() runtime.Object {
	return e.n
}

func (EC2HealthWarning) Type() events.EventType {
	return events.WarningType
}

func (e EC2HealthWarning) Reason() string {
	return reflect.TypeOf(e).Name()
}

func (e EC2HealthWarning) Message() string {
	return fmt.Sprintf("Node %s event: EC2 triggered a health warning for the node", e.n.Name)
}

type EC2StateTerminating struct {
	n *v1.Node
}

func NewEC2StateTerminating(n *v1.Node) EC2StateTerminating {
	return EC2StateTerminating{
		n: n,
	}
}

func (e EC2StateTerminating) InvolvedObject() runtime.Object {
	return e.n
}

func (EC2StateTerminating) Type() events.EventType {
	return events.WarningType
}

func (e EC2StateTerminating) Reason() string {
	return reflect.TypeOf(e).Name()
}

func (e EC2StateTerminating) Message() string {
	return fmt.Sprintf("Node %s event: EC2 node is terminating", e.n.Name)
}

type EC2StateStopping struct {
	n *v1.Node
}

func NewEC2StateStopping(n *v1.Node) EC2StateStopping {
	return EC2StateStopping{
		n: n,
	}
}

func (e EC2StateStopping) InvolvedObject() runtime.Object {
	return e.n
}

func (EC2StateStopping) Type() events.EventType {
	return events.WarningType
}

func (e EC2StateStopping) Reason() string {
	return reflect.TypeOf(e).Name()
}

func (e EC2StateStopping) Message() string {
	return fmt.Sprintf("Node %s event: EC2 node is stopping", e.n.Name)
}

type TerminatingNodeOnNotification struct {
	n *v1.Node
}

func NewTerminatingNodeOnNotification(n *v1.Node) TerminatingNodeOnNotification {
	return TerminatingNodeOnNotification{
		n: n,
	}
}

func (e TerminatingNodeOnNotification) InvolvedObject() runtime.Object {
	return e.n
}

func (TerminatingNodeOnNotification) Type() events.EventType {
	return events.WarningType
}

func (e TerminatingNodeOnNotification) Reason() string {
	return reflect.TypeOf(e).Name()
}

func (e TerminatingNodeOnNotification) Message() string {
	return fmt.Sprintf("Node %s event: EC2 node is stopping", e.n.Name)
}
