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

package replay

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/format"
)

// Engine orchestrates workload replay against a target cluster
type Engine struct {
	kubeClient client.Client
	namespace  string
	// deploymentNames maps original key to replayed deployment name
	deploymentNames map[string]string
}

// NewEngine creates a new replay engine
func NewEngine(kubeClient client.Client, namespace string) *Engine {
	return &Engine{
		kubeClient:      kubeClient,
		namespace:       namespace,
		deploymentNames: make(map[string]string),
	}
}

// RunTimed creates workloads according to their original timing with optional time dilation.
// speed controls replay speed: 1.0 = real-time, 24.0 = 24x faster (24h replays in 1h).
// If dryRun is true, prints events without actually creating workloads.
func (e *Engine) RunTimed(ctx context.Context, log *format.ReplayLog, speed float64, dryRun bool) (int, error) {
	if len(log.Events) == 0 {
		return 0, nil
	}

	// Sort events by timestamp
	events := make([]format.WorkloadEvent, len(log.Events))
	copy(events, log.Events)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})


	// Calculate time span
	firstTimestamp := events[0].Timestamp
	lastTimestamp := events[len(events)-1].Timestamp
	originalDuration := lastTimestamp.Sub(firstTimestamp)
	replayDuration := time.Duration(float64(originalDuration) / speed)

	fmt.Printf("Replaying %d events over %v (original: %v, speed: %.1fx)\n",
		len(events), replayDuration.Round(time.Second), originalDuration.Round(time.Second), speed)

	// Track replay start time
	replayStart := time.Now()
	var applied int

	for _, event := range events {
		// Calculate when this event should be applied relative to replay start
		originalOffset := event.Timestamp.Sub(firstTimestamp)
		replayOffset := time.Duration(float64(originalOffset) / speed)
		targetTime := replayStart.Add(replayOffset)

		// Wait until target time
		waitDuration := time.Until(targetTime)
		if waitDuration > 0 {
			select {
			case <-ctx.Done():
				return applied, ctx.Err()
			case <-time.After(waitDuration):
			}
		}

		offsetStr := formatOffset(originalOffset)
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		origStr := event.Timestamp.Format("2006-01-02 15:04:05")

		if dryRun {
			if event.Type == format.EventScale && event.Replicas != nil {
				fmt.Printf("[%s] [%s] [%s] [dry-run] %s %s %s -> %d replicas\n",
					nowStr, origStr, offsetStr, event.Type, event.Kind, event.Key, *event.Replicas)
			} else {
				fmt.Printf("[%s] [%s] [%s] [dry-run] %s %s %s\n",
					nowStr, origStr, offsetStr, event.Type, event.Kind, event.Key)
			}
			// Don't count dry-run events as applied
		} else {
			name, err := e.applyEvent(ctx, &event)
			if err != nil {
				fmt.Printf("[%s] [%s] [%s] FAILED %s %s %s: %v\n",
					nowStr, origStr, offsetStr, event.Type, event.Kind, event.Key, err)
				continue
			}
			fmt.Printf("[%s] [%s] [%s] %s %s %s -> %s\n",
				nowStr, origStr, offsetStr, event.Type, event.Kind, event.Key, name)
			applied++
		}
	}

	return applied, nil
}

// applyEvent applies a single workload event and returns the created/updated resource name
func (e *Engine) applyEvent(ctx context.Context, event *format.WorkloadEvent) (string, error) {
	switch event.Type {
	case format.EventCreate:
		switch event.Kind {
		case format.KindDeployment:
			return e.createDeployment(ctx, event.Deployment)
		case format.KindJob:
			return e.createJob(ctx, event.Job)
		}
	case format.EventScale:
		if event.Replicas == nil {
			return "", fmt.Errorf("scale event missing replicas for %s", event.Key)
		}
		return e.scaleDeployment(ctx, event.Key, *event.Replicas)
	}
	return "", nil
}

func (e *Engine) createDeployment(ctx context.Context, deployment *appsv1.Deployment) (string, error) {
	deployCopy := deployment.DeepCopy()
	deployCopy.Namespace = e.namespace
	deployCopy.ResourceVersion = ""
	deployCopy.UID = ""

	// Track the mapping from original key to new name
	originalKey := deployment.Namespace + "/" + deployment.Name
	e.deploymentNames[originalKey] = deployCopy.Name

	err := e.kubeClient.Create(ctx, deployCopy)
	if err != nil && errors.IsAlreadyExists(err) {
		return deployCopy.Name, nil
	}
	if err != nil {
		return "", err
	}

	// Create PDB for this deployment
	if err := e.createPDBForDeployment(ctx, deployCopy); err != nil {
		fmt.Printf("  Warning: failed to create PDB for %s: %v\n", deployCopy.Name, err)
	}

	return deployCopy.Name, nil
}

