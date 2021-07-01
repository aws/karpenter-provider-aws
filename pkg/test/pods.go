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

// PodOptions customizes a Pod.
type PodOptions struct {
	Name                 string
	Namespace            string
	OwnerReferences      []metav1.OwnerReference
	Image                string
	NodeName             string
	ResourceRequirements v1.ResourceRequirements
	NodeSelector         map[string]string
	Tolerations          []v1.Toleration
	Conditions           []v1.PodCondition
	Annotations          map[string]string
}

// Pod creates a test pod with defaults that can be overriden by PodOptions.
// Overrides are applied in order, with a last write wins semantic.
func Pod(overrides ...PodOptions) *v1.Pod {
	options := PodOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err.Error()))
		}
	}
	if options.Name == "" {
		options.Name = strings.ToLower(randomdata.SillyName())
	}
	if options.Namespace == "" {
		options.Namespace = "default"
	}
	if options.Image == "" {
		options.Image = "k8s.gcr.io/pause"
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            options.Name,
			Namespace:       options.Namespace,
			OwnerReferences: options.OwnerReferences,
			Annotations:     options.Annotations,
		},
		Spec: v1.PodSpec{
			NodeSelector: options.NodeSelector,
			Tolerations:  options.Tolerations,
			Containers: []v1.Container{{
				Name:      options.Name,
				Image:     options.Image,
				Resources: options.ResourceRequirements,
			}},
			NodeName: options.NodeName,
		},
		Status: v1.PodStatus{Conditions: options.Conditions},
	}
}

// PendingPod creates a test pod with a pending scheduling status condition
func PendingPod(options ...PodOptions) *v1.Pod {
	return Pod(append(options, PodOptions{
		Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
	})...)
}
