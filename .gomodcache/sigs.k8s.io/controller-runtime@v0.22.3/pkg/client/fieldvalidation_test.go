/*
Copyright 2024 The Kubernetes Authors.

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

package client_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1applyconfigurations "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("ClientWithFieldValidation", func() {
	It("should return errors for invalid fields when using strict validation", func(ctx SpecContext) {
		cl, err := client.New(cfg, client.Options{})
		Expect(err).NotTo(HaveOccurred())
		Expect(cl).NotTo(BeNil())

		wrappedClient := client.WithFieldValidation(cl, metav1.FieldValidationStrict)

		baseNode := &unstructured.Unstructured{}
		baseNode.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Kind:    "Node",
			Version: "v1",
		})
		baseNode.SetName("client-with-field-validation-test-node")

		validNode := baseNode.DeepCopy()
		patch := client.MergeFrom(validNode.DeepCopy())

		invalidNode := baseNode.DeepCopy()
		Expect(unstructured.SetNestedField(invalidNode.Object, "value", "spec", "invalidField")).To(Succeed())

		invalidStatusNode := baseNode.DeepCopy()
		Expect(unstructured.SetNestedField(invalidStatusNode.Object, "value", "status", "invalidStatusField")).To(Succeed())

		err = wrappedClient.Create(ctx, invalidNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("strict decoding error: unknown field \"spec.invalidField\""))

		err = wrappedClient.Create(ctx, validNode)
		Expect(err).ToNot(HaveOccurred())

		err = wrappedClient.Update(ctx, invalidNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("strict decoding error: unknown field \"spec.invalidField\""))

		err = wrappedClient.Patch(ctx, invalidNode, patch)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("strict decoding error: unknown field \"spec.invalidField\""))

		// Status.Create is not supported on Nodes

		err = wrappedClient.Status().Update(ctx, invalidStatusNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("strict decoding error: unknown field \"status.invalidStatusField\""))

		err = wrappedClient.Status().Patch(ctx, invalidStatusNode, patch)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("strict decoding error: unknown field \"status.invalidStatusField\""))

		// Status.Create is not supported on Nodes

		err = wrappedClient.SubResource("status").Update(ctx, invalidStatusNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("strict decoding error: unknown field \"status.invalidStatusField\""))

		err = wrappedClient.SubResource("status").Patch(ctx, invalidStatusNode, patch)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("strict decoding error: unknown field \"status.invalidStatusField\""))
	})
})

func TestWithStrictFieldValidation(t *testing.T) {
	calls := 0
	fakeClient := testFieldValidationClient(t, metav1.FieldValidationStrict, func() { calls++ })
	wrappedClient := client.WithFieldValidation(fakeClient, metav1.FieldValidationStrict)

	ctx := t.Context()
	dummyObj := &corev1.Namespace{}

	_ = wrappedClient.Create(ctx, dummyObj)
	_ = wrappedClient.Update(ctx, dummyObj)
	_ = wrappedClient.Apply(ctx, corev1applyconfigurations.ConfigMap("foo", "bar"))
	_ = wrappedClient.Patch(ctx, dummyObj, nil)
	_ = wrappedClient.Status().Create(ctx, dummyObj, dummyObj)
	_ = wrappedClient.Status().Update(ctx, dummyObj)
	_ = wrappedClient.Status().Patch(ctx, dummyObj, nil)
	_ = wrappedClient.SubResource("some-subresource").Create(ctx, dummyObj, dummyObj)
	_ = wrappedClient.SubResource("some-subresource").Update(ctx, dummyObj)
	_ = wrappedClient.SubResource("some-subresource").Patch(ctx, dummyObj, nil)

	if expectedCalls := 10; calls != expectedCalls {
		t.Fatalf("wrong number of calls to assertions: expected=%d; got=%d", expectedCalls, calls)
	}
}

func TestWithStrictFieldValidationOverridden(t *testing.T) {
	calls := 0

	fakeClient := testFieldValidationClient(t, metav1.FieldValidationWarn, func() { calls++ })
	wrappedClient := client.WithFieldValidation(fakeClient, metav1.FieldValidationStrict)

	ctx := t.Context()
	dummyObj := &corev1.Namespace{}

	_ = wrappedClient.Create(ctx, dummyObj, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.Update(ctx, dummyObj, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.Patch(ctx, dummyObj, nil, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.Status().Create(ctx, dummyObj, dummyObj, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.Status().Update(ctx, dummyObj, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.Status().Patch(ctx, dummyObj, nil, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.SubResource("some-subresource").Create(ctx, dummyObj, dummyObj, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.SubResource("some-subresource").Update(ctx, dummyObj, client.FieldValidation(metav1.FieldValidationWarn))
	_ = wrappedClient.SubResource("some-subresource").Patch(ctx, dummyObj, nil, client.FieldValidation(metav1.FieldValidationWarn))

	if expectedCalls := 9; calls != expectedCalls {
		t.Fatalf("wrong number of calls to assertions: expected=%d; got=%d", expectedCalls, calls)
	}
}

// testFieldValidationClient is a helper function that checks if calls have the expected field validation,
// and calls the callback function on each intercepted call.
func testFieldValidationClient(t *testing.T, expectedFieldValidation string, callback func()) client.Client {
	// TODO: we could use the dummyClient in interceptor pkg if we move it to an internal pkg
	return fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			callback()
			out := &client.CreateOptions{}
			for _, f := range opts {
				f.ApplyToCreate(out)
			}
			if got := out.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			if got := out.AsCreateOptions().FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			co := &client.CreateOptions{}
			out.ApplyToCreate(co)
			if got := co.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}
			return nil
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			callback()
			out := &client.UpdateOptions{}
			for _, f := range opts {
				f.ApplyToUpdate(out)
			}
			if got := out.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			if got := out.AsUpdateOptions().FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			co := &client.UpdateOptions{}
			out.ApplyToUpdate(co)
			if got := co.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}
			return nil
		},
		Apply: func(ctx context.Context, client client.WithWatch, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
			callback()
			return nil
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			callback()
			out := &client.PatchOptions{}
			for _, f := range opts {
				f.ApplyToPatch(out)
			}
			if got := out.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			if got := out.AsPatchOptions().FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			co := &client.PatchOptions{}
			out.ApplyToPatch(co)
			if got := co.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}
			return nil
		},
		SubResourceCreate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
			callback()
			out := &client.SubResourceCreateOptions{}
			for _, f := range opts {
				f.ApplyToSubResourceCreate(out)
			}
			if got := out.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			if got := out.AsCreateOptions().FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			co := &client.CreateOptions{}
			out.ApplyToCreate(co)
			if got := co.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}
			return nil
		},
		SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			callback()
			out := &client.SubResourceUpdateOptions{}
			for _, f := range opts {
				f.ApplyToSubResourceUpdate(out)
			}
			if got := out.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			if got := out.AsUpdateOptions().FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			co := &client.UpdateOptions{}
			out.ApplyToUpdate(co)
			if got := co.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}
			return nil
		},
		SubResourcePatch: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			callback()
			out := &client.SubResourcePatchOptions{}
			for _, f := range opts {
				f.ApplyToSubResourcePatch(out)
			}
			if got := out.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			if got := out.AsPatchOptions().FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}

			co := &client.PatchOptions{}
			out.ApplyToPatch(co)
			if got := co.FieldValidation; expectedFieldValidation != got {
				t.Fatalf("wrong field validation: expected=%q; got=%q", expectedFieldValidation, got)
			}
			return nil
		},
	}).Build()
}
