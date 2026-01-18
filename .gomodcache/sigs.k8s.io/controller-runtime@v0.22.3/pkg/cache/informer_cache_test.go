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

package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("informerCache", func() {
	It("should not require LeaderElection", func() {
		cfg := &rest.Config{}

		httpClient, err := rest.HTTPClientFor(cfg)
		Expect(err).ToNot(HaveOccurred())
		mapper, err := apiutil.NewDynamicRESTMapper(cfg, httpClient)
		Expect(err).ToNot(HaveOccurred())

		c, err := cache.New(cfg, cache.Options{Mapper: mapper})
		Expect(err).ToNot(HaveOccurred())

		leaderElectionRunnable, ok := c.(manager.LeaderElectionRunnable)
		Expect(ok).To(BeTrue())
		Expect(leaderElectionRunnable.NeedLeaderElection()).To(BeFalse())
	})
})
