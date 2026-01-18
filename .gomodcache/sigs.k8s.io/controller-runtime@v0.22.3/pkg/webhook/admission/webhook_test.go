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
	"context"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("Admission Webhooks", func() {
	var (
		logBuffer  *gbytes.Buffer
		testLogger logr.Logger
	)

	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		testLogger = zap.New(zap.JSONEncoder(), zap.WriteTo(io.MultiWriter(logBuffer, GinkgoWriter)))
	})

	allowHandler := func() *Webhook {
		handler := &fakeHandler{
			fn: func(ctx context.Context, req Request) Response {
				return Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}
			},
		}
		webhook := &Webhook{
			Handler: handler,
		}

		return webhook
	}

	It("should invoke the handler to get a response", func(ctx SpecContext) {
		By("setting up a webhook with an allow handler")
		webhook := allowHandler()

		By("invoking the webhook")
		resp := webhook.Handle(ctx, Request{})

		By("checking that it allowed the request")
		Expect(resp.Allowed).To(BeTrue())
	})

	It("should ensure that the response's UID is set to the request's UID", func(ctx SpecContext) {
		By("setting up a webhook")
		webhook := allowHandler()

		By("invoking the webhook")
		resp := webhook.Handle(ctx, Request{AdmissionRequest: admissionv1.AdmissionRequest{UID: "foobar"}})

		By("checking that the response share's the request's UID")
		Expect(resp.UID).To(Equal(machinerytypes.UID("foobar")))
	})

	It("should populate the status on a response if one is not provided", func(ctx SpecContext) {
		By("setting up a webhook")
		webhook := allowHandler()

		By("invoking the webhook")
		resp := webhook.Handle(ctx, Request{})

		By("checking that the response share's the request's UID")
		Expect(resp.Result).To(Equal(&metav1.Status{Code: http.StatusOK}))
	})

	It("shouldn't overwrite the status on a response", func(ctx SpecContext) {
		By("setting up a webhook that sets a status")
		webhook := &Webhook{
			Handler: HandlerFunc(func(ctx context.Context, req Request) Response {
				return Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
						Result:  &metav1.Status{Message: "Ground Control to Major Tom"},
					},
				}
			}),
		}

		By("invoking the webhook")
		resp := webhook.Handle(ctx, Request{})

		By("checking that the message is intact")
		Expect(resp.Result).NotTo(BeNil())
		Expect(resp.Result.Message).To(Equal("Ground Control to Major Tom"))
	})

	It("should serialize patch operations into a single jsonpatch blob", func(ctx SpecContext) {
		By("setting up a webhook with a patching handler")
		webhook := &Webhook{
			Handler: HandlerFunc(func(ctx context.Context, req Request) Response {
				return Patched("", jsonpatch.Operation{Operation: "add", Path: "/a", Value: 2}, jsonpatch.Operation{Operation: "replace", Path: "/b", Value: 4})
			}),
		}

		By("invoking the webhook")
		resp := webhook.Handle(ctx, Request{})

		By("checking that a JSON patch is populated on the response")
		patchType := admissionv1.PatchTypeJSONPatch
		Expect(resp.PatchType).To(Equal(&patchType))
		Expect(resp.Patch).To(Equal([]byte(`[{"op":"add","path":"/a","value":2},{"op":"replace","path":"/b","value":4}]`)))
	})

	It("should pass a request logger via the context", func(ctx SpecContext) {
		By("setting up a webhook that uses the request logger")
		webhook := &Webhook{
			Handler: HandlerFunc(func(ctx context.Context, req Request) Response {
				logf.FromContext(ctx).Info("Received request")

				return Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}
			}),
			log: testLogger,
		}

		By("invoking the webhook")
		resp := webhook.Handle(ctx, Request{AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test123",
			Name:      "foo",
			Namespace: "bar",
			Resource: metav1.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			UserInfo: authenticationv1.UserInfo{
				Username: "tim",
			},
		}})
		Expect(resp.Allowed).To(BeTrue())

		By("checking that the log message contains the request fields")
		Eventually(logBuffer).Should(gbytes.Say(`"msg":"Received request","object":{"name":"foo","namespace":"bar"},"namespace":"bar","name":"foo","resource":{"group":"apps","version":"v1","resource":"deployments"},"user":"tim","requestID":"test123"}`))
	})

	It("should pass a request logger created by LogConstructor via the context", func(ctx SpecContext) {
		By("setting up a webhook that uses the request logger")
		webhook := &Webhook{
			Handler: HandlerFunc(func(ctx context.Context, req Request) Response {
				logf.FromContext(ctx).Info("Received request")

				return Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: true,
					},
				}
			}),
			LogConstructor: func(base logr.Logger, req *Request) logr.Logger {
				return base.WithValues("operation", req.Operation, "requestID", req.UID)
			},
			log: testLogger,
		}

		By("invoking the webhook")
		resp := webhook.Handle(ctx, Request{AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test123",
			Operation: admissionv1.Create,
		}})
		Expect(resp.Allowed).To(BeTrue())

		By("checking that the log message contains the request fields")
		Eventually(logBuffer).Should(gbytes.Say(`"msg":"Received request","operation":"CREATE","requestID":"test123"}`))
	})

	Describe("panic recovery", func() {
		It("should recover panic if RecoverPanic is true by default", func(ctx SpecContext) {
			panicHandler := func() *Webhook {
				handler := &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						panic("fake panic test")
					},
				}
				webhook := &Webhook{
					Handler: handler,
					// RecoverPanic defaults to true.
				}

				return webhook
			}

			By("setting up a webhook with a panicking handler")
			webhook := panicHandler()

			By("invoking the webhook")
			resp := webhook.Handle(ctx, Request{})

			By("checking that it errored the request")
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Code).To(Equal(int32(http.StatusInternalServerError)))
			Expect(resp.Result.Message).To(Equal("panic: fake panic test [recovered]"))
		})

		It("should recover panic if RecoverPanic is true", func(ctx SpecContext) {
			panicHandler := func() *Webhook {
				handler := &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						panic("fake panic test")
					},
				}
				webhook := &Webhook{
					Handler:      handler,
					RecoverPanic: ptr.To[bool](true),
				}

				return webhook
			}

			By("setting up a webhook with a panicking handler")
			webhook := panicHandler()

			By("invoking the webhook")
			resp := webhook.Handle(ctx, Request{})

			By("checking that it errored the request")
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Code).To(Equal(int32(http.StatusInternalServerError)))
			Expect(resp.Result.Message).To(Equal("panic: fake panic test [recovered]"))
		})

		It("should not recover panic if RecoverPanic is false", func(ctx SpecContext) {
			panicHandler := func() *Webhook {
				handler := &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						panic("fake panic test")
					},
				}
				webhook := &Webhook{
					Handler:      handler,
					RecoverPanic: ptr.To[bool](false),
				}

				return webhook
			}

			By("setting up a webhook with a panicking handler")
			defer func() {
				Expect(recover()).ShouldNot(BeNil())
			}()
			webhook := panicHandler()

			By("invoking the webhook")
			webhook.Handle(ctx, Request{})
		})
	})
})

var _ = It("Should be able to write/read admission.Request to/from context", func(specContext SpecContext) {
	testRequest := Request{
		admissionv1.AdmissionRequest{
			UID: "test-uid",
		},
	}

	ctx := NewContextWithRequest(specContext, testRequest)

	gotRequest, err := RequestFromContext(ctx)
	Expect(err).To(Not(HaveOccurred()))
	Expect(gotRequest).To(Equal(testRequest))
})
