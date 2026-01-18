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

package recorder_test

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/internal/recorder"
)

var _ = Describe("recorder.Provider", func() {
	makeBroadcaster := func() (record.EventBroadcaster, bool) { return record.NewBroadcaster(), true }
	Describe("NewProvider", func() {
		It("should return a provider instance and a nil error.", func() {
			provider, err := recorder.NewProvider(cfg, httpClient, scheme.Scheme, logr.Discard(), makeBroadcaster)
			Expect(provider).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if failed to init client.", func() {
			// Invalid the config
			cfg1 := *cfg
			cfg1.Host = "invalid host"
			_, err := recorder.NewProvider(&cfg1, httpClient, scheme.Scheme, logr.Discard(), makeBroadcaster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to init client"))
		})
	})
	Describe("GetEventRecorder", func() {
		It("should return a recorder instance.", func() {
			provider, err := recorder.NewProvider(cfg, httpClient, scheme.Scheme, logr.Discard(), makeBroadcaster)
			Expect(err).NotTo(HaveOccurred())

			recorder := provider.GetEventRecorderFor("test")
			Expect(recorder).NotTo(BeNil())
		})
	})
})
