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

package parser

import (
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
)

// Parser extracts workload events from audit logs
type Parser struct {
	excludeNamespaces map[string]bool
	// deploymentReplicas tracks last known replicas for each deployment to detect scale changes
	deploymentReplicas map[string]int32
	// deploymentEmitted tracks which deployments we've emitted a create event for
	deploymentEmitted map[string]bool
	// jobCreateTimes tracks when jobs were created for duration calculation
	jobCreateTimes map[string]time.Time
}

// NewParser creates a new audit log parser
func NewParser() *Parser {
	return &Parser{
		excludeNamespaces:  DefaultExcludedNamespaces,
		deploymentReplicas: make(map[string]int32),
		deploymentEmitted:  make(map[string]bool),
		jobCreateTimes:     make(map[string]time.Time),
	}
}

// GetJobCreateTime returns the creation time for a job key (namespace/name)
func (p *Parser) GetJobCreateTime(key string) (time.Time, bool) {
	t, ok := p.jobCreateTimes[key]
	return t, ok
}

// WorkloadResult represents the result of parsing an audit event
type WorkloadResult struct {
	// Deployment is set for deployment create events
	Deployment *appsv1.Deployment
	// Job is set for job create events
	Job *batchv1.Job
	// ScaleEvent is set for deployment scale changes
	ScaleEvent *ScaleEvent
	// DeleteEvent is set for deployment delete events
	DeleteEvent *DeleteEvent
	// JobCompleteEvent is set when a job completes
	JobCompleteEvent *JobCompleteEvent
	// Timestamp is when the event occurred
	Timestamp time.Time
}

// ScaleEvent represents a deployment scale change
type ScaleEvent struct {
	Namespace string
	Name      string
	Replicas  int32
}

// DeleteEvent represents a deployment deletion
type DeleteEvent struct {
	Namespace string
	Name      string
}

// JobCompleteEvent represents a job completion
type JobCompleteEvent struct {
	Namespace      string
	Name           string
	CompletionTime time.Time
}

// ParseEvent extracts workload information from an audit event.
// Returns nil result if the event should be skipped.
func (p *Parser) ParseEvent(event AuditEvent) (*WorkloadResult, error) {
	// Filter: must be ResponseComplete stage
	if event.Stage != "ResponseComplete" {
		return nil, nil
	}

	// Filter: must be successful response (2xx)
	if event.ResponseStatus != nil && (event.ResponseStatus.Code < 200 || event.ResponseStatus.Code >= 300) {
		return nil, nil
	}

	// Filter: namespace exclusion
	ns := event.ObjectRef.Namespace
	if p.excludeNamespaces[ns] {
		return nil, nil
	}

	switch event.ObjectRef.Resource {
	case "deployments":
		return p.parseDeploymentEvent(event)
	case "jobs":
		return p.parseJobEvent(event)
	default:
		return nil, nil
	}
}

func (p *Parser) parseDeploymentEvent(event AuditEvent) (*WorkloadResult, error) {
	// Handle delete events - emit all deletes, let caller correlate
	if event.Verb == "delete" {
		key := event.ObjectRef.Namespace + "/" + event.ObjectRef.Name
		// Clean up tracking state if we had it
		delete(p.deploymentEmitted, key)
		delete(p.deploymentReplicas, key)
		return &WorkloadResult{
			DeleteEvent: &DeleteEvent{
				Namespace: event.ObjectRef.Namespace,
				Name:      event.ObjectRef.Name,
			},
			Timestamp: event.RequestReceivedTimestamp,
		}, nil
	}

	// Handle scale subresource specially
	if event.ObjectRef.Subresource == "scale" {
		return p.parseScaleSubresource(event)
	}

	// Skip other subresources
	if event.ObjectRef.Subresource != "" {
		return nil, nil
	}

	// Parse the deployment
	var deployment appsv1.Deployment
	if event.ResponseObject != nil {
		if err := json.Unmarshal(event.ResponseObject, &deployment); err != nil {
			return nil, fmt.Errorf("failed to parse deployment from response object: %w", err)
		}
	} else if event.RequestObject != nil {
		if err := json.Unmarshal(event.RequestObject, &deployment); err != nil {
			return nil, fmt.Errorf("failed to parse deployment from request object: %w", err)
		}
	} else {
		return nil, nil
	}

	key := deployment.Namespace + "/" + deployment.Name
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	switch event.Verb {
	case "create":
		// Track and emit
		p.deploymentReplicas[key] = replicas
		p.deploymentEmitted[key] = true
		return &WorkloadResult{
			Deployment: &deployment,
			Timestamp:  event.RequestReceivedTimestamp,
		}, nil

	case "update", "patch":
		// Check if we've emitted a create for this deployment
		if !p.deploymentEmitted[key] {
			// First time seeing full spec for this deployment
			// It existed before our audit window - emit as create
			p.deploymentReplicas[key] = replicas
			p.deploymentEmitted[key] = true
			return &WorkloadResult{
				Deployment: &deployment,
				Timestamp:  event.RequestReceivedTimestamp,
			}, nil
		}
		// Check if replicas changed
		if lastReplicas, exists := p.deploymentReplicas[key]; exists && replicas != lastReplicas {
			p.deploymentReplicas[key] = replicas
			return &WorkloadResult{
				ScaleEvent: &ScaleEvent{
					Namespace: deployment.Namespace,
					Name:      deployment.Name,
					Replicas:  replicas,
				},
				Timestamp: event.RequestReceivedTimestamp,
			}, nil
		}
		p.deploymentReplicas[key] = replicas
		// No meaningful change, skip
		return nil, nil
	}

	return nil, nil
}

