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

package sanitizer

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const Namespace = "kubereplay"

// Sanitizer transforms workloads for replay by stripping unnecessary fields
type Sanitizer struct {
	deploymentCounter atomic.Uint64
	jobCounter        atomic.Uint64
	// keyMapping tracks original key -> sanitized key for scale event correlation
	keyMapping map[string]string
}

// New creates a new workload sanitizer
func New() *Sanitizer {
	return &Sanitizer{
		keyMapping: make(map[string]string),
	}
}

// GetSanitizedKey returns the sanitized key for an original deployment key.
// This is used to correlate scale events with their sanitized deployments.
func (s *Sanitizer) GetSanitizedKey(originalKey string) (string, bool) {
	key, ok := s.keyMapping[originalKey]
	return key, ok
}

// SanitizeDeployment creates a sanitized copy of a deployment suitable for replay
func (s *Sanitizer) SanitizeDeployment(deployment *appsv1.Deployment) *appsv1.Deployment {
	seqID := s.deploymentCounter.Add(1) - 1
	newDeploy := deployment.DeepCopy()

	// Track original -> sanitized key mapping for scale event correlation
	originalKey := deployment.Namespace + "/" + deployment.Name
	sanitizedKey := Namespace + "/" + fmt.Sprintf("deployment-%d", seqID)
	s.keyMapping[originalKey] = sanitizedKey

	// Set new name and namespace
	newDeploy.Name = fmt.Sprintf("deployment-%d", seqID)
	newDeploy.Namespace = Namespace

	// Add tracking label
	if newDeploy.Labels == nil {
		newDeploy.Labels = map[string]string{}
	}
	newDeploy.Labels["kubereplay.karpenter.sh/managed"] = "true"

	// Clear metadata
	clearObjectMeta(&newDeploy.ObjectMeta)

	// Preserve Karpenter annotations
	newDeploy.Annotations = filterKarpenterAnnotations(deployment.Annotations)

	// Update selector and template labels to match new name
	appLabel := fmt.Sprintf("app-%d", seqID)
	newDeploy.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": appLabel,
		},
	}

	// Sanitize pod template
	newDeploy.Spec.Template = sanitizePodTemplateSpec(newDeploy.Spec.Template, appLabel)

	// Clear status
	newDeploy.Status = appsv1.DeploymentStatus{}

	return newDeploy
}

// SanitizeJob creates a sanitized copy of a job suitable for replay
func (s *Sanitizer) SanitizeJob(job *batchv1.Job) *batchv1.Job {
	seqID := s.jobCounter.Add(1) - 1
	newJob := job.DeepCopy()

	// Set new name and namespace
	newJob.Name = fmt.Sprintf("job-%d", seqID)
	newJob.Namespace = Namespace

	// Add tracking label
	if newJob.Labels == nil {
		newJob.Labels = map[string]string{}
	}
	newJob.Labels["kubereplay.karpenter.sh/managed"] = "true"

	// Clear metadata
	clearObjectMeta(&newJob.ObjectMeta)

	// Preserve Karpenter annotations
	newJob.Annotations = filterKarpenterAnnotations(job.Annotations)

	// Update selector and template labels to match new name
	appLabel := fmt.Sprintf("job-%d", seqID)

	// Jobs auto-generate selectors, so we clear it to let k8s regenerate
	newJob.Spec.Selector = nil

	// Sanitize pod template
	newJob.Spec.Template = sanitizePodTemplateSpec(newJob.Spec.Template, appLabel)

	// Clear TTL (we manage cleanup)
	newJob.Spec.TTLSecondsAfterFinished = nil

	// Clear status
	newJob.Status = batchv1.JobStatus{}

	return newJob
}

func clearObjectMeta(meta *metav1.ObjectMeta) {
	meta.UID = ""
	meta.ResourceVersion = ""
	meta.CreationTimestamp = metav1.Time{}
	meta.DeletionTimestamp = nil
	meta.DeletionGracePeriodSeconds = nil
	meta.OwnerReferences = nil
	meta.Finalizers = nil
	meta.ManagedFields = nil
	meta.Generation = 0
	meta.GenerateName = ""
}

func sanitizePodTemplateSpec(template corev1.PodTemplateSpec, appLabel string) corev1.PodTemplateSpec {
	// Clear auto-generated labels (Job controller labels, etc.) and set fresh ones
	// We keep Karpenter labels if any
	karpenterLabels := lo.PickBy(template.Labels, func(v string, k string) bool {
		return strings.HasPrefix(k, "karpenter.sh/")
	})
	template.Labels = map[string]string{
		"app":                           appLabel,
		"kubereplay.karpenter.sh/managed": "true",
	}
	for k, v := range karpenterLabels {
		template.Labels[k] = v
	}

	// Preserve Karpenter annotations on pod template
	template.Annotations = filterKarpenterAnnotations(template.Annotations)

	// Sanitize containers
	template.Spec.Containers = sanitizeContainers(template.Spec.Containers)
	template.Spec.InitContainers = sanitizeContainers(template.Spec.InitContainers)

	// Clear non-scheduling fields
	template.Spec.ServiceAccountName = "default"
	template.Spec.AutomountServiceAccountToken = lo.ToPtr(false)
	template.Spec.Volumes = nil
	template.Spec.ImagePullSecrets = nil
	template.Spec.HostNetwork = false
	template.Spec.HostPID = false
	template.Spec.HostIPC = false
	template.Spec.SecurityContext = nil
	template.Spec.DNSPolicy = corev1.DNSDefault
	template.Spec.DNSConfig = nil
	template.Spec.Hostname = ""
	template.Spec.Subdomain = ""
	template.Spec.NodeName = ""

	// Keep scheduling-relevant fields:
	// - NodeSelector
	// - Affinity
	// - Tolerations
	// - TopologySpreadConstraints
	// - PriorityClassName
	// - Resources (in containers)

	return template
}

func sanitizeContainers(containers []corev1.Container) []corev1.Container {
	return lo.Map(containers, func(c corev1.Container, i int) corev1.Container {
		return corev1.Container{
			Name:      fmt.Sprintf("container-%d", i),
			Image:     "registry.k8s.io/pause:3.9",
			Resources: c.Resources,
		}
	})
}

func filterKarpenterAnnotations(annotations map[string]string) map[string]string {
	result := lo.PickBy(annotations, func(v string, k string) bool {
		return strings.HasPrefix(k, "karpenter.sh/")
	})
	if len(result) == 0 {
		return nil
	}
	return result
}