func (e *Engine) createJob(ctx context.Context, job *batchv1.Job) (string, error) {
	jobCopy := job.DeepCopy()
	jobCopy.Namespace = e.namespace
	jobCopy.ResourceVersion = ""
	jobCopy.UID = ""

	err := e.kubeClient.Create(ctx, jobCopy)
	if err != nil && errors.IsAlreadyExists(err) {
		return jobCopy.Name, nil
	}
	return jobCopy.Name, err
}

func (e *Engine) scaleDeployment(ctx context.Context, originalKey string, replicas int32) (string, error) {
	// Look up the replayed deployment name
	deployName, exists := e.deploymentNames[originalKey]
	if !exists {
		return "", fmt.Errorf("unknown deployment: %s (scale event without prior create)", originalKey)
	}

	// Use merge patch to update only the replicas field
	deployment := &appsv1.Deployment{}
	if err := e.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: e.namespace,
		Name:      deployName,
	}, deployment); err != nil {
		return "", fmt.Errorf("failed to get deployment %s: %w", deployName, err)
	}

	patch := client.MergeFrom(deployment.DeepCopy())
	deployment.Spec.Replicas = &replicas
	if err := e.kubeClient.Patch(ctx, deployment, patch); err != nil {
		return "", fmt.Errorf("failed to scale deployment %s: %w", deployName, err)
	}

	return fmt.Sprintf("%s (replicas=%d)", deployName, replicas), nil
}

// EnsureNamespace ensures the target namespace exists and is ready.
// If the namespace is terminating, it waits for deletion to complete before recreating.
func (e *Engine) EnsureNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{}
	err := e.kubeClient.Get(ctx, client.ObjectKey{Name: e.namespace}, ns)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check namespace: %w", err)
	}

	// If namespace exists and is terminating, wait for it to be deleted
	if err == nil && ns.Status.Phase == corev1.NamespaceTerminating {
		fmt.Printf("Waiting for namespace %s to finish terminating...\n", e.namespace)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}

			err := e.kubeClient.Get(ctx, client.ObjectKey{Name: e.namespace}, ns)
			if errors.IsNotFound(err) {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to check namespace: %w", err)
			}
		}
	}

	// Create the namespace if it doesn't exist
	if errors.IsNotFound(err) || ns.Status.Phase == corev1.NamespaceTerminating {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: e.namespace,
			},
		}
		if err := e.kubeClient.Create(ctx, ns); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace: %w", err)
		}
	}

	return nil
}

// createPDBForDeployment creates a PodDisruptionBudget for a deployment
func (e *Engine) createPDBForDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	maxUnavailable := intstr.FromString("50%")
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pdb-%s", deployment.Name),
			Namespace: e.namespace,
			Labels: map[string]string{
				"kubereplay.karpenter.sh/managed": "true",
			},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector:       deployment.Spec.Selector,
		},
	}

	err := e.kubeClient.Create(ctx, pdb)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// CleanupWorkloads deletes all workloads with the kubereplay tracking label
func (e *Engine) CleanupWorkloads(ctx context.Context) error {
	// Delete PDBs first
	if err := e.kubeClient.DeleteAllOf(ctx, &policyv1.PodDisruptionBudget{},
		client.InNamespace(e.namespace),
		client.MatchingLabels{"kubereplay.karpenter.sh/managed": "true"},
	); err != nil {
		return fmt.Errorf("failed to cleanup PDBs: %w", err)
	}

	// Delete deployments (cascading delete will remove pods)
	if err := e.kubeClient.DeleteAllOf(ctx, &appsv1.Deployment{},
		client.InNamespace(e.namespace),
		client.MatchingLabels{"kubereplay.karpenter.sh/managed": "true"},
	); err != nil {
		return fmt.Errorf("failed to cleanup deployments: %w", err)
	}

	// Delete jobs (cascading delete will remove pods)
	if err := e.kubeClient.DeleteAllOf(ctx, &batchv1.Job{},
		client.InNamespace(e.namespace),
		client.MatchingLabels{"kubereplay.karpenter.sh/managed": "true"},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	); err != nil {
		return fmt.Errorf("failed to cleanup jobs: %w", err)
	}

	return nil
}

// Status represents the current state of pods and nodes
type Status struct {
	// Workloads
	Deployments      int
	DeploymentsReady int
	Jobs             int
	JobsComplete     int
	// Pods
	Pods        int
	PodsPending int
	PodsRunning int
	PodsFailed  int
	// Nodes
	Nodes      int
	NodesReady int
}

