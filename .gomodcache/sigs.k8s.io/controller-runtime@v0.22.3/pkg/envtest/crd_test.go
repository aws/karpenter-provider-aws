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

package envtest

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = Describe("Test", func() {
	Describe("readCRDFiles", func() {
		It("should not mix up files from different directories", func() {
			opt := CRDInstallOptions{
				Paths: []string{
					"testdata/crds",
					"testdata/crdv1_original",
				},
			}
			err := ReadCRDFiles(&opt)
			Expect(err).NotTo(HaveOccurred())

			expectedCRDs := sets.NewString(
				"frigates.ship.example.com",
				"configs.foo.example.com",
				"drivers.crew.example.com",
			)

			foundCRDs := sets.NewString()
			for _, crd := range opt.CRDs {
				foundCRDs.Insert(crd.Name)
			}

			Expect(expectedCRDs).To(Equal(foundCRDs))
		})
	})
})
