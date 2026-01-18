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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Admission Webhook Decoder", func() {
	var decoder Decoder
	BeforeEach(func() {
		By("creating a new decoder for a scheme")
		decoder = NewDecoder(scheme.Scheme)
		Expect(decoder).NotTo(BeNil())
	})

	req := Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "foo",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
                "image": "bar:v2",
                "name": "bar"
            }
        ]
    }
}`),
			},
			OldObject: runtime.RawExtension{
				Raw: []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "foo",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
                "image": "bar:v1",
                "name": "bar"
            }
        ]
    }
}`),
			},
		},
	}

	It("should decode a valid admission request", func() {
		By("extracting the object from the request")
		var actualObj corev1.Pod
		Expect(decoder.Decode(req, &actualObj)).To(Succeed())

		By("verifying that all data is present in the object")
		Expect(actualObj).To(Equal(corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "bar:v2", Name: "bar"},
				},
			},
		}))
	})

	It("should decode a valid RawExtension object", func() {
		By("decoding the RawExtension object")
		var actualObj corev1.Pod
		Expect(decoder.DecodeRaw(req.OldObject, &actualObj)).To(Succeed())

		By("verifying that all data is present in the object")
		Expect(actualObj).To(Equal(corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "bar:v1", Name: "bar"},
				},
			},
		}))
	})

	// NOTE: This will only pass if a GVK is provided. An unstructered object without a GVK may succeed
	// in decoding to an alternate type.
	It("should fail to decode if the object in the request doesn't match the passed-in type", func() {
		By("trying to extract a pod from the quest into a node")
		Expect(decoder.Decode(req, &corev1.Node{})).NotTo(Succeed())

		By("trying to extract a pod in RawExtension format into a node")
		Expect(decoder.DecodeRaw(req.OldObject, &corev1.Node{})).NotTo(Succeed())
	})

	It("should be able to decode into an unstructured object", func() {
		By("decoding the request into an unstructured object")
		var target unstructured.Unstructured
		Expect(decoder.Decode(req, &target)).To(Succeed())

		By("sanity-checking the metadata on the output object")
		Expect(target.Object["metadata"]).To(Equal(map[string]interface{}{
			"name":      "foo",
			"namespace": "default",
		}))

		By("decoding the RawExtension object into an unstructured object")
		var target2 unstructured.Unstructured
		Expect(decoder.DecodeRaw(req.Object, &target2)).To(Succeed())

		By("sanity-checking the metadata on the output object")
		Expect(target2.Object["metadata"]).To(Equal(map[string]interface{}{
			"name":      "foo",
			"namespace": "default",
		}))
	})

	req2 := Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: "CREATE",
			Object: runtime.RawExtension{
				Raw: []byte(`{
    "metadata": {
        "name": "foo",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
                "image": "bar:v2",
                "name": "bar"
            }
        ]
    }
	}`),
			},
			OldObject: runtime.RawExtension{
				Object: nil,
			},
		},
	}

	It("should decode a valid admission request without GVK", func() {
		By("extracting the object from the request")
		var target3 unstructured.Unstructured
		Expect(decoder.DecodeRaw(req2.Object, &target3)).To(Succeed())
	})
})
