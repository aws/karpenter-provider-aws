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

package scheme_test

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var _ = Describe("Scheme", func() {
	Describe("Builder", func() {
		It("should provide a Scheme with the types registered", func() {
			gv := schema.GroupVersion{Group: "core", Version: "v1"}

			s, err := (&scheme.Builder{GroupVersion: gv}).
				Register(&corev1.Pod{}, &corev1.PodList{}).
				Build()
			Expect(err).NotTo(HaveOccurred())

			internalGv := schema.GroupVersion{Group: "core", Version: "__internal"}
			emptyGv := schema.GroupVersion{Group: "", Version: "v1"}
			Expect(s.AllKnownTypes()).To(MatchAllKeys(Keys{
				gv.WithKind("Pod"):     Equal(reflect.TypeOf(corev1.Pod{})),
				gv.WithKind("PodList"): Equal(reflect.TypeOf(corev1.PodList{})),

				// Base types
				gv.WithKind("CreateOptions"): Ignore(),
				gv.WithKind("UpdateOptions"): Ignore(),
				gv.WithKind("PatchOptions"):  Ignore(),
				gv.WithKind("DeleteOptions"): Ignore(),
				gv.WithKind("GetOptions"):    Ignore(),
				gv.WithKind("ListOptions"):   Ignore(),
				gv.WithKind("WatchEvent"):    Ignore(),

				internalGv.WithKind("WatchEvent"): Ignore(),

				emptyGv.WithKind("APIGroup"):        Ignore(),
				emptyGv.WithKind("APIGroupList"):    Ignore(),
				emptyGv.WithKind("APIResourceList"): Ignore(),
				emptyGv.WithKind("APIVersions"):     Ignore(),
				emptyGv.WithKind("Status"):          Ignore(),
			}))
		})

		It("should be able to add types from other Builders", func() {
			gv1 := schema.GroupVersion{Group: "core", Version: "v1"}
			b1 := (&scheme.Builder{GroupVersion: gv1}).Register(&corev1.Pod{}, &corev1.PodList{})

			gv2 := schema.GroupVersion{Group: "apps", Version: "v1"}
			s, err := (&scheme.Builder{GroupVersion: gv2}).
				Register(&appsv1.Deployment{}).
				Register(&appsv1.DeploymentList{}).
				RegisterAll(b1).
				Build()

			Expect(err).NotTo(HaveOccurred())
			internalGv1 := schema.GroupVersion{Group: "core", Version: "__internal"}
			internalGv2 := schema.GroupVersion{Group: "apps", Version: "__internal"}
			emptyGv := schema.GroupVersion{Group: "", Version: "v1"}
			Expect(s.AllKnownTypes()).To(MatchAllKeys(Keys{
				// Types from b1
				gv1.WithKind("Pod"):     Equal(reflect.TypeOf(corev1.Pod{})),
				gv1.WithKind("PodList"): Equal(reflect.TypeOf(corev1.PodList{})),

				// Types from b2
				gv2.WithKind("Deployment"):     Equal(reflect.TypeOf(appsv1.Deployment{})),
				gv2.WithKind("DeploymentList"): Equal(reflect.TypeOf(appsv1.DeploymentList{})),

				// Base types
				gv1.WithKind("CreateOptions"): Ignore(),
				gv1.WithKind("UpdateOptions"): Ignore(),
				gv1.WithKind("PatchOptions"):  Ignore(),
				gv1.WithKind("DeleteOptions"): Ignore(),
				gv1.WithKind("GetOptions"):    Ignore(),
				gv1.WithKind("ListOptions"):   Ignore(),
				gv1.WithKind("WatchEvent"):    Ignore(),

				internalGv1.WithKind("WatchEvent"): Ignore(),

				gv2.WithKind("CreateOptions"): Ignore(),
				gv2.WithKind("UpdateOptions"): Ignore(),
				gv2.WithKind("PatchOptions"):  Ignore(),
				gv2.WithKind("DeleteOptions"): Ignore(),
				gv2.WithKind("GetOptions"):    Ignore(),
				gv2.WithKind("ListOptions"):   Ignore(),
				gv2.WithKind("WatchEvent"):    Ignore(),

				internalGv2.WithKind("WatchEvent"): Ignore(),

				emptyGv.WithKind("APIGroup"):        Ignore(),
				emptyGv.WithKind("APIGroupList"):    Ignore(),
				emptyGv.WithKind("APIResourceList"): Ignore(),
				emptyGv.WithKind("APIVersions"):     Ignore(),
				emptyGv.WithKind("Status"):          Ignore(),
			}))
		})
	})
})
