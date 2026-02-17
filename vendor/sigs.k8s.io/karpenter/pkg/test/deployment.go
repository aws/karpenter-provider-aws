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

	"github.com/imdario/mergo"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	karpenterv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

type DeploymentOptions struct {
	metav1.ObjectMeta
	Labels     map[string]string
	Replicas   int32
	PodOptions PodOptions
}

func Deployment(overrides ...DeploymentOptions) *appsv1.Deployment {
	options := DeploymentOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge deployment options: %s", err))
		}
	}

	objectMeta := NamespacedObjectMeta(options.ObjectMeta)

	if options.PodOptions.Image == "" {
		options.PodOptions.Image = DefaultImage
	}
	if options.PodOptions.Labels == nil {
		options.PodOptions.Labels = map[string]string{
			"app": objectMeta.Name,
		}
	}
	pod := Pod(options.PodOptions)
	return &appsv1.Deployment{
		ObjectMeta: objectMeta,
		Spec: appsv1.DeploymentSpec{
			Replicas: lo.ToPtr(options.Replicas),
			Selector: &metav1.LabelSelector{MatchLabels: options.PodOptions.Labels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			},
		},
	}
}

// DeploymentOptionModifier is a function that modifies DeploymentOptions
type DeploymentOptionModifier func(*DeploymentOptions)

// CreateDeploymentOptions creates a test.DeploymentOptions with the specified parameters
// and applies any provided modifiers. This provides a clean, extensible way to create
// deployment configurations for tests.
//
// Example usage:
//
//	// Simple deployment
//	opts := CreateDeploymentOptions("my-app", 10, "100m", "128Mi")
//
//	// With modifiers
//	opts := CreateDeploymentOptions("my-app", 10, "100m", "128Mi",
//	    WithHostnameSpread(),
//	    WithDoNotDisrupt(),
//	    WithLabels(map[string]string{"tier": "frontend"}))
//
//	// Use with test.Deployment
//	deployment := test.Deployment(opts)
func CreateDeploymentOptions(name string, replicas int32, cpuRequest, memoryRequest string, opts ...DeploymentOptionModifier) DeploymentOptions {
	// Create base deployment options
	deploymentOpts := DeploymentOptions{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Replicas: replicas,
		PodOptions: PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app":          name,
					DiscoveryLabel: "unspecified",
				},
			},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(cpuRequest),
					v1.ResourceMemory: resource.MustParse(memoryRequest),
				},
			},
		},
	}

	// Apply all modifiers
	for _, opt := range opts {
		opt(&deploymentOpts)
	}

	return deploymentOpts
}

// WithLabels adds or merges labels to the pod template
func WithLabels(labels map[string]string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		if opts.PodOptions.Labels == nil {
			opts.PodOptions.Labels = make(map[string]string)
		}
		for k, v := range labels {
			opts.PodOptions.Labels[k] = v
		}
	}
}

// WithAnnotations adds or merges annotations to the pod template
func WithAnnotations(annotations map[string]string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		if opts.PodOptions.Annotations == nil {
			opts.PodOptions.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			opts.PodOptions.Annotations[k] = v
		}
	}
}

// WithDoNotDisrupt adds the do-not-disrupt annotation to prevent Karpenter disruption
func WithDoNotDisrupt() DeploymentOptionModifier {
	return WithAnnotations(map[string]string{
		karpenterv1.DoNotDisruptAnnotationKey: "true",
	})
}

// WithHostnameSpread adds topology spread constraints to spread pods across hostnames
func WithHostnameSpread() DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		// Get the current labels to use in the label selector
		labels := opts.PodOptions.Labels
		if labels == nil {
			labels = map[string]string{"app": opts.Name}
		}

		constraint := v1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       v1.LabelHostname,
			WhenUnsatisfiable: v1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		}

		opts.PodOptions.TopologySpreadConstraints = append(
			opts.PodOptions.TopologySpreadConstraints,
			constraint,
		)
	}
}

