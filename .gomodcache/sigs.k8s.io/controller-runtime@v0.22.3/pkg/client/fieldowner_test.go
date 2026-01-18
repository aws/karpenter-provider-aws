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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestWithFieldOwner(t *testing.T) {
	calls := 0
	fakeClient := testClient(t, "custom-field-mgr", func() { calls++ })
	wrappedClient := client.WithFieldOwner(fakeClient, "custom-field-mgr")

	ctx := t.Context()
	dummyObj := &corev1.Namespace{}

	_ = wrappedClient.Create(ctx, dummyObj)
	_ = wrappedClient.Update(ctx, dummyObj)
	_ = wrappedClient.Patch(ctx, dummyObj, nil)
	_ = wrappedClient.Status().Create(ctx, dummyObj, dummyObj)
	_ = wrappedClient.Status().Update(ctx, dummyObj)
	_ = wrappedClient.Status().Patch(ctx, dummyObj, nil)
	_ = wrappedClient.SubResource("some-subresource").Create(ctx, dummyObj, dummyObj)
	_ = wrappedClient.SubResource("some-subresource").Update(ctx, dummyObj)
	_ = wrappedClient.SubResource("some-subresource").Patch(ctx, dummyObj, nil)

	if expectedCalls := 9; calls != expectedCalls {
		t.Fatalf("wrong number of calls to assertions: expected=%d; got=%d", expectedCalls, calls)
	}
}

func TestWithFieldOwnerOverridden(t *testing.T) {
	calls := 0

	fakeClient := testClient(t, "new-field-manager", func() { calls++ })
	wrappedClient := client.WithFieldOwner(fakeClient, "old-field-manager")

	ctx := t.Context()
	dummyObj := &corev1.Namespace{}

	_ = wrappedClient.Create(ctx, dummyObj, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.Update(ctx, dummyObj, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.Patch(ctx, dummyObj, nil, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.Status().Create(ctx, dummyObj, dummyObj, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.Status().Update(ctx, dummyObj, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.Status().Patch(ctx, dummyObj, nil, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.SubResource("some-subresource").Create(ctx, dummyObj, dummyObj, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.SubResource("some-subresource").Update(ctx, dummyObj, client.FieldOwner("new-field-manager"))
	_ = wrappedClient.SubResource("some-subresource").Patch(ctx, dummyObj, nil, client.FieldOwner("new-field-manager"))

	if expectedCalls := 9; calls != expectedCalls {
		t.Fatalf("wrong number of calls to assertions: expected=%d; got=%d", expectedCalls, calls)
	}
}

// testClient is a helper function that checks if calls have the expected field manager,
// and calls the callback function on each intercepted call.
func testClient(t *testing.T, expectedFieldManager string, callback func()) client.Client {
	// TODO: we could use the dummyClient in interceptor pkg if we move it to an internal pkg
	return fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			callback()
			out := &client.CreateOptions{}
			for _, f := range opts {
				f.ApplyToCreate(out)
			}
			if got := out.FieldManager; expectedFieldManager != got {
				t.Fatalf("wrong field manager: expected=%q; got=%q", expectedFieldManager, got)
			}
			return nil
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			callback()
			out := &client.UpdateOptions{}
			for _, f := range opts {
				f.ApplyToUpdate(out)
			}
			if got := out.FieldManager; expectedFieldManager != got {
				t.Fatalf("wrong field manager: expected=%q; got=%q", expectedFieldManager, got)
			}
			return nil
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			callback()
			out := &client.PatchOptions{}
			for _, f := range opts {
				f.ApplyToPatch(out)
			}
			if got := out.FieldManager; expectedFieldManager != got {
				t.Fatalf("wrong field manager: expected=%q; got=%q", expectedFieldManager, got)
			}
			return nil
		},
		SubResourceCreate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
			callback()
			out := &client.SubResourceCreateOptions{}
			for _, f := range opts {
				f.ApplyToSubResourceCreate(out)
			}
			if got := out.FieldManager; expectedFieldManager != got {
				t.Fatalf("wrong field manager: expected=%q; got=%q", expectedFieldManager, got)
			}
			return nil
		},
		SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			callback()
			out := &client.SubResourceUpdateOptions{}
			for _, f := range opts {
				f.ApplyToSubResourceUpdate(out)
			}
			if got := out.FieldManager; expectedFieldManager != got {
				t.Fatalf("wrong field manager: expected=%q; got=%q", expectedFieldManager, got)
			}
			return nil
		},
		SubResourcePatch: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			callback()
			out := &client.SubResourcePatchOptions{}
			for _, f := range opts {
				f.ApplyToSubResourcePatch(out)
			}
			if got := out.FieldManager; expectedFieldManager != got {
				t.Fatalf("wrong field manager: expected=%q; got=%q", expectedFieldManager, got)
			}
			return nil
		},
	}).Build()
}
