/*
Copyright 2018 The Kubernetes Authors.

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

package admission

import (
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Admission Webhook Response Helpers", func() {
	Describe("Allowed", func() {
		It("should return an 'allowed' response", func() {
			Expect(Allowed("")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Code: http.StatusOK,
						},
					},
				},
			))
		})

		It("should populate a status with a reason when a reason is given", func() {
			Expect(Allowed("acceptable")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Code:    http.StatusOK,
							Message: "acceptable",
						},
					},
				},
			))
		})
	})

	Describe("Denied", func() {
		It("should return a 'not allowed' response", func() {
			Expect(Denied("")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:   http.StatusForbidden,
							Reason: metav1.StatusReasonForbidden,
						},
					},
				},
			))
		})

		It("should populate a status with a reason when a reason is given", func() {
			Expect(Denied("UNACCEPTABLE!")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:    http.StatusForbidden,
							Reason:  metav1.StatusReasonForbidden,
							Message: "UNACCEPTABLE!",
						},
					},
				},
			))
		})
	})

	Describe("Patched", func() {
		ops := []jsonpatch.JsonPatchOperation{
			{
				Operation: "replace",
				Path:      "/spec/selector/matchLabels",
				Value:     map[string]string{"foo": "bar"},
			},
			{
				Operation: "delete",
				Path:      "/spec/replicas",
			},
		}
		It("should return an 'allowed' response with the given patches", func() {
			Expect(Patched("", ops...)).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Code: http.StatusOK,
						},
					},
					Patches: ops,
				},
			))
		})
		It("should populate a status with a reason when a reason is given", func() {
			Expect(Patched("some changes", ops...)).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Code:    http.StatusOK,
							Message: "some changes",
						},
					},
					Patches: ops,
				},
			))
		})
	})

	Describe("Errored", func() {
		It("should return a denied response with an error", func() {
			err := errors.New("this is an error")
			expected := Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusBadRequest,
						Message: err.Error(),
					},
				},
			}
			resp := Errored(http.StatusBadRequest, err)
			Expect(resp).To(Equal(expected))
		})
	})

	Describe("ValidationResponse", func() {
		It("should populate a status with a message when a message is given", func() {
			By("checking that a message is populated for 'allowed' responses")
			Expect(ValidationResponse(true, "acceptable")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Code:    http.StatusOK,
							Message: "acceptable",
						},
					},
				},
			))

			By("checking that a message is populated for 'denied' responses")
			Expect(ValidationResponse(false, "UNACCEPTABLE!")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:    http.StatusForbidden,
							Reason:  metav1.StatusReasonForbidden,
							Message: "UNACCEPTABLE!",
						},
					},
				},
			))
		})

		It("should return an admission decision", func() {
			By("checking that it returns an 'allowed' response when allowed is true")
			Expect(ValidationResponse(true, "")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Code: http.StatusOK,
						},
					},
				},
			))

			By("checking that it returns an 'denied' response when allowed is false")
			Expect(ValidationResponse(false, "")).To(Equal(
				Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:   http.StatusForbidden,
							Reason: metav1.StatusReasonForbidden,
						},
					},
				},
			))
		})
	})

	Describe("PatchResponseFromRaw", func() {
		It("should return an 'allowed' response with a patch of the diff between two sets of serialized JSON", func() {
			expected := Response{
				Patches: []jsonpatch.JsonPatchOperation{
					{Operation: "replace", Path: "/a", Value: "bar"},
				},
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: func() *admissionv1.PatchType { pt := admissionv1.PatchTypeJSONPatch; return &pt }(),
				},
			}
			resp := PatchResponseFromRaw([]byte(`{"a": "foo"}`), []byte(`{"a": "bar"}`))
			Expect(resp).To(Equal(expected))
		})
	})

	Describe("WithWarnings", func() {
		It("should add the warnings to the existing response without removing any existing warnings", func() {
			initialResponse := Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code: http.StatusOK,
					},
					Warnings: []string{"existing-warning"},
				},
			}
			warnings := []string{"additional-warning-1", "additional-warning-2"}
			expectedResponse := Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code: http.StatusOK,
					},
					Warnings: []string{"existing-warning", "additional-warning-1", "additional-warning-2"},
				},
			}

			Expect(initialResponse.WithWarnings(warnings...)).To(Equal(expectedResponse))
		})
	})
})