func (s Status) String() string {
	if s.PodsFailed > 0 {
		return fmt.Sprintf("deployments=%d/%d ready, jobs=%d/%d complete, pods=%d/%d ready (%d failed), nodes=%d/%d ready",
			s.DeploymentsReady, s.Deployments, s.JobsComplete, s.Jobs,
			s.PodsRunning, s.Pods, s.PodsFailed, s.NodesReady, s.Nodes)
	}
	return fmt.Sprintf("deployments=%d/%d ready, jobs=%d/%d complete, pods=%d/%d ready, nodes=%d/%d ready",
		s.DeploymentsReady, s.Deployments, s.JobsComplete, s.Jobs,
		s.PodsRunning, s.Pods, s.NodesReady, s.Nodes)
}

func (s Status) IsStable() bool {
	// Must have at least some workloads
	if s.Deployments == 0 && s.Jobs == 0 {
		return false
	}
	// All deployments must be ready (if any)
	if s.Deployments > 0 && s.DeploymentsReady != s.Deployments {
		return false
	}
	// All jobs must be complete (if any)
	if s.Jobs > 0 && s.JobsComplete != s.Jobs {
		return false
	}
	// No pods pending or failed
	if s.PodsPending > 0 || s.PodsFailed > 0 {
		return false
	}
	// All nodes ready
	return s.Nodes == s.NodesReady
}

// WaitForStable waits until workloads and nodes stabilize
func (e *Engine) WaitForStable(ctx context.Context, quietPeriod time.Duration, onStatus func(Status)) error {
	var lastStatus Status
	var stableSince time.Time

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := e.getStatus(ctx)
			if err != nil {
				return err
			}

			if onStatus != nil {
				onStatus(status)
			}

			// Check if status changed
			if status != lastStatus {
				lastStatus = status
				stableSince = time.Now()
				continue
			}

			// Check if stable for long enough
			if status.IsStable() && time.Since(stableSince) >= quietPeriod {
				return nil
			}
		}
	}
}

func (e *Engine) getStatus(ctx context.Context) (Status, error) {
	var status Status

	// Get deployment status
	var deployments appsv1.DeploymentList
	if err := e.kubeClient.List(ctx, &deployments,
		client.InNamespace(e.namespace),
		client.MatchingLabels{"kubereplay.karpenter.sh/managed": "true"},
	); err != nil {
		return Status{}, err
	}

	status.Deployments = len(deployments.Items)
	status.DeploymentsReady = lo.CountBy(deployments.Items, func(d appsv1.Deployment) bool {
		return d.Status.ReadyReplicas == lo.FromPtrOr(d.Spec.Replicas, 1)
	})

	// Get job status
	var jobs batchv1.JobList
	if err := e.kubeClient.List(ctx, &jobs,
		client.InNamespace(e.namespace),
		client.MatchingLabels{"kubereplay.karpenter.sh/managed": "true"},
	); err != nil {
		return Status{}, err
	}

	status.Jobs = len(jobs.Items)
	status.JobsComplete = lo.CountBy(jobs.Items, func(j batchv1.Job) bool {
		return j.Status.Succeeded > 0 || j.Status.CompletionTime != nil
	})

	// Get pod status
	var pods corev1.PodList
	if err := e.kubeClient.List(ctx, &pods,
		client.InNamespace(e.namespace),
		client.MatchingLabels{"kubereplay.karpenter.sh/managed": "true"},
	); err != nil {
		return Status{}, err
	}

	status.Pods = len(pods.Items)
	status.PodsPending = lo.CountBy(pods.Items, func(p corev1.Pod) bool { return p.Status.Phase == corev1.PodPending })
	status.PodsRunning = lo.CountBy(pods.Items, func(p corev1.Pod) bool { return p.Status.Phase == corev1.PodRunning })
	status.PodsFailed = lo.CountBy(pods.Items, func(p corev1.Pod) bool { return p.Status.Phase == corev1.PodFailed })

	// Get node status (only Karpenter-managed nodes)
	var nodes corev1.NodeList
	if err := e.kubeClient.List(ctx, &nodes,
		client.HasLabels{"karpenter.sh/nodepool"},
	); err != nil {
		return Status{}, err
	}

	status.Nodes = len(nodes.Items)
	status.NodesReady = lo.CountBy(nodes.Items, func(n corev1.Node) bool {
		return lo.ContainsBy(n.Status.Conditions, func(c corev1.NodeCondition) bool {
			return c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue
		})
	})

	return status, nil
}

// formatOffset formats a duration as a human-readable offset string like "+1h2m3s"
func formatOffset(d time.Duration) string {
	if d < time.Second {
		return "+0s"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("+%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("+%dm%ds", m, s)
	}
	return fmt.Sprintf("+%ds", s)
}
