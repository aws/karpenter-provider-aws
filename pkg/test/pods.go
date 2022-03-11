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
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PodOptions customizes a Pod.
type PodOptions struct {
	metav1.ObjectMeta
	Image                     string
	NodeName                  string
	PriorityClassName         string
	ResourceRequirements      v1.ResourceRequirements
	NodeSelector              map[string]string
	NodeRequirements          []v1.NodeSelectorRequirement
	NodePreferences           []v1.NodeSelectorRequirement
	PodRequirements           []v1.PodAffinityTerm
	PodPreferences            []v1.WeightedPodAffinityTerm
	PodAntiRequirements       []v1.PodAffinityTerm
	PodAntiPreferences        []v1.WeightedPodAffinityTerm
	TopologySpreadConstraints []v1.TopologySpreadConstraint
	Tolerations               []v1.Toleration
	PersistentVolumeClaims    []string
	Conditions                []v1.PodCondition
	Phase                     v1.PodPhase
}

type PDBOptions struct {
	metav1.ObjectMeta
	Labels         map[string]string
	MinAvailable   *intstr.IntOrString
	MaxUnavailable *intstr.IntOrString
}

var (
	sequentialPodNumber     = 0
	randomSource            = rand.NewSource(time.Now().UnixNano())
	randomizer              = rand.New(randomSource) //nolint
	sequentialPodNumberLock = new(sync.Mutex)
)

// Pod creates a test pod with defaults that can be overridden by PodOptions.
// Overrides are applied in order, with a last write wins semantic.
func Pod(overrides ...PodOptions) *v1.Pod {
	options := PodOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}
	if options.Image == "" {
		options.Image = "k8s.gcr.io/pause"
	}
	volumes := []v1.Volume{}
	for _, pvc := range options.PersistentVolumeClaims {
		volumes = append(volumes, v1.Volume{
			Name:         strings.ToLower(randomdata.SillyName()),
			VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvc}},
		})
	}
	return &v1.Pod{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: v1.PodSpec{
			NodeSelector:              options.NodeSelector,
			Affinity:                  buildAffinity(options),
			TopologySpreadConstraints: options.TopologySpreadConstraints,
			Tolerations:               options.Tolerations,
			Containers: []v1.Container{{
				Name:      strings.ToLower(sequentialRandomName()),
				Image:     options.Image,
				Resources: options.ResourceRequirements,
			}},
			NodeName:          options.NodeName,
			Volumes:           volumes,
			PriorityClassName: options.PriorityClassName,
		},
		Status: v1.PodStatus{
			Conditions: options.Conditions,
			Phase:      options.Phase,
		},
	}
}

func sequentialRandomName() string {
	sequentialPodNumberLock.Lock()
	defer sequentialPodNumberLock.Unlock()
	sequentialPodNumber++
	return fmt.Sprintf("P%04d-%s-%06d", sequentialPodNumber, randomdata.SillyName(), randomizer.Intn(99999))
}

// Pods creates homogeneous groups of pods based on the passed in options, evenly divided by the total pods requested
func Pods(total int, options ...PodOptions) []*v1.Pod {
	pods := []*v1.Pod{}
	for _, opts := range options {
		for i := 0; i < total/len(options); i++ {
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

// PodDisruptionBudget creates a PodDisruptionBudget.  To function properly, it should have its status applied
// after creation with something like ExpectCreatedWithStatus
func PodDisruptionBudget(overrides ...PDBOptions) *v1beta1.PodDisruptionBudget {
	options := PDBOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}
	return &v1beta1.PodDisruptionBudget{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MinAvailable: options.MinAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: options.Labels,
			},
			MaxUnavailable: options.MaxUnavailable,
		},
		Status: v1beta1.PodDisruptionBudgetStatus{
			// To be considered for application by eviction, the Status.ObservedGeneration must be >= the PDB generation.
			// kube-controller-manager normally sets ObservedGeneration, but we don't have one when running under
			// EnvTest. If this isn't modified the eviction controller assumes that the PDB hasn't been processed
			// by the disruption controller yet and adds a 10 second retry to our evict() call
			ObservedGeneration: 1,
		},
	}
}

func buildAffinity(options PodOptions) *v1.Affinity {
	affinity := &v1.Affinity{}
	if nodeAffinity := buildNodeAffinity(options.NodeRequirements, options.NodePreferences); nodeAffinity != nil {
		affinity.NodeAffinity = nodeAffinity
	}
	if podAffinity := buildPodAffinity(options.PodRequirements, options.PodPreferences); podAffinity != nil {
		affinity.PodAffinity = podAffinity
	}
	if podAntiAffinity := buildPodAntiAffinity(options.PodAntiRequirements, options.PodAntiPreferences); podAntiAffinity != nil {
		affinity.PodAntiAffinity = podAntiAffinity
	}
	if affinity.NodeAffinity == nil && affinity.PodAffinity == nil && affinity.PodAntiAffinity == nil {
		return nil
	}
	return affinity
}

func buildPodAffinity(podRequirements []v1.PodAffinityTerm, podPreferences []v1.WeightedPodAffinityTerm) *v1.PodAffinity {
	var podAffinity *v1.PodAffinity
	if podRequirements == nil && podPreferences == nil {
		return podAffinity
	}
	podAffinity = &v1.PodAffinity{}

	if podRequirements != nil {
		podAffinity.RequiredDuringSchedulingIgnoredDuringExecution = podRequirements
	}
	if podPreferences != nil {
		podAffinity.PreferredDuringSchedulingIgnoredDuringExecution = podPreferences
	}
	return podAffinity
}

func buildPodAntiAffinity(podAntiRequirements []v1.PodAffinityTerm, podAntiPreferences []v1.WeightedPodAffinityTerm) *v1.PodAntiAffinity {
	var podAntiAffinity *v1.PodAntiAffinity
	if podAntiRequirements == nil && podAntiPreferences == nil {
		return podAntiAffinity
	}
	podAntiAffinity = &v1.PodAntiAffinity{}

	if podAntiRequirements != nil {
		podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = podAntiRequirements
	}
	if podAntiPreferences != nil {
		podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = podAntiPreferences
	}
	return podAntiAffinity
}

func buildNodeAffinity(nodeRequirements []v1.NodeSelectorRequirement, nodePreferences []v1.NodeSelectorRequirement) *v1.NodeAffinity {
	var nodeAffinity *v1.NodeAffinity
	if nodeRequirements == nil && nodePreferences == nil {
		return nodeAffinity
	}
	nodeAffinity = &v1.NodeAffinity{}

	if nodeRequirements != nil {
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{{MatchExpressions: nodeRequirements}},
		}
	}
	if nodePreferences != nil {
		nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []v1.PreferredSchedulingTerm{
			{Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: nodePreferences}},
		}
	}
	return nodeAffinity
}
