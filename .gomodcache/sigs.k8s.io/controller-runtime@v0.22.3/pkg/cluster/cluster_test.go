/*
Copyright 2020 The Kubernetes Authors.

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

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	intrec "sigs.k8s.io/controller-runtime/pkg/internal/recorder"
)

var _ = Describe("cluster.Cluster", func() {
	Describe("New", func() {
		It("should return an error if there is no Config", func() {
			c, err := New(nil)
			Expect(c).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("must specify Config"))

		})

		It("should return an error if it can't create a RestMapper", func() {
			expected := fmt.Errorf("expected error: RestMapper")
			c, err := New(cfg, func(o *Options) {
				o.MapperProvider = func(c *rest.Config, httpClient *http.Client) (meta.RESTMapper, error) { return nil, expected }
			})
			Expect(c).To(BeNil())
			Expect(err).To(Equal(expected))

		})

		It("should return an error it can't create a client.Client", func() {
			c, err := New(cfg, func(o *Options) {
				o.NewClient = func(config *rest.Config, options client.Options) (client.Client, error) {
					return nil, errors.New("expected error")
				}
			})
			Expect(c).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected error"))
		})

		It("should return an error it can't create a cache.Cache", func() {
			c, err := New(cfg, func(o *Options) {
				o.NewCache = func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
					return nil, fmt.Errorf("expected error")
				}
			})
			Expect(c).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected error"))
		})

		It("should create a client defined in by the new client function", func() {
			c, err := New(cfg, func(o *Options) {
				o.NewClient = func(config *rest.Config, options client.Options) (client.Client, error) {
					return nil, nil
				}
			})
			Expect(c).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
			Expect(c.GetClient()).To(BeNil())
		})

		It("should return an error it can't create a recorder.Provider", func() {
			c, err := New(cfg, func(o *Options) {
				o.newRecorderProvider = func(_ *rest.Config, _ *http.Client, _ *runtime.Scheme, _ logr.Logger, _ intrec.EventBroadcasterProducer) (*intrec.Provider, error) {
					return nil, fmt.Errorf("expected error")
				}
			})
			Expect(c).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected error"))
		})

	})

	Describe("Start", func() {
		It("should stop when context is cancelled", func(specCtx SpecContext) {
			c, err := New(cfg)
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithCancel(specCtx)
			cancel()
			Expect(c.Start(ctx)).NotTo(HaveOccurred())
		})
	})

	It("should not leak goroutines when stopped", func(specCtx SpecContext) {
		currentGRs := goleak.IgnoreCurrent()

		c, err := New(cfg)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(specCtx)
		cancel()
		Expect(c.Start(ctx)).NotTo(HaveOccurred())

		// force-close keep-alive connections.  These'll time anyway (after
		// like 30s or so) but force it to speed up the tests.
		clientTransport.CloseIdleConnections()
		Eventually(func() error { return goleak.Find(currentGRs) }).Should(Succeed())
	})

	It("should provide a function to get the Config", func() {
		c, err := New(cfg)
		Expect(err).NotTo(HaveOccurred())
		cluster, ok := c.(*cluster)
		Expect(ok).To(BeTrue())
		Expect(c.GetConfig()).To(Equal(cluster.config))
	})

	It("should provide a function to get the Client", func() {
		c, err := New(cfg)
		Expect(err).NotTo(HaveOccurred())
		cluster, ok := c.(*cluster)
		Expect(ok).To(BeTrue())
		Expect(c.GetClient()).To(Equal(cluster.client))
	})

	It("should provide a function to get the Scheme", func() {
		c, err := New(cfg)
		Expect(err).NotTo(HaveOccurred())
		cluster, ok := c.(*cluster)
		Expect(ok).To(BeTrue())
		Expect(c.GetScheme()).To(Equal(cluster.scheme))
	})

	It("should provide a function to get the FieldIndexer", func() {
		c, err := New(cfg)
		Expect(err).NotTo(HaveOccurred())
		cluster, ok := c.(*cluster)
		Expect(ok).To(BeTrue())
		Expect(c.GetFieldIndexer()).To(Equal(cluster.cache))
	})

	It("should provide a function to get the EventRecorder", func() {
		c, err := New(cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.GetEventRecorderFor("test")).NotTo(BeNil())
	})
	It("should provide a function to get the APIReader", func() {
		c, err := New(cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.GetAPIReader()).NotTo(BeNil())
	})
})