func (p *Parser) parseScaleSubresource(event AuditEvent) (*WorkloadResult, error) {
	// Scale subresource updates come with a Scale object (no full deployment spec)
	type scaleSpec struct {
		Replicas int32 `json:"replicas"`
	}
	type scaleObject struct {
		Spec scaleSpec `json:"spec"`
	}

	var scale scaleObject
	if event.ResponseObject != nil {
		if err := json.Unmarshal(event.ResponseObject, &scale); err != nil {
			return nil, nil // Skip if we can't parse
		}
	} else if event.RequestObject != nil {
		if err := json.Unmarshal(event.RequestObject, &scale); err != nil {
			return nil, nil
		}
	} else {
		return nil, nil
	}

	key := event.ObjectRef.Namespace + "/" + event.ObjectRef.Name
	lastReplicas := p.deploymentReplicas[key]

	// Only emit scale event if we've already emitted a create for this deployment
	// and replicas actually changed
	if p.deploymentEmitted[key] && scale.Spec.Replicas != lastReplicas {
		p.deploymentReplicas[key] = scale.Spec.Replicas
		return &WorkloadResult{
			ScaleEvent: &ScaleEvent{
				Namespace: event.ObjectRef.Namespace,
				Name:      event.ObjectRef.Name,
				Replicas:  scale.Spec.Replicas,
			},
			Timestamp: event.RequestReceivedTimestamp,
		}, nil
	}

	// Track replicas even if we haven't emitted yet
	// When we later see a full spec (update/patch), we'll emit create
	// and subsequent scale changes will be detected
	p.deploymentReplicas[key] = scale.Spec.Replicas

	return nil, nil
}

func (p *Parser) parseJobEvent(event AuditEvent) (*WorkloadResult, error) {
	// Skip subresources (like /status)
	if event.ObjectRef.Subresource != "" {
		return nil, nil
	}

	var job batchv1.Job
	if event.ResponseObject != nil {
		if err := json.Unmarshal(event.ResponseObject, &job); err != nil {
			return nil, fmt.Errorf("failed to parse job from response object: %w", err)
		}
	} else if event.RequestObject != nil {
		if err := json.Unmarshal(event.RequestObject, &job); err != nil {
			return nil, fmt.Errorf("failed to parse job from request object: %w", err)
		}
	} else {
		return nil, nil
	}

	key := event.ObjectRef.Namespace + "/" + event.ObjectRef.Name

	switch event.Verb {
	case "create":
		// Track creation time for duration calculation
		p.jobCreateTimes[key] = event.RequestReceivedTimestamp
		return &WorkloadResult{
			Job:       &job,
			Timestamp: event.RequestReceivedTimestamp,
		}, nil

	case "update", "patch":
		// Check if job just completed (has completionTime set)
		if job.Status.CompletionTime != nil {
			return &WorkloadResult{
				JobCompleteEvent: &JobCompleteEvent{
					Namespace:      job.Namespace,
					Name:           job.Name,
					CompletionTime: job.Status.CompletionTime.Time,
				},
				Timestamp: event.RequestReceivedTimestamp,
			}, nil
		}
	}

	return nil, nil
}

