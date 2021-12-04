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
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PodOptions customizes a Pod.
type PodOptions struct {
	Name                      string
	Namespace                 string
	OwnerReferences           []metav1.OwnerReference
	Image                     string
	NodeName                  string
	ResourceRequirements      v1.ResourceRequirements
	NodeSelector              map[string]string
	NodeRequirements          []v1.NodeSelectorRequirement
	NodePreferences           []v1.NodeSelectorRequirement
	TopologySpreadConstraints []v1.TopologySpreadConstraint
	Tolerations               []v1.Toleration
	Conditions                []v1.PodCondition
	Annotations               map[string]string
	Labels                    map[string]string
	Finalizers                []string
}

type PDBOptions struct {
	Name           string
	Namespace      string
	Labels         map[string]string
	MinAvailable   *intstr.IntOrString
	MaxUnavailable *intstr.IntOrString
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
			Labels:          options.Labels,
			Finalizers:      options.Finalizers,
		},
		Spec: v1.PodSpec{
			NodeSelector:              options.NodeSelector,
			Affinity:                  buildAffinity(options.NodeRequirements, options.NodePreferences),
			TopologySpreadConstraints: options.TopologySpreadConstraints,
			Tolerations:               options.Tolerations,
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

// Pods creates homogeneous groups of pods based on the passed in options, evenly divided by the total pods requested
func Pods(total int, options ...PodOptions) []*v1.Pod {
	pods := []*v1.Pod{}
	count := 0
	for _, opts := range options {
		for i := 0; i < total/len(options); i++ {
			opts.Name = fmt.Sprintf("fake-test-pod-%d", count)
			pods = append(pods, Pod(opts))
		}
	}
	return pods
}

// UnschedulablePod creates a test pod with a pending scheduling status condition
func UnschedulablePod(options ...PodOptions) *v1.Pod {
	return Pod(append(options, PodOptions{
		Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
	})...)
}

func PodDisruptionBudget(overrides ...PDBOptions) *v1beta1.PodDisruptionBudget {
	options := PDBOptions{}
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
	return &v1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: options.Namespace,
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MinAvailable: options.MinAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: options.Labels,
			},
			MaxUnavailable: options.MaxUnavailable,
		},
	}
}

func buildAffinity(nodeRequirements []v1.NodeSelectorRequirement, nodePreferences []v1.NodeSelectorRequirement) *v1.Affinity {
	var affinity *v1.Affinity
	if nodeRequirements == nil && nodePreferences == nil {
		return affinity
	}
	affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{}}

	if nodeRequirements != nil {
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{{MatchExpressions: nodeRequirements}},
		}
	}
	if nodePreferences != nil {
		affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []v1.PreferredSchedulingTerm{
			{Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: nodePreferences}},
		}
	}
	return affinity
}
