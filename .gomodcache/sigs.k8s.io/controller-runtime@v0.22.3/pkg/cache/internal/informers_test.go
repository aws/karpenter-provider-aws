/*
Copyright 2022 The Kubernetes Authors.

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

package internal

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// Test that gvkFixupWatcher behaves like watch.FakeWatcher
// and that it overrides the GVK.
// These tests are adapted from the watch.FakeWatcher tests in:
// https://github.com/kubernetes/kubernetes/blob/adbda068c1808fcc8a64a94269e0766b5c46ec41/staging/src/k8s.io/apimachinery/pkg/watch/watch_test.go#L33-L78
var _ = Describe("gvkFixupWatcher", func() {
	It("behaves like watch.FakeWatcher", func() {
		newTestType := func(name string) runtime.Object {
			return &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}
		}

		f := watch.NewFake()
		// This is the GVK which we expect the wrapper to set on all the events
		expectedGVK := schema.GroupVersionKind{
			Group:   "testgroup",
			Version: "v1test2",
			Kind:    "TestKind",
		}
		gvkfw := newGVKFixupWatcher(expectedGVK, f)

		table := []struct {
			t watch.EventType
			s runtime.Object
		}{
			{watch.Added, newTestType("foo")},
			{watch.Modified, newTestType("qux")},
			{watch.Modified, newTestType("bar")},
			{watch.Deleted, newTestType("bar")},
			{watch.Error, newTestType("error: blah")},
		}

		consumer := func(w watch.Interface) {
			for _, expect := range table {
				By(fmt.Sprintf("Fixing up watch.EventType: %v and passing it on", expect.t))
				got, ok := <-w.ResultChan()
				Expect(ok).To(BeTrue(), "closed early")
				Expect(expect.t).To(Equal(got.Type), "unexpected Event.Type or out-of-order Event")
				Expect(got.Object).To(BeAssignableToTypeOf(&metav1.PartialObjectMetadata{}), "unexpected Event.Object type")
				a := got.Object.(*metav1.PartialObjectMetadata)
				Expect(got.Object.GetObjectKind().GroupVersionKind()).To(Equal(expectedGVK), "GVK was not fixed up")
				expected := expect.s.DeepCopyObject()
				expected.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{})
				actual := a.DeepCopyObject()
				actual.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{})
				Expect(actual).To(Equal(expected), "unexpected change to the Object")
			}
			Eventually(w.ResultChan()).Should(BeClosed())
		}

		sender := func() {
			f.Add(newTestType("foo"))
			f.Action(watch.Modified, newTestType("qux"))
			f.Modify(newTestType("bar"))
			f.Delete(newTestType("bar"))
			f.Error(newTestType("error: blah"))
			f.Stop()
		}

		go sender()
		consumer(gvkfw)
	})
})
