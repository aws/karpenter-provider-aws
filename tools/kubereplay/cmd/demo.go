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

package main

import (
	"fmt"
	"math/rand"
	"slices"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/format"
	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/sanitizer"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Generate synthetic replay data",
	RunE:  runDemo,
}

var (
	demoOutput      string
	demoDeployments int
	demoJobs        int
	demoDuration    time.Duration
)

func init() {
	demoCmd.Flags().StringVarP(&demoOutput, "output", "o", "replay.json", "Output file")
	demoCmd.Flags().IntVar(&demoDeployments, "deployments", 20, "Number of deployments")
	demoCmd.Flags().IntVar(&demoJobs, "jobs", 10, "Number of jobs")
	demoCmd.Flags().DurationVar(&demoDuration, "duration", time.Hour, "Time span to spread events across")
}

func runDemo(cmd *cobra.Command, args []string) error {
	replayLog := format.NewReplayLog("demo")
	baseTime := time.Now().Add(-demoDuration)

	// Generate deployments spread across timeline
	deploymentTimestamps := generateSortedTimestamps(baseTime, demoDuration, demoDeployments)
	for i := range demoDeployments {
		deployment := generateDeployment(i)
		replayLog.AddDeploymentCreate(deployment, deploymentTimestamps[i])

		// Track latest event time for this deployment (for potential delete)
		latestEventTime := deploymentTimestamps[i]

		// Generate 1-3 scale events per deployment (simulates HPA)
		scaleCount := rand.Intn(3) + 1
		for range scaleCount {
			// Scale happens after creation
			remainingTime := baseTime.Add(demoDuration).Sub(deploymentTimestamps[i])
			if remainingTime > time.Minute {
				scaleOffset := time.Duration(rand.Int63n(int64(remainingTime)))
				scaleTime := deploymentTimestamps[i].Add(scaleOffset)

				newReplicas := int32(rand.Intn(10) + 1)
				replayLog.AddDeploymentScale(
					sanitizer.Namespace,
					fmt.Sprintf("deployment-%d", i),
					newReplicas,
					scaleTime,
				)
				if scaleTime.After(latestEventTime) {
					latestEventTime = scaleTime
				}
			}
		}

		// 50% chance of deletion (simulates workload churn for consolidation testing)
		if rand.Float32() < 0.5 {
			remainingTime := baseTime.Add(demoDuration).Sub(latestEventTime)
			if remainingTime > time.Minute {
				deleteOffset := time.Duration(rand.Int63n(int64(remainingTime)))
				deleteTime := latestEventTime.Add(deleteOffset)
				replayLog.AddDeploymentDelete(
					sanitizer.Namespace,
					fmt.Sprintf("deployment-%d", i),
					deleteTime,
				)
			}
		}
	}

	// Generate jobs spread across timeline
	jobTimestamps := generateSortedTimestamps(baseTime, demoDuration, demoJobs)
	for i := range demoJobs {
		job := generateJob(i)
		replayLog.AddJobCreate(job, jobTimestamps[i])
	}

	// Sort all events by timestamp
	slices.SortFunc(replayLog.Events, func(a, b format.WorkloadEvent) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	if err := replayLog.WriteToFile(demoOutput); err != nil {
		return err
	}

	summary := eventSummary(replayLog)
	fmt.Printf("Generated %s to %s\n", summary, demoOutput)
	return nil
}

func generateSortedTimestamps(base time.Time, duration time.Duration, count int) []time.Time {
	timestamps := lo.Times(count, func(_ int) time.Time {
		return base.Add(time.Duration(rand.Int63n(int64(duration))))
	})
	slices.SortFunc(timestamps, func(a, b time.Time) int { return a.Compare(b) })
	return timestamps
}

func generateDeployment(index int) *appsv1.Deployment {
	appLabel := fmt.Sprintf("app-%d", index)
	replicas := int32(rand.Intn(5) + 1) // 1-5 replicas

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("deployment-%d", index),
			Namespace: sanitizer.Namespace,
			Labels: map[string]string{
				"kubereplay.karpenter.sh/managed": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appLabel,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                              appLabel,
						"kubereplay.karpenter.sh/managed": "true",
					},
				},
				Spec: generatePodSpec(false, 0),
			},
		},
	}

	return deployment
}

func generateJob(index int) *batchv1.Job {
	appLabel := fmt.Sprintf("job-%d", index)
	parallelism := int32(rand.Intn(3) + 1) // 1-3 parallelism
	completions := int32(rand.Intn(5) + 1) // 1-5 completions
	sleepSeconds := rand.Intn(20) + 10     // 10-30 seconds to complete

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("job-%d", index),
			Namespace: sanitizer.Namespace,
			Labels: map[string]string{
				"kubereplay.karpenter.sh/managed": "true",
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &parallelism,
			Completions: &completions,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                              appLabel,
						"kubereplay.karpenter.sh/managed": "true",
					},
				},
				Spec: generatePodSpec(true, sleepSeconds),
			},
		},
	}

	return job
}

func generatePodSpec(forJob bool, sleepSeconds int) corev1.PodSpec {
	container := corev1.Container{
		Name: "container-0",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity([]int64{100, 250, 500, 1000, 2000}[rand.Intn(5)], resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity([]int64{128, 256, 512, 1024, 2048}[rand.Intn(5)]*1024*1024, resource.BinarySI),
			},
		},
	}

	// Jobs use busybox with sleep so they complete; Deployments use pause
	if forJob {
		container.Image = "busybox:1.36"
		container.Command = []string{"sh", "-c", fmt.Sprintf("sleep %d", sleepSeconds)}
	} else {
		container.Image = "registry.k8s.io/pause:3.9"
	}

	spec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	// Only jobs support RestartPolicyNever; deployments require Always (default)
	if forJob {
		spec.RestartPolicy = corev1.RestartPolicyNever
	}

	// 30% node selector
	if rand.Float32() < 0.3 {
		spec.NodeSelector = map[string]string{
			"karpenter.sh/capacity-type": lo.Sample([]string{"spot", "on-demand"}),
		}
	}

	// 25% topology spread
	if rand.Float32() < 0.25 {
		spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"kubereplay.karpenter.sh/managed": "true"}},
		}}
	}

	return spec
}
