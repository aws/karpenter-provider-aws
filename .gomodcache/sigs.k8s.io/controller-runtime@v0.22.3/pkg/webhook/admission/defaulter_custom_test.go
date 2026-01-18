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

package admission

import (
	"context"
	"maps"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Defaulter Handler", func() {

	It("should remove unknown fields when DefaulterRemoveUnknownFields is passed", func(ctx SpecContext) {
		obj := &TestDefaulter{}
		handler := WithCustomDefaulter(admissionScheme, obj, &TestCustomDefaulter{}, DefaulterRemoveUnknownOrOmitableFields)

		resp := handler.Handle(ctx, Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Object: runtime.RawExtension{
					Raw: []byte(`{"newField":"foo", "totalReplicas":5}`),
				},
			},
		})
		Expect(resp.Allowed).Should(BeTrue())
		Expect(resp.Patches).To(HaveLen(4))
		Expect(resp.Patches).To(ContainElements(
			jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/labels",
				Value:     map[string]any{"foo": "bar"},
			},
			jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/replica",
				Value:     2.0,
			},
			jsonpatch.JsonPatchOperation{
				Operation: "remove",
				Path:      "/newField",
			},
			jsonpatch.JsonPatchOperation{
				Operation: "remove",
				Path:      "/totalReplicas",
			},
		))
		Expect(resp.Result.Code).Should(Equal(int32(http.StatusOK)))
	})

	It("should preserve unknown fields by default", func(ctx SpecContext) {
		obj := &TestDefaulter{}
		handler := WithCustomDefaulter(admissionScheme, obj, &TestCustomDefaulter{})

		resp := handler.Handle(ctx, Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Object: runtime.RawExtension{
					Raw: []byte(`{"newField":"foo", "totalReplicas":5}`),
				},
			},
		})
		Expect(resp.Allowed).Should(BeTrue())
		Expect(resp.Patches).To(HaveLen(3))
		Expect(resp.Patches).To(ContainElements(
			jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/labels",
				Value:     map[string]any{"foo": "bar"},
			},
			jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/replica",
				Value:     2.0,
			},
			jsonpatch.JsonPatchOperation{
				Operation: "remove",
				Path:      "/totalReplicas",
			},
		))
		Expect(resp.Result.Code).Should(Equal(int32(http.StatusOK)))
	})

	It("should return ok if received delete verb in defaulter handler", func(ctx SpecContext) {
		obj := &TestDefaulter{}
		handler := WithCustomDefaulter(admissionScheme, obj, &TestCustomDefaulter{})
		resp := handler.Handle(ctx, Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
				OldObject: runtime.RawExtension{
					Raw: []byte("{}"),
				},
			},
		})
		Expect(resp.Allowed).Should(BeTrue())
		Expect(resp.Result.Code).Should(Equal(int32(http.StatusOK)))
	})
})

// TestDefaulter.
var _ runtime.Object = &TestDefaulter{}

type TestDefaulter struct {
	Labels map[string]string `json:"labels,omitempty"`

	Replica       int `json:"replica,omitempty"`
	TotalReplicas int `json:"totalReplicas,omitempty"`
}

var testDefaulterGVK = schema.GroupVersionKind{Group: "foo.test.org", Version: "v1", Kind: "TestDefaulter"}

func (d *TestDefaulter) GetObjectKind() schema.ObjectKind { return d }
func (d *TestDefaulter) DeepCopyObject() runtime.Object {
	return &TestDefaulter{
		Labels:        maps.Clone(d.Labels),
		Replica:       d.Replica,
		TotalReplicas: d.TotalReplicas,
	}
}

func (d *TestDefaulter) GroupVersionKind() schema.GroupVersionKind {
	return testDefaulterGVK
}

func (d *TestDefaulter) SetGroupVersionKind(gvk schema.GroupVersionKind) {}

var _ runtime.Object = &TestDefaulterList{}

type TestDefaulterList struct{}

func (*TestDefaulterList) GetObjectKind() schema.ObjectKind { return nil }
func (*TestDefaulterList) DeepCopyObject() runtime.Object   { return nil }

// TestCustomDefaulter
type TestCustomDefaulter struct{}

func (d *TestCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	o := obj.(*TestDefaulter)

	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels["foo"] = "bar"

	if o.Replica < 2 {
		o.Replica = 2
	}
	o.TotalReplicas = 0
	return nil
}
