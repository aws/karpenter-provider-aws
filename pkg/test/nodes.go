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

package test

import (
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeOptions struct {
	Name          string
	Labels        map[string]string
	Annotations   map[string]string
	ReadyStatus   v1.ConditionStatus
	ReadyReason   string
	Conditions    []v1.NodeCondition
	Unschedulable bool
	Taints        []v1.Taint
	Allocatable   v1.ResourceList
	Finalizers    []string
}

func Node(overrides ...NodeOptions) *v1.Node {
	options := NodeOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge node options: %s", err.Error()))
		}
	}
	if options.Name == "" {
		options.Name = strings.ToLower(randomdata.SillyName())
	}
	if options.ReadyStatus == "" {
		options.ReadyStatus = v1.ConditionTrue
	}
	if options.Labels == nil {
		options.Labels = map[string]string{}
	}
	if options.Annotations == nil {
		options.Annotations = map[string]string{}
	}
	if options.Finalizers == nil {
		options.Finalizers = []string{}
	}
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        options.Name,
			Labels:      options.Labels,
			Annotations: options.Annotations,
			Finalizers:  options.Finalizers,
		},
		Spec: v1.NodeSpec{
			Unschedulable: options.Unschedulable,
			Taints:        options.Taints,
		},
		Status: v1.NodeStatus{
			Allocatable: options.Allocatable,
			Conditions:  []v1.NodeCondition{{Type: v1.NodeReady, Status: options.ReadyStatus, Reason: options.ReadyReason}},
		},
	}
}
