/*
Copyright 2021 The Kubernetes Authors.

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

package spec3_test

import (
	"encoding/json"
	"testing"

	"k8s.io/kube-openapi/pkg/util/jsontesting"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"reflect"

	"k8s.io/kube-openapi/pkg/spec3"
)

func TestSchemasJSONSerialization(t *testing.T) {
	cases := []struct {
		name           string
		target         spec3.Components
		expectedOutput string
	}{
		{
			name: "scenario1: smoke test serialization of spec3.Components.Schemas",
			target: spec3.Components{
				Schemas: map[string]*spec.Schema{
					"io.k8s.api.admissionregistration.v1beta1.MutatingWebhook": {
						SchemaProps: spec.SchemaProps{
							Description: "MutatingWebhook describes an admission webhook and the resources and operations it applies to.",
							Type:        []string{"object"},
							Properties: map[string]spec.Schema{
								"name": {
									SchemaProps: spec.SchemaProps{
										Description: "The name of the admission webhook. Name should be fully qualified, e.g., imagepolicy.kubernetes.io, where \"imagepolicy\" is the name of the webhook, and kubernetes.io is the name of the organization. Required.",
										Type:        []string{"string"},
										Format:      "",
									},
								},
								"clientConfig": {
									SchemaProps: spec.SchemaProps{
										Description: "ClientConfig defines how to communicate with the hook. Required",
										Ref:         spec.MustCreateRef("k8s.io/api/admissionregistration/v1beta1.WebhookClientConfig"),
									},
								},
								"rules": {
									SchemaProps: spec.SchemaProps{
										Description: "Rules describes what operations on what resources/subresources the webhook cares about. The webhook cares about an operation if it matches _any_ Rule. However, in order to prevent ValidatingAdmissionWebhooks and MutatingAdmissionWebhooks from putting the cluster in a state which cannot be recovered from without completely disabling the plugin, ValidatingAdmissionWebhooks and MutatingAdmissionWebhooks are never called on admission requests for ValidatingWebhookConfiguration and MutatingWebhookConfiguration objects.",
										Type:        []string{"array"},
										Items: &spec.SchemaOrArray{
											Schema: &spec.Schema{
												SchemaProps: spec.SchemaProps{
													Ref: spec.MustCreateRef("k8s.io/api/admissionregistration/v1beta1.RuleWithOperations"),
												},
											},
										},
									},
								},
								"failurePolicy": {
									SchemaProps: spec.SchemaProps{
										Description: "FailurePolicy defines how unrecognized errors from the admission endpoint are handled - allowed values are Ignore or Fail. Defaults to Ignore.",
										Type:        []string{"string"},
										Format:      "",
									},
								},
								"matchPolicy": {
									SchemaProps: spec.SchemaProps{
										Description: "matchPolicy defines how the \"rules\" list is used to match incoming requests. Allowed values are \"Exact\" or \"Equivalent\".\n\n- Exact: match a request only if it exactly matches a specified rule. For example, if deployments can be modified via apps/v1, apps/v1beta1, and extensions/v1beta1, but \"rules\" only included apiGroups:[\"apps\"], apiVersions:[\"v1\"], resources: [\"deployments\"], a request to apps/v1beta1 or extensions/v1beta1 would not be sent to the webhook.\n\n- Equivalent: match a request if modifies a resource listed in rules, even via another API group or version. For example, if deployments can be modified via apps/v1, apps/v1beta1, and extensions/v1beta1, and \"rules\" only included apiGroups:[\"apps\"], apiVersions:[\"v1\"], resources: [\"deployments\"], a request to apps/v1beta1 or extensions/v1beta1 would be converted to apps/v1 and sent to the webhook.\n\nDefaults to \"Exact\"",
										Type:        []string{"string"},
										Format:      "",
									},
								},
								"namespaceSelector": {
									SchemaProps: spec.SchemaProps{
										Description: "NamespaceSelector decides whether to run the webhook on an object based on whether the namespace for that object matches the selector. If the object itself is a namespace, the matching is performed on object.metadata.labels. If the object is another cluster scoped resource, it never skips the webhook.\n\nFor example, to run the webhook on any objects whose namespace is not associated with \"runlevel\" of \"0\" or \"1\";  you will set the selector as follows: \"namespaceSelector\": {\n  \"matchExpressions\": [\n    {\n      \"key\": \"runlevel\",\n      \"operator\": \"NotIn\",\n      \"values\": [\n        \"0\",\n        \"1\"\n      ]\n    }\n  ]\n}\n\nIf instead you want to only run the webhook on any objects whose namespace is associated with the \"environment\" of \"prod\" or \"staging\"; you will set the selector as follows: \"namespaceSelector\": {\n  \"matchExpressions\": [\n    {\n      \"key\": \"environment\",\n      \"operator\": \"In\",\n      \"values\": [\n        \"prod\",\n        \"staging\"\n      ]\n    }\n  ]\n}\n\nSee https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ for more examples of label selectors.\n\nDefault to the empty LabelSelector, which matches everything.",
										Ref:         spec.MustCreateRef("k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector"),
									},
								},
								"objectSelector": {
									SchemaProps: spec.SchemaProps{
										Description: "ObjectSelector decides whether to run the webhook based on if the object has matching labels. objectSelector is evaluated against both the oldObject and newObject that would be sent to the webhook, and is considered to match if either object matches the selector. A null object (oldObject in the case of create, or newObject in the case of delete) or an object that cannot have labels (like a DeploymentRollback or a PodProxyOptions object) is not considered to match. Use the object selector only if the webhook is opt-in, because end users may skip the admission webhook by setting the labels. Default to the empty LabelSelector, which matches everything.",
										Ref:         spec.MustCreateRef("k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector"),
									},
								},
								"sideEffects": {
									SchemaProps: spec.SchemaProps{
										Description: "SideEffects states whether this webhookk has side effects. Acceptable values are: Unknown, None, Some, NoneOnDryRun Webhooks with side effects MUST implement a reconciliation system, since a request may be rejected by a future step in the admission change and the side effects therefore need to be undone. Requests with the dryRun attribute will be auto-rejected if they match a webhook with sideEffects == Unknown or Some. Defaults to Unknown.",
										Type:        []string{"string"},
										Format:      "",
									},
								},
								"timeoutSeconds": {
									SchemaProps: spec.SchemaProps{
										Description: "TimeoutSeconds specifies the timeout for this webhook. After the timeout passes, the webhook call will be ignored or the API call will fail based on the failure policy. The timeout value must be between 1 and 30 seconds. Default to 30 seconds.",
										Type:        []string{"integer"},
										Format:      "int32",
									},
								},
								"admissionReviewVersions": {
									SchemaProps: spec.SchemaProps{
										Description: "AdmissionReviewVersions is an ordered list of preferred AdmissionReview versions the Webhook expects. API server will try to use first version in the list which it supports. If none of the versions specified in this list supported by API server, validation will fail for this object. If a persisted webhook configuration specifies allowed versions and does not include any versions known to the API Server, calls to the webhook will fail and be subject to the failure policy. Default to ['v1beta1'].",
										Type:        []string{"array"},
										Items: &spec.SchemaOrArray{
											Schema: &spec.Schema{
												SchemaProps: spec.SchemaProps{
													Type:   []string{"string"},
													Format: "",
												},
											},
										},
									},
								},
								"reinvocationPolicy": {
									SchemaProps: spec.SchemaProps{
										Description: "reinvocationPolicy indicates whether this webhook should be called multiple times as part of a single admission evaluation. Allowed values are \"Never\" and \"IfNeeded\".\n\nNever: the webhook will not be called more than once in a single admission evaluation.\n\nIfNeeded: the webhook will be called at least one additional time as part of the admission evaluation if the object being admitted is modified by other admission plugins after the initial webhook call. Webhooks that specify this option *must* be idempotent, able to process objects they previously admitted. Note: * the number of additional invocations is not guaranteed to be exactly one. * if additional invocations result in further modifications to the object, webhooks are not guaranteed to be invoked again. * webhooks that use this option may be reordered to minimize the number of additional invocations. * to validate an object after all mutations are guaranteed complete, use a validating admission webhook instead.\n\nDefaults to \"Never\".",
										Type:        []string{"string"},
										Format:      "",
									},
								},
							},
							Required: []string{"name", "clientConfig"},
						},
					},
				},
			},
			expectedOutput: `{"schemas":{"io.k8s.api.admissionregistration.v1beta1.MutatingWebhook":{"description":"MutatingWebhook describes an admission webhook and the resources and operations it applies to.","type":"object","required":["name","clientConfig"],"properties":{"admissionReviewVersions":{"description":"AdmissionReviewVersions is an ordered list of preferred AdmissionReview versions the Webhook expects. API server will try to use first version in the list which it supports. If none of the versions specified in this list supported by API server, validation will fail for this object. If a persisted webhook configuration specifies allowed versions and does not include any versions known to the API Server, calls to the webhook will fail and be subject to the failure policy. Default to ['v1beta1'].","type":"array","items":{"type":"string"}},"clientConfig":{"description":"ClientConfig defines how to communicate with the hook. Required","$ref":"k8s.io/api/admissionregistration/v1beta1.WebhookClientConfig"},"failurePolicy":{"description":"FailurePolicy defines how unrecognized errors from the admission endpoint are handled - allowed values are Ignore or Fail. Defaults to Ignore.","type":"string"},"matchPolicy":{"description":"matchPolicy defines how the \"rules\" list is used to match incoming requests. Allowed values are \"Exact\" or \"Equivalent\".\n\n- Exact: match a request only if it exactly matches a specified rule. For example, if deployments can be modified via apps/v1, apps/v1beta1, and extensions/v1beta1, but \"rules\" only included apiGroups:[\"apps\"], apiVersions:[\"v1\"], resources: [\"deployments\"], a request to apps/v1beta1 or extensions/v1beta1 would not be sent to the webhook.\n\n- Equivalent: match a request if modifies a resource listed in rules, even via another API group or version. For example, if deployments can be modified via apps/v1, apps/v1beta1, and extensions/v1beta1, and \"rules\" only included apiGroups:[\"apps\"], apiVersions:[\"v1\"], resources: [\"deployments\"], a request to apps/v1beta1 or extensions/v1beta1 would be converted to apps/v1 and sent to the webhook.\n\nDefaults to \"Exact\"","type":"string"},"name":{"description":"The name of the admission webhook. Name should be fully qualified, e.g., imagepolicy.kubernetes.io, where \"imagepolicy\" is the name of the webhook, and kubernetes.io is the name of the organization. Required.","type":"string"},"namespaceSelector":{"description":"NamespaceSelector decides whether to run the webhook on an object based on whether the namespace for that object matches the selector. If the object itself is a namespace, the matching is performed on object.metadata.labels. If the object is another cluster scoped resource, it never skips the webhook.\n\nFor example, to run the webhook on any objects whose namespace is not associated with \"runlevel\" of \"0\" or \"1\";  you will set the selector as follows: \"namespaceSelector\": {\n  \"matchExpressions\": [\n    {\n      \"key\": \"runlevel\",\n      \"operator\": \"NotIn\",\n      \"values\": [\n        \"0\",\n        \"1\"\n      ]\n    }\n  ]\n}\n\nIf instead you want to only run the webhook on any objects whose namespace is associated with the \"environment\" of \"prod\" or \"staging\"; you will set the selector as follows: \"namespaceSelector\": {\n  \"matchExpressions\": [\n    {\n      \"key\": \"environment\",\n      \"operator\": \"In\",\n      \"values\": [\n        \"prod\",\n        \"staging\"\n      ]\n    }\n  ]\n}\n\nSee https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ for more examples of label selectors.\n\nDefault to the empty LabelSelector, which matches everything.","$ref":"k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector"},"objectSelector":{"description":"ObjectSelector decides whether to run the webhook based on if the object has matching labels. objectSelector is evaluated against both the oldObject and newObject that would be sent to the webhook, and is considered to match if either object matches the selector. A null object (oldObject in the case of create, or newObject in the case of delete) or an object that cannot have labels (like a DeploymentRollback or a PodProxyOptions object) is not considered to match. Use the object selector only if the webhook is opt-in, because end users may skip the admission webhook by setting the labels. Default to the empty LabelSelector, which matches everything.","$ref":"k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector"},"reinvocationPolicy":{"description":"reinvocationPolicy indicates whether this webhook should be called multiple times as part of a single admission evaluation. Allowed values are \"Never\" and \"IfNeeded\".\n\nNever: the webhook will not be called more than once in a single admission evaluation.\n\nIfNeeded: the webhook will be called at least one additional time as part of the admission evaluation if the object being admitted is modified by other admission plugins after the initial webhook call. Webhooks that specify this option *must* be idempotent, able to process objects they previously admitted. Note: * the number of additional invocations is not guaranteed to be exactly one. * if additional invocations result in further modifications to the object, webhooks are not guaranteed to be invoked again. * webhooks that use this option may be reordered to minimize the number of additional invocations. * to validate an object after all mutations are guaranteed complete, use a validating admission webhook instead.\n\nDefaults to \"Never\".","type":"string"},"rules":{"description":"Rules describes what operations on what resources/subresources the webhook cares about. The webhook cares about an operation if it matches _any_ Rule. However, in order to prevent ValidatingAdmissionWebhooks and MutatingAdmissionWebhooks from putting the cluster in a state which cannot be recovered from without completely disabling the plugin, ValidatingAdmissionWebhooks and MutatingAdmissionWebhooks are never called on admission requests for ValidatingWebhookConfiguration and MutatingWebhookConfiguration objects.","type":"array","items":{"$ref":"k8s.io/api/admissionregistration/v1beta1.RuleWithOperations"}},"sideEffects":{"description":"SideEffects states whether this webhookk has side effects. Acceptable values are: Unknown, None, Some, NoneOnDryRun Webhooks with side effects MUST implement a reconciliation system, since a request may be rejected by a future step in the admission change and the side effects therefore need to be undone. Requests with the dryRun attribute will be auto-rejected if they match a webhook with sideEffects == Unknown or Some. Defaults to Unknown.","type":"string"},"timeoutSeconds":{"description":"TimeoutSeconds specifies the timeout for this webhook. After the timeout passes, the webhook call will be ignored or the API call will fail based on the failure policy. The timeout value must be between 1 and 30 seconds. Default to 30 seconds.","type":"integer","format":"int32"}}}}}`,
		},

		// scenario 2
		{
			name: "scenario2: schema can be defined as a ref, see: https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.0.md#componentsObject",
			target: spec3.Components{
				Schemas: map[string]*spec.Schema{
					"io.k8s.api.admissionregistration.v1beta1.MutatingWebhook": {
						SchemaProps: spec.SchemaProps{
							Ref: spec.MustCreateRef("k8s.io/api/admissionregistration/v1beta1.WebhookClientConfig"),
						},
					},
				},
			},
			expectedOutput: `{"schemas":{"io.k8s.api.admissionregistration.v1beta1.MutatingWebhook":{"$ref":"k8s.io/api/admissionregistration/v1beta1.WebhookClientConfig"}}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawTarget, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			if err := jsontesting.JsonCompare([]byte(tc.expectedOutput), rawTarget); err != nil {
				t.Error(err)
			}

			var expected spec3.Components
			json.Unmarshal(rawTarget, &expected)

			if !reflect.DeepEqual(expected, tc.target) {
				t.Fatalf("round trip error %s", tc.name)
			}
		})
	}
}
