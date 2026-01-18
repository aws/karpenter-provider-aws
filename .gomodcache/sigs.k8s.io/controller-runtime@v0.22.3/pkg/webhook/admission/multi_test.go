/*
Copyright 2019 The Kubernetes Authors.

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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
)

var _ = Describe("Multi-Handler Admission Webhooks", func() {
	alwaysAllow := &fakeHandler{
		fn: func(ctx context.Context, req Request) Response {
			return Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
				},
			}
		},
	}
	alwaysDeny := &fakeHandler{
		fn: func(ctx context.Context, req Request) Response {
			return Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
				},
			}
		},
	}

	withWarnings := &fakeHandler{
		fn: func(ctx context.Context, req Request) Response {
			return Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:  true,
					Warnings: []string{"handler-warning"},
				},
			}
		},
	}

	Context("with validating handlers", func() {
		It("should deny the request if any handler denies the request", func(ctx SpecContext) {
			By("setting up a handler with accept and deny")
			handler := MultiValidatingHandler(alwaysAllow, alwaysDeny)

			By("checking that the handler denies the request")
			resp := handler.Handle(ctx, Request{})
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Warnings).To(BeEmpty())
		})

		It("should allow the request if all handlers allow the request", func(ctx SpecContext) {
			By("setting up a handler with only accept")
			handler := MultiValidatingHandler(alwaysAllow, alwaysAllow)

			By("checking that the handler allows the request")
			resp := handler.Handle(ctx, Request{})
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Warnings).To(BeEmpty())
		})

		It("should show the warnings if all handlers allow the request", func(ctx SpecContext) {
			By("setting up a handler with only accept")
			handler := MultiValidatingHandler(alwaysAllow, withWarnings)

			By("checking that the handler allows the request")
			resp := handler.Handle(ctx, Request{})
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Warnings).To(HaveLen(1))
		})
	})

	Context("with mutating handlers", func() {
		patcher1 := &fakeHandler{
			fn: func(ctx context.Context, req Request) Response {
				return Response{
					Patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "add",
							Path:      "/metadata/annotation/new-key",
							Value:     "new-value",
						},
						{
							Operation: "replace",
							Path:      "/spec/replicas",
							Value:     "2",
						},
					},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed:   true,
						PatchType: func() *admissionv1.PatchType { pt := admissionv1.PatchTypeJSONPatch; return &pt }(),
					},
				}
			},
		}
		patcher2 := &fakeHandler{
			fn: func(ctx context.Context, req Request) Response {
				return Response{
					Patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "add",
							Path:      "/metadata/annotation/hello",
							Value:     "world",
						},
					},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed:   true,
						PatchType: func() *admissionv1.PatchType { pt := admissionv1.PatchTypeJSONPatch; return &pt }(),
					},
				}
			},
		}

		patcher3 := &fakeHandler{
			fn: func(ctx context.Context, req Request) Response {
				return Response{
					Patches: []jsonpatch.JsonPatchOperation{
						{
							Operation: "add",
							Path:      "/metadata/annotation/newest-key",
							Value:     "value",
						},
					},
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed:   true,
						Warnings:  []string{"annotation-warning"},
						PatchType: func() *admissionv1.PatchType { pt := admissionv1.PatchTypeJSONPatch; return &pt }(),
					},
				}
			},
		}

		It("should not return any patches if the request is denied", func(ctx SpecContext) {
			By("setting up a webhook with some patches and a deny")
			handler := MultiMutatingHandler(patcher1, patcher2, alwaysDeny)

			By("checking that the handler denies the request and produces no patches")
			resp := handler.Handle(ctx, Request{})
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Patches).To(BeEmpty())
		})

		It("should produce all patches if the requests are all allowed", func(ctx SpecContext) {
			By("setting up a webhook with some patches")
			handler := MultiMutatingHandler(patcher1, patcher2, alwaysAllow)

			By("checking that the handler accepts the request and returns all patches")
			resp := handler.Handle(ctx, Request{})
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Patch).To(Equal([]byte(
				`[{"op":"add","path":"/metadata/annotation/new-key","value":"new-value"},` +
					`{"op":"replace","path":"/spec/replicas","value":"2"},{"op":"add","path":"/metadata/annotation/hello","value":"world"}]`)))
		})

		It("should produce all patches if the requests are all allowed and show warnings", func(ctx SpecContext) {
			By("setting up a webhook with some patches")
			handler := MultiMutatingHandler(patcher1, patcher2, alwaysAllow, patcher3)

			By("checking that the handler accepts the request and returns all patches")
			resp := handler.Handle(ctx, Request{})
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Patch).To(Equal([]byte(
				`[{"op":"add","path":"/metadata/annotation/new-key","value":"new-value"},` +
					`{"op":"replace","path":"/spec/replicas","value":"2"},{"op":"add","path":"/metadata/annotation/hello","value":"world"},` +
					`{"op":"add","path":"/metadata/annotation/newest-key","value":"value"}]`)))
			Expect(resp.Warnings).To(HaveLen(1))
		})

	})
})
