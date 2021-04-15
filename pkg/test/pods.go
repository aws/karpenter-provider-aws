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
	Name             string
	Namespace        string
	Image            string
	NodeName         string
	ResourceRequests v1.ResourceList
	NodeSelector     map[string]string
	Tolerations      []v1.Toleration
	Conditions       []v1.PodCondition
}

func defaults(options PodOptions) *v1.Pod {
	if options.Name == "" {
		options.Name = strings.ToLower(randomdata.SillyName())
	}
	if options.Namespace == "" {
		options.Namespace = "default"
	}
	if options.Image == "" {
		options.Image = "k8s.gcr.io/pause"
	}
	if len(options.Conditions) == 0 {
		options.Conditions = []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}}
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: options.Namespace,
		},
		Spec: v1.PodSpec{
			NodeSelector: options.NodeSelector,
			Tolerations:  options.Tolerations,
			Containers: []v1.Container{{
				Name:  options.Name,
				Image: options.Image,
				Resources: v1.ResourceRequirements{
					Requests: options.ResourceRequests,
				},
			}},
			NodeName: options.NodeName,
		},
		Status: v1.PodStatus{Conditions: options.Conditions},
	}
}

// PendingPod creates a pending test pod with the minimal set of other
// fields defaulted to something sane.
func PendingPod() *v1.Pod {
	return defaults(PodOptions{})
}

// PendingPodWith creates a pending test pod with fields overridden by
// options.
func PendingPodWith(options PodOptions) *v1.Pod {
	return PodWith(PendingPod(), options)
}

// PodWith overrides, in-place, pod with any non-zero elements of
// options. It returns the same pod simply for ease of use.
func PodWith(pod *v1.Pod, options PodOptions) *v1.Pod {
	if err := mergo.Merge(pod, defaults(options), mergo.WithOverride); err != nil {
		panic(fmt.Sprintf("unexpected error in test code: %v", err))
	}
	return pod
}
