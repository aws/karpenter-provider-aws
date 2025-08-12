/*
Copyright The Kubernetes Authors.

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

	"github.com/imdario/mergo"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PodOptions customizes a Pod.
type PodOptions struct {
	metav1.ObjectMeta
	Image                         string
	NodeName                      string
	Overhead                      v1.ResourceList
	PriorityClassName             string
	InitContainers                []v1.Container
	ResourceRequirements          v1.ResourceRequirements
	NodeSelector                  map[string]string
	NodeRequirements              []v1.NodeSelectorRequirement
	NodePreferences               []v1.NodeSelectorRequirement
	PodRequirements               []v1.PodAffinityTerm
	PodPreferences                []v1.WeightedPodAffinityTerm
	PodAntiRequirements           []v1.PodAffinityTerm
	PodAntiPreferences            []v1.WeightedPodAffinityTerm
	TopologySpreadConstraints     []v1.TopologySpreadConstraint
	Tolerations                   []v1.Toleration
	PersistentVolumeClaims        []string
	EphemeralVolumeTemplates      []EphemeralVolumeTemplateOptions
	HostPorts                     []int32
	Conditions                    []v1.PodCondition
	Phase                         v1.PodPhase
	RestartPolicy                 v1.RestartPolicy
	TerminationGracePeriodSeconds *int64
	ReadinessProbe                *v1.Probe
	LivenessProbe                 *v1.Probe
	PreStopSleep                  *int64
	Command                       []string
}

type PDBOptions struct {
	metav1.ObjectMeta
	Labels                     map[string]string
	MinAvailable               *intstr.IntOrString
	MaxUnavailable             *intstr.IntOrString
	UnhealthyPodEvictionPolicy *policyv1.UnhealthyPodEvictionPolicyType
	Status                     *policyv1.PodDisruptionBudgetStatus
}

type EphemeralVolumeTemplateOptions struct {
	StorageClassName *string
}

var (
	DefaultImage        = "public.ecr.aws/eks-distro/kubernetes/pause:3.2"
	KWOKDelayAnnotation = "pod-delete.stage.kwok.x-k8s.io/delay"
)

// Pod creates a test pod with defaults that can be overridden by PodOptions.
// Overrides are applied in order, with a last write wins semantic.
// nolint:gocyclo
func Pod(overrides ...PodOptions) *v1.Pod {
	options := PodOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}
	if options.Image == "" {
		options.Image = DefaultImage
	}
	var volumes []v1.Volume
	for _, pvc := range options.PersistentVolumeClaims {
		volumes = append(volumes, v1.Volume{
			Name:         RandomName(),
			VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvc}},
		})
	}
	for _, evt := range options.EphemeralVolumeTemplates {
		volumes = append(volumes, v1.Volume{
			Name: RandomName(),
			VolumeSource: v1.VolumeSource{
				Ephemeral: &v1.EphemeralVolumeSource{
					VolumeClaimTemplate: &v1.PersistentVolumeClaimTemplate{
						Spec: v1.PersistentVolumeClaimSpec{
							AccessModes: []v1.PersistentVolumeAccessMode{
								v1.ReadWriteOnce,
							},
							Resources: v1.VolumeResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
							StorageClassName: evt.StorageClassName,
						},
					},
				},
			},
		})
	}

	p := &v1.Pod{
		ObjectMeta: NamespacedObjectMeta(options.ObjectMeta),
		Spec: v1.PodSpec{
			NodeSelector:              options.NodeSelector,
			Affinity:                  buildAffinity(options),
			TopologySpreadConstraints: options.TopologySpreadConstraints,
			Tolerations:               options.Tolerations,
			Containers: []v1.Container{{
				Name:      RandomName(),
				Image:     options.Image,
				Resources: options.ResourceRequirements,
				Ports: lo.Map(options.HostPorts, func(p int32, _ int) v1.ContainerPort {
					return v1.ContainerPort{
						HostPort:      p,
						Protocol:      v1.ProtocolTCP,
						ContainerPort: int32(80),
					}
				}),
				ReadinessProbe: options.ReadinessProbe,
				LivenessProbe:  options.LivenessProbe,
			}},
			NodeName:                      options.NodeName,
			Volumes:                       volumes,
			PriorityClassName:             options.PriorityClassName,
			RestartPolicy:                 options.RestartPolicy,
			TerminationGracePeriodSeconds: options.TerminationGracePeriodSeconds,
		},
		Status: v1.PodStatus{
			Conditions: options.Conditions,
			Phase:      options.Phase,
		},
	}
	// If PreStopSleep is enabled, add it to each of the containers.
	// Can't use v1.LifecycleHandler == v1.SleepAction as that's a feature gate in Alpha 1.29.
	// https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#hook-handler-implementations
	if options.PreStopSleep != nil {
		p.ObjectMeta.Annotations = lo.Assign(p.Annotations, map[string]string{
			KWOKDelayAnnotation: fmt.Sprintf("%ds", lo.FromPtr(options.PreStopSleep)),
		})
		p.Spec.Containers[0].Lifecycle = &v1.Lifecycle{
			PreStop: &v1.LifecycleHandler{
				Exec: &v1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-c",
						fmt.Sprintf("sleep %d", lo.FromPtr(options.PreStopSleep)),
					},
				},
			},
		}
	}
	if options.Command != nil {
		p.Spec.Containers[0].Command = options.Command
	}
	if options.Overhead != nil {
		p.Spec.Overhead = options.Overhead
	}
	if options.InitContainers != nil {
		for _, init := range options.InitContainers {
			init.Name = RandomName()
			if init.Image == "" {
				init.Image = DefaultImage
			}
			p.Spec.InitContainers = append(p.Spec.InitContainers, init)
		}
	}
	return p
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

func UnscheduleablePodOptions(overrides ...PodOptions) PodOptions {
	options := PodOptions{Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}}}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}
	return options
}

// UnschedulablePod creates a test pod with a pending scheduling status condition
func UnschedulablePod(options ...PodOptions) *v1.Pod {
	return Pod(append(options, PodOptions{
		Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
	})...)
}

// UnschedulablePods returns slice of configurable length of identical test pods with a pending scheduling status condition
func UnschedulablePods(options PodOptions, num int) []*v1.Pod {
	var pods []*v1.Pod
	for i := 0; i < num; i++ {
		pods = append(pods, UnschedulablePod(options))
	}
	return pods
}

// PodDisruptionBudget creates a PodDisruptionBudget.  To function properly, it should have its status applied
func PodDisruptionBudget(overrides ...PDBOptions) *policyv1.PodDisruptionBudget {
	options := PDBOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pdb options: %s", err))
		}
	}
	status := policyv1.PodDisruptionBudgetStatus{
		// To be considered for application by eviction, the Status.ObservedGeneration must be >= the PDB generation.
		// kube-controller-manager normally sets ObservedGeneration, but we don't have one when running under
		// EnvTest. If this isn't modified the eviction controller assumes that the PDB hasn't been processed
		// by the disruption controller yet and adds a 10 second retry to our evict() call
		ObservedGeneration: 1,
	}
	if options.Status != nil {
		status = *options.Status
	}

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: NamespacedObjectMeta(options.ObjectMeta),
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: options.MinAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: options.Labels,
			},
			MaxUnavailable:             options.MaxUnavailable,
			UnhealthyPodEvictionPolicy: options.UnhealthyPodEvictionPolicy,
		},
		Status: status,
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

func MakePodAntiAffinityPodOptions(key string) PodOptions {
	// all of these pods have anti-affinity to each other
	labels := map[string]string{
		"app": "nginx",
	}
	return PodOptions{
		ObjectMeta: metav1.ObjectMeta{Labels: lo.Assign(labels, map[string]string{DiscoveryLabel: "owned"})},
		PodAntiRequirements: []v1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
				TopologyKey:   key,
			},
		},
		ResourceRequirements: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    RandomCPU(),
				v1.ResourceMemory: RandomMemory(),
			},
		}}
}
func MakePodAffinityPodOptions(key string) PodOptions {
	affinityLabels := RandomAffinityLabels()
	return PodOptions{
		ObjectMeta: metav1.ObjectMeta{Labels: lo.Assign(affinityLabels, map[string]string{DiscoveryLabel: "owned"})},
		PodPreferences: []v1.WeightedPodAffinityTerm{
			{
				Weight: 1,
				PodAffinityTerm: v1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{MatchLabels: affinityLabels},
					TopologyKey:   key,
				},
			},
		},
		ResourceRequirements: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    RandomCPU(),
				v1.ResourceMemory: RandomMemory(),
			},
		}}
}

func MakeTopologySpreadPodOptions(key string) PodOptions {
	return PodOptions{
		ObjectMeta: metav1.ObjectMeta{Labels: lo.Assign(RandomLabels(), map[string]string{DiscoveryLabel: "owned"})},
		TopologySpreadConstraints: []v1.TopologySpreadConstraint{
			{
				MaxSkew:           1,
				TopologyKey:       key,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: RandomLabels(),
				},
			},
		},
		ResourceRequirements: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    RandomCPU(),
				v1.ResourceMemory: RandomMemory(),
			},
		}}
}

func MakeGenericPodOptions() PodOptions {
	return PodOptions{
		ObjectMeta: metav1.ObjectMeta{Labels: lo.Assign(RandomLabels(), map[string]string{DiscoveryLabel: "owned"})},
		ResourceRequirements: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    RandomCPU(),
				v1.ResourceMemory: RandomMemory(),
			},
		}}
}

func MakeDiversePodOptions() []PodOptions {
	var pods []PodOptions
	pods = append(pods, MakeGenericPodOptions())
	pods = append(pods, MakeTopologySpreadPodOptions(v1.LabelTopologyZone))
	pods = append(pods, MakeTopologySpreadPodOptions(v1.LabelHostname))
	pods = append(pods, MakePodAffinityPodOptions(v1.LabelHostname))
	pods = append(pods, MakePodAffinityPodOptions(v1.LabelTopologyZone))
	pods = append(pods, MakePodAntiAffinityPodOptions(v1.LabelHostname))
	return pods
}

func RandomAffinityLabels() map[string]string {
	return map[string]string{
		"my-affinity": RandomLabelValue(),
	}
}

func RandomLabels() map[string]string {
	return map[string]string{
		"my-label": RandomLabelValue(),
	}
}

//nolint:gosec
var r = rand.New(rand.NewSource(42))

func RandomLabelValue() string {
	labelValues := []string{"a", "b", "c", "d", "e", "f", "g"}
	return labelValues[r.Intn(len(labelValues))]
}

func RandomMemory() resource.Quantity {
	mem := []int{100, 256, 512, 1024, 2048, 4096}
	return resource.MustParse(fmt.Sprintf("%dMi", mem[r.Intn(len(mem))]))
}

func RandomCPU() resource.Quantity {
	cpu := []int{100, 250, 500, 1000, 1500}
	return resource.MustParse(fmt.Sprintf("%dm", cpu[r.Intn(len(cpu))]))
}
