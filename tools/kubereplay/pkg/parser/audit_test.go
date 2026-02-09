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
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParser_DeploymentCreate(t *testing.T) {
	p := NewParser()

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "default",
		},
	}
	deploymentJSON, _ := json.Marshal(deployment)

	event := AuditEvent{
		Stage: "ResponseComplete",
		Verb:  "create",
		ObjectRef: ObjectReference{
			Resource:  "deployments",
			Namespace: "default",
			Name:      "test-deploy",
		},
		ResponseObject:           deploymentJSON,
		ResponseStatus:           &metav1.Status{Code: 201},
		RequestReceivedTimestamp: time.Now(),
	}

	result, err := p.ParseEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Deployment == nil {
		t.Fatal("expected deployment in result")
	}
	if result.PreExisting {
		t.Error("expected PreExisting=false for create event")
	}
}

func TestParser_PreExistingDeployment(t *testing.T) {
	p := NewParser()

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pre-existing",
			Namespace: "default",
		},
	}
	deploymentJSON, _ := json.Marshal(deployment)

	// First event is an update (no prior create seen) - this is a pre-existing deployment
	event := AuditEvent{
		Stage: "ResponseComplete",
		Verb:  "update",
		ObjectRef: ObjectReference{
			Resource:  "deployments",
			Namespace: "default",
			Name:      "pre-existing",
		},
		ResponseObject:           deploymentJSON,
		ResponseStatus:           &metav1.Status{Code: 200},
		RequestReceivedTimestamp: time.Now(),
	}

	result, err := p.ParseEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Deployment == nil {
		t.Fatal("expected deployment in result")
	}
	if !result.PreExisting {
		t.Error("expected PreExisting=true for deployment seen via update before create")
	}
}

func TestParser_SubsequentUpdate_NotPreExisting(t *testing.T) {
	p := NewParser()

	replicas := int32(1)
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
	deploymentJSON, _ := json.Marshal(deployment)

	// First: create event
	createEvent := AuditEvent{
		Stage: "ResponseComplete",
		Verb:  "create",
		ObjectRef: ObjectReference{
			Resource:  "deployments",
			Namespace: "default",
			Name:      "test-deploy",
		},
		ResponseObject:           deploymentJSON,
		ResponseStatus:           &metav1.Status{Code: 201},
		RequestReceivedTimestamp: time.Now(),
	}

	result, _ := p.ParseEvent(createEvent)
	if result.PreExisting {
		t.Error("create event should not be pre-existing")
	}

	// Second: update event (should not emit anything since replicas unchanged)
	updateEvent := AuditEvent{
		Stage: "ResponseComplete",
		Verb:  "update",
		ObjectRef: ObjectReference{
			Resource:  "deployments",
			Namespace: "default",
			Name:      "test-deploy",
		},
		ResponseObject:           deploymentJSON,
		ResponseStatus:           &metav1.Status{Code: 200},
		RequestReceivedTimestamp: time.Now(),
	}

	result, _ = p.ParseEvent(updateEvent)
	if result != nil {
		t.Error("update with no replica change should return nil")
	}
}

func TestParser_ScaleChange(t *testing.T) {
	p := NewParser()

	replicas := int32(1)
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
	deploymentJSON, _ := json.Marshal(deployment)

	// Create
	createEvent := AuditEvent{
		Stage: "ResponseComplete",
		Verb:  "create",
		ObjectRef: ObjectReference{
			Resource:  "deployments",
			Namespace: "default",
			Name:      "test-deploy",
		},
		ResponseObject:           deploymentJSON,
		ResponseStatus:           &metav1.Status{Code: 201},
		RequestReceivedTimestamp: time.Now(),
	}
	p.ParseEvent(createEvent)

	// Scale up
	replicas = 3
	deployment.Spec.Replicas = &replicas
	deploymentJSON, _ = json.Marshal(deployment)

	updateEvent := AuditEvent{
		Stage: "ResponseComplete",
		Verb:  "update",
		ObjectRef: ObjectReference{
			Resource:  "deployments",
			Namespace: "default",
			Name:      "test-deploy",
		},
		ResponseObject:           deploymentJSON,
		ResponseStatus:           &metav1.Status{Code: 200},
		RequestReceivedTimestamp: time.Now(),
	}

	result, err := p.ParseEvent(updateEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected scale event result")
	}
	if result.ScaleEvent == nil {
		t.Fatal("expected ScaleEvent in result")
	}
	if result.ScaleEvent.Replicas != 3 {
		t.Errorf("expected replicas=3, got %d", result.ScaleEvent.Replicas)
	}
}

func TestParser_ExcludedNamespaces(t *testing.T) {
	p := NewParser()

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "kube-system",
		},
	}
	deploymentJSON, _ := json.Marshal(deployment)

	event := AuditEvent{
		Stage: "ResponseComplete",
		Verb:  "create",
		ObjectRef: ObjectReference{
			Resource:  "deployments",
			Namespace: "kube-system",
			Name:      "test",
		},
		ResponseObject:           deploymentJSON,
		ResponseStatus:           &metav1.Status{Code: 201},
		RequestReceivedTimestamp: time.Now(),
	}

	result, err := p.ParseEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for excluded namespace")
	}
}
