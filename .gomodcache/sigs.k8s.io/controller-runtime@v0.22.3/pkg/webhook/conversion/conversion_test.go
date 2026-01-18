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

package conversion_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
	jobsv1 "sigs.k8s.io/controller-runtime/pkg/webhook/conversion/testdata/api/v1"
	jobsv2 "sigs.k8s.io/controller-runtime/pkg/webhook/conversion/testdata/api/v2"
	jobsv3 "sigs.k8s.io/controller-runtime/pkg/webhook/conversion/testdata/api/v3"
)

var _ = Describe("Conversion Webhook", func() {

	var respRecorder *httptest.ResponseRecorder
	var decoder *conversion.Decoder
	var scheme *runtime.Scheme
	var wh http.Handler

	BeforeEach(func() {
		respRecorder = &httptest.ResponseRecorder{
			Body: bytes.NewBuffer(nil),
		}

		scheme = runtime.NewScheme()
		Expect(kscheme.AddToScheme(scheme)).To(Succeed())
		Expect(jobsv1.AddToScheme(scheme)).To(Succeed())
		Expect(jobsv2.AddToScheme(scheme)).To(Succeed())
		Expect(jobsv3.AddToScheme(scheme)).To(Succeed())

		decoder = conversion.NewDecoder(scheme)
		wh = conversion.NewWebhookHandler(scheme)
	})

	doRequest := func(convReq *apix.ConversionReview) *apix.ConversionReview {
		var payload bytes.Buffer

		Expect(json.NewEncoder(&payload).Encode(convReq)).Should(Succeed())

		convReview := &apix.ConversionReview{}
		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(payload.Bytes())),
		}
		wh.ServeHTTP(respRecorder, req)
		Expect(json.NewDecoder(respRecorder.Result().Body).Decode(convReview)).To(Succeed())
		return convReview
	}

	makeV1Obj := func() *jobsv1.ExternalJob {
		return &jobsv1.ExternalJob{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ExternalJob",
				APIVersion: "jobs.testprojects.kb.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "obj-1",
			},
			Spec: jobsv1.ExternalJobSpec{
				RunAt: "every 2 seconds",
			},
		}
	}

	makeV2Obj := func() *jobsv2.ExternalJob {
		return &jobsv2.ExternalJob{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ExternalJob",
				APIVersion: "jobs.testprojects.kb.io/v2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "obj-1",
			},
			Spec: jobsv2.ExternalJobSpec{
				ScheduleAt: "every 2 seconds",
			},
		}
	}

	It("should convert spoke to hub successfully", func() {

		v1Obj := makeV1Obj()

		expected := &jobsv2.ExternalJob{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ExternalJob",
				APIVersion: "jobs.testprojects.kb.io/v2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "obj-1",
			},
			Spec: jobsv2.ExternalJobSpec{
				ScheduleAt: "every 2 seconds",
			},
		}

		convReq := &apix.ConversionReview{
			TypeMeta: metav1.TypeMeta{},
			Request: &apix.ConversionRequest{
				DesiredAPIVersion: "jobs.testprojects.kb.io/v2",
				Objects: []runtime.RawExtension{
					{
						Object: v1Obj,
					},
				},
			},
		}

		convReview := doRequest(convReq)

		Expect(convReview.Response.ConvertedObjects).To(HaveLen(1))
		Expect(convReview.Response.Result.Status).To(Equal(metav1.StatusSuccess))
		got, _, err := decoder.Decode(convReview.Response.ConvertedObjects[0].Raw)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(expected))
	})

	It("should convert hub to spoke successfully", func() {

		v2Obj := makeV2Obj()

		expected := &jobsv1.ExternalJob{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ExternalJob",
				APIVersion: "jobs.testprojects.kb.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "obj-1",
			},
			Spec: jobsv1.ExternalJobSpec{
				RunAt: "every 2 seconds",
			},
		}

		convReq := &apix.ConversionReview{
			TypeMeta: metav1.TypeMeta{},
			Request: &apix.ConversionRequest{
				DesiredAPIVersion: "jobs.testprojects.kb.io/v1",
				Objects: []runtime.RawExtension{
					{
						Object: v2Obj,
					},
				},
			},
		}

		convReview := doRequest(convReq)

		Expect(convReview.Response.ConvertedObjects).To(HaveLen(1))
		Expect(convReview.Response.Result.Status).To(Equal(metav1.StatusSuccess))
		got, _, err := decoder.Decode(convReview.Response.ConvertedObjects[0].Raw)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(expected))
	})

	It("should convert spoke to spoke successfully", func() {

		v1Obj := makeV1Obj()

		expected := &jobsv3.ExternalJob{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ExternalJob",
				APIVersion: "jobs.testprojects.kb.io/v3",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "obj-1",
			},
			Spec: jobsv3.ExternalJobSpec{
				DeferredAt: "every 2 seconds",
			},
		}

		convReq := &apix.ConversionReview{
			TypeMeta: metav1.TypeMeta{},
			Request: &apix.ConversionRequest{
				DesiredAPIVersion: "jobs.testprojects.kb.io/v3",
				Objects: []runtime.RawExtension{
					{
						Object: v1Obj,
					},
				},
			},
		}

		convReview := doRequest(convReq)

		Expect(convReview.Response.ConvertedObjects).To(HaveLen(1))
		Expect(convReview.Response.Result.Status).To(Equal(metav1.StatusSuccess))
		got, _, err := decoder.Decode(convReview.Response.ConvertedObjects[0].Raw)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(expected))
	})

	It("should return error when dest/src objects belong to different API groups", func() {
		v1Obj := makeV1Obj()

		convReq := &apix.ConversionReview{
			TypeMeta: metav1.TypeMeta{},
			Request: &apix.ConversionRequest{
				// request conversion for different group
				DesiredAPIVersion: "jobss.example.org/v2",
				Objects: []runtime.RawExtension{
					{
						Object: v1Obj,
					},
				},
			},
		}

		convReview := doRequest(convReq)
		Expect(convReview.Response.Result.Status).To(Equal("Failure"))
		Expect(convReview.Response.ConvertedObjects).To(BeEmpty())
	})

	It("should return error when dest/src objects are of same type", func() {

		v1Obj := makeV1Obj()

		convReq := &apix.ConversionReview{
			TypeMeta: metav1.TypeMeta{},
			Request: &apix.ConversionRequest{
				DesiredAPIVersion: "jobs.testprojects.kb.io/v1",
				Objects: []runtime.RawExtension{
					{
						Object: v1Obj,
					},
				},
			},
		}

		convReview := doRequest(convReq)
		Expect(convReview.Response.Result.Status).To(Equal("Failure"))
		Expect(convReview.Response.ConvertedObjects).To(BeEmpty())
	})

	It("should return error when the API group does not have a hub defined", func() {

		v1Obj := &appsv1beta1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1beta1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "obj-1",
			},
		}

		convReq := &apix.ConversionReview{
			TypeMeta: metav1.TypeMeta{},
			Request: &apix.ConversionRequest{
				DesiredAPIVersion: "apps/v1",
				Objects: []runtime.RawExtension{
					{
						Object: v1Obj,
					},
				},
			},
		}

		convReview := doRequest(convReq)
		Expect(convReview.Response.Result.Status).To(Equal("Failure"))
		Expect(convReview.Response.ConvertedObjects).To(BeEmpty())
	})

	It("should return error on panic in conversion", func() {

		v1Obj := makeV1Obj()
		v1Obj.Spec.PanicInConversion = true

		convReq := &apix.ConversionReview{
			TypeMeta: metav1.TypeMeta{},
			Request: &apix.ConversionRequest{
				DesiredAPIVersion: "jobs.testprojects.kb.io/v3",
				Objects: []runtime.RawExtension{
					{
						Object: v1Obj,
					},
				},
			},
		}

		convReview := doRequest(convReq)

		Expect(convReview.Response.ConvertedObjects).To(HaveLen(0))
		Expect(convReview.Response.Result.Status).To(Equal(metav1.StatusFailure))
		Expect(convReview.Response.Result.Message).To(Equal("internal error occurred during conversion"))
	})
})

var _ = Describe("IsConvertible", func() {

	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()

		Expect(kscheme.AddToScheme(scheme)).To(Succeed())
		Expect(jobsv1.AddToScheme(scheme)).To(Succeed())
		Expect(jobsv2.AddToScheme(scheme)).To(Succeed())
		Expect(jobsv3.AddToScheme(scheme)).To(Succeed())
	})

	It("should not error for uninitialized types", func() {
		obj := &jobsv2.ExternalJob{}

		ok, err := conversion.IsConvertible(scheme, obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("should not error for unstructured types", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       "ExternalJob",
				"apiVersion": "jobs.testprojects.kb.io/v2",
			},
		}

		ok, err := conversion.IsConvertible(scheme, obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("should return true for convertible types", func() {
		obj := &jobsv2.ExternalJob{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ExternalJob",
				APIVersion: "jobs.testprojects.kb.io/v2",
			},
		}

		ok, err := conversion.IsConvertible(scheme, obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("should return false for a non convertible type", func() {
		obj := &appsv1beta1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1beta1",
			},
		}

		ok, err := conversion.IsConvertible(scheme, obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).ToNot(BeTrue())
	})
})
