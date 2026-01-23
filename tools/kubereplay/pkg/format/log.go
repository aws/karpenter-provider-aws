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

package format

import (
	"encoding/json"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
)

// EventType represents the type of workload event
type EventType string

const (
	// EventCreate represents creation of a new workload
	EventCreate EventType = "create"
	// EventScale represents a scale change (replicas update)
	EventScale EventType = "scale"
	// EventDelete represents deletion of a workload
	EventDelete EventType = "delete"
)

// WorkloadKind represents the kind of workload
type WorkloadKind string

const (
	KindDeployment WorkloadKind = "Deployment"
	KindJob        WorkloadKind = "Job"
)

// WorkloadEvent represents a workload event from the audit log
type WorkloadEvent struct {
	// Type is the event type (create, scale)
	Type EventType `json:"type"`
	// Kind is the workload kind (Deployment, Job)
	Kind WorkloadKind `json:"kind"`
	// Key is the original namespace/name for tracking across events
	Key string `json:"key"`
	// Deployment is set when Kind is Deployment
	Deployment *appsv1.Deployment `json:"deployment,omitempty"`
	// Job is set when Kind is Job
	Job *batchv1.Job `json:"job,omitempty"`
	// Replicas is set for scale events to track the new replica count
	Replicas *int32 `json:"replicas,omitempty"`
	// Timestamp is when this event occurred
	Timestamp time.Time `json:"timestamp"`
}

// ReplayLog is the top-level structure for a replay log file
type ReplayLog struct {
	Cluster  string          `json:"cluster"`
	Captured time.Time       `json:"captured"`
	Events   []WorkloadEvent `json:"events"`
}

// NewReplayLog creates a new replay log
func NewReplayLog(cluster string) *ReplayLog {
	return &ReplayLog{
		Cluster:  cluster,
		Captured: time.Now(),
		Events:   []WorkloadEvent{},
	}
}

// AddDeploymentCreate adds a deployment creation event
func (r *ReplayLog) AddDeploymentCreate(deployment *appsv1.Deployment, timestamp time.Time) {
	r.Events = append(r.Events, WorkloadEvent{
		Type:       EventCreate,
		Kind:       KindDeployment,
		Key:        deployment.Namespace + "/" + deployment.Name,
		Deployment: deployment,
		Timestamp:  timestamp,
	})
}

// AddDeploymentScale adds a deployment scale event
func (r *ReplayLog) AddDeploymentScale(namespace, name string, replicas int32, timestamp time.Time) {
	r.Events = append(r.Events, WorkloadEvent{
		Type:      EventScale,
		Kind:      KindDeployment,
		Key:       namespace + "/" + name,
		Replicas:  &replicas,
		Timestamp: timestamp,
	})
}

// AddDeploymentDelete adds a deployment deletion event
func (r *ReplayLog) AddDeploymentDelete(namespace, name string, timestamp time.Time) {
	r.Events = append(r.Events, WorkloadEvent{
		Type:      EventDelete,
		Kind:      KindDeployment,
		Key:       namespace + "/" + name,
		Timestamp: timestamp,
	})
}

// AddJobCreate adds a job creation event
func (r *ReplayLog) AddJobCreate(job *batchv1.Job, timestamp time.Time) {
	r.Events = append(r.Events, WorkloadEvent{
		Type:      EventCreate,
		Kind:      KindJob,
		Key:       job.Namespace + "/" + job.Name,
		Job:       job,
		Timestamp: timestamp,
	})
}

// WriteToFile writes the replay log to a file
func (r *ReplayLog) WriteToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// ReadFromFile reads a replay log from a file
func ReadFromFile(path string) (*ReplayLog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var log ReplayLog
	if err := json.NewDecoder(f).Decode(&log); err != nil {
		return nil, err
	}
	return &log, nil
}