// WithZoneSpread adds topology spread constraints to spread pods across zones
func WithZoneSpread() DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		// Get the current labels to use in the label selector
		labels := opts.PodOptions.Labels
		if labels == nil {
			labels = map[string]string{"app": opts.Name}
		}

		constraint := v1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       v1.LabelTopologyZone,
			WhenUnsatisfiable: v1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		}

		opts.PodOptions.TopologySpreadConstraints = append(
			opts.PodOptions.TopologySpreadConstraints,
			constraint,
		)
	}
}

// WithPodAntiAffinity adds pod anti-affinity to prevent pods from being scheduled on the same topology
func WithPodAntiAffinity(topologyKey string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		// Get the current labels to use in the label selector
		labels := opts.PodOptions.Labels
		if labels == nil {
			labels = map[string]string{"app": opts.Name}
		}

		antiAffinityTerm := v1.PodAffinityTerm{
			TopologyKey: topologyKey,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		}

		opts.PodOptions.PodAntiRequirements = append(
			opts.PodOptions.PodAntiRequirements,
			antiAffinityTerm,
		)
	}
}

// WithTopologySpreadConstraints adds custom topology spread constraints
func WithTopologySpreadConstraints(constraints []v1.TopologySpreadConstraint) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.TopologySpreadConstraints = append(
			opts.PodOptions.TopologySpreadConstraints,
			constraints...,
		)
	}
}

// WithResourceLimits adds resource limits to the pod containers
func WithResourceLimits(cpuLimit, memoryLimit string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		if opts.PodOptions.ResourceRequirements.Limits == nil {
			opts.PodOptions.ResourceRequirements.Limits = make(v1.ResourceList)
		}
		opts.PodOptions.ResourceRequirements.Limits[v1.ResourceCPU] = resource.MustParse(cpuLimit)
		opts.PodOptions.ResourceRequirements.Limits[v1.ResourceMemory] = resource.MustParse(memoryLimit)
	}
}

// WithPriorityClass sets the priority class for the pods
func WithPriorityClass(priorityClassName string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.PriorityClassName = priorityClassName
	}
}

// WithNodeSelector adds node selector requirements
func WithNodeSelector(nodeSelector map[string]string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		if opts.PodOptions.NodeSelector == nil {
			opts.PodOptions.NodeSelector = make(map[string]string)
		}
		for k, v := range nodeSelector {
			opts.PodOptions.NodeSelector[k] = v
		}
	}
}

// WithTolerations adds tolerations to the pod spec
func WithTolerations(tolerations []v1.Toleration) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.Tolerations = append(opts.PodOptions.Tolerations, tolerations...)
	}
}

// WithTerminationGracePeriod sets the termination grace period for pods
func WithTerminationGracePeriod(seconds int64) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.TerminationGracePeriodSeconds = &seconds
	}
}

// WithNoResourceRequests removes resource requirements entirely (for backward compatibility)
func WithNoResourceRequests() DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.ResourceRequirements = v1.ResourceRequirements{}
	}
}

// WithImage sets a custom container image
func WithImage(image string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.Image = image
	}
}

// WithRestartPolicy sets the pod restart policy
func WithRestartPolicy(policy v1.RestartPolicy) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.RestartPolicy = policy
	}
}

// WithReadinessProbe adds a readiness probe to the pod
func WithReadinessProbe(probe *v1.Probe) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.ReadinessProbe = probe
	}
}

// WithLivenessProbe adds a liveness probe to the pod
func WithLivenessProbe(probe *v1.Probe) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.LivenessProbe = probe
	}
}

// WithPodAntiAffinityHostname is a convenience function for hostname anti-affinity
func WithPodAntiAffinityHostname() DeploymentOptionModifier {
	return WithPodAntiAffinity(v1.LabelHostname)
}

// WithCommand sets a custom command for the container
func WithCommand(command []string) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.Command = command
	}
}

// WithPreStopSleep adds a pre-stop sleep hook to the container
func WithPreStopSleep(seconds int64) DeploymentOptionModifier {
	return func(opts *DeploymentOptions) {
		opts.PodOptions.PreStopSleep = &seconds
	}
}
