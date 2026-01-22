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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuditEvent represents a Kubernetes audit log event
type AuditEvent struct {
	Kind                     string          `json:"kind"`
	APIVersion               string          `json:"apiVersion"`
	Stage                    string          `json:"stage"`
	Verb                     string          `json:"verb"`
	ObjectRef                ObjectReference `json:"objectRef"`
	RequestObject            json.RawMessage `json:"requestObject,omitempty"`
	ResponseObject           json.RawMessage `json:"responseObject,omitempty"`
	ResponseStatus           *metav1.Status  `json:"responseStatus,omitempty"`
	RequestReceivedTimestamp time.Time       `json:"requestReceivedTimestamp"`
}

// ObjectReference contains information about the object referenced in the audit event
type ObjectReference struct {
	Resource    string `json:"resource"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Subresource string `json:"subresource"`
}

// DefaultExcludedNamespaces lists namespaces to exclude by default
var DefaultExcludedNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}
