/*
Copyright 2019 The Kubernetes Authors.

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

package webhook_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ = Describe("Webhook Server", func() {
	var (
		ctxCancel          context.CancelFunc
		testHostPort       string
		client             *http.Client
		server             webhook.Server
		servingOpts        envtest.WebhookInstallOptions
		genericStartServer func(f func(ctx context.Context)) (done <-chan struct{})
	)

	BeforeEach(func() {
		var ctx context.Context
		// Has to be derived from context.Background() as it needs to be
		// valid past the BeforeEach
		ctx, ctxCancel = context.WithCancel(context.Background()) //nolint:forbidigo

		servingOpts = envtest.WebhookInstallOptions{}
		Expect(servingOpts.PrepWithoutInstalling()).To(Succeed())

		testHostPort = net.JoinHostPort(servingOpts.LocalServingHost, fmt.Sprintf("%d", servingOpts.LocalServingPort))

		// bypass needing to set up the x509 cert pool, etc ourselves
		clientTransport, err := rest.TransportFor(&rest.Config{
			TLSClientConfig: rest.TLSClientConfig{CAData: servingOpts.LocalServingCAData},
		})
		Expect(err).NotTo(HaveOccurred())
		client = &http.Client{
			Transport: clientTransport,
		}

		server = webhook.NewServer(webhook.Options{
			Host:    servingOpts.LocalServingHost,
			Port:    servingOpts.LocalServingPort,
			CertDir: servingOpts.LocalServingCertDir,
		})

		genericStartServer = func(f func(ctx context.Context)) (done <-chan struct{}) {
			doneCh := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(doneCh)
				f(ctx)
			}()
			// wait till we can ping the server to start the test
			Eventually(func() error {
				_, err := client.Get(fmt.Sprintf("https://%s/unservedpath", testHostPort))
				return err
			}).Should(Succeed())

			return doneCh
		}
	})
	AfterEach(func() {
		Expect(servingOpts.Cleanup()).To(Succeed())
	})

	startServer := func() (done <-chan struct{}) {
		return genericStartServer(func(ctx context.Context) {
			Expect(server.Start(ctx)).To(Succeed())
		})
	}

	// TODO(directxman12): figure out a good way to test all the serving setup
	// with httptest.Server to get all the niceness from that.

	Context("when serving", func() {
		PIt("should verify the client CA name when asked to", func() {

		})
		PIt("should support HTTP/2", func() {

		})

		// TODO(directxman12): figure out a good way to test the port default, etc
	})

	It("should panic if a duplicate path is registered", func() {
		server.Register("/somepath", &testHandler{})
		doneCh := startServer()

		Expect(func() { server.Register("/somepath", &testHandler{}) }).To(Panic())

		ctxCancel()
		Eventually(doneCh, "4s").Should(BeClosed())
	})

	Context("when registering new webhooks before starting", func() {
		It("should serve a webhook on the requested path", func() {
			server.Register("/somepath", &testHandler{})

			Expect(server.StartedChecker()(nil)).ToNot(Succeed())

			doneCh := startServer()

			Eventually(func() ([]byte, error) {
				resp, err := client.Get(fmt.Sprintf("https://%s/somepath", testHostPort))
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				return io.ReadAll(resp.Body)
			}).Should(Equal([]byte("gadzooks!")))

			Expect(server.StartedChecker()(nil)).To(Succeed())

			ctxCancel()
			Eventually(doneCh, "4s").Should(BeClosed())
		})
	})

	Context("when registering webhooks after starting", func() {
		var (
			doneCh <-chan struct{}
		)
		BeforeEach(func() {
			doneCh = startServer()
		})
		AfterEach(func() {
			// wait for cleanup to happen
			ctxCancel()
			Eventually(doneCh, "4s").Should(BeClosed())
		})

		It("should serve a webhook on the requested path", func() {
			server.Register("/somepath", &testHandler{})
			resp, err := client.Get(fmt.Sprintf("https://%s/somepath", testHostPort))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(io.ReadAll(resp.Body)).To(Equal([]byte("gadzooks!")))
		})
	})

	It("should respect passed in TLS configurations", func() {
		var finalCfg *tls.Config
		tlsCfgFunc := func(cfg *tls.Config) {
			cfg.CipherSuites = []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
			}
			cfg.MinVersion = tls.VersionTLS12
			// save cfg after changes to test against
			finalCfg = cfg
		}
		server = webhook.NewServer(webhook.Options{
			Host:    servingOpts.LocalServingHost,
			Port:    servingOpts.LocalServingPort,
			CertDir: servingOpts.LocalServingCertDir,
			TLSOpts: []func(*tls.Config){
				tlsCfgFunc,
			},
		})
		server.Register("/somepath", &testHandler{})
		doneCh := genericStartServer(func(ctx context.Context) {
			Expect(server.Start(ctx)).To(Succeed())
		})

		Eventually(func() ([]byte, error) {
			resp, err := client.Get(fmt.Sprintf("https://%s/somepath", testHostPort))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		}).Should(Equal([]byte("gadzooks!")))
		Expect(finalCfg.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
		Expect(finalCfg.CipherSuites).To(ContainElements(
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
		))

		ctxCancel()
		Eventually(doneCh, "4s").Should(BeClosed())
	})

	It("should prefer GetCertificate through TLSOpts", func() {
		var finalCfg *tls.Config
		finalCert, err := tls.LoadX509KeyPair(
			path.Join(servingOpts.LocalServingCertDir, "tls.crt"),
			path.Join(servingOpts.LocalServingCertDir, "tls.key"),
		)
		Expect(err).NotTo(HaveOccurred())
		finalGetCertificate := func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) { //nolint:unparam
			return &finalCert, nil
		}
		server = &webhook.DefaultServer{Options: webhook.Options{
			Host:    servingOpts.LocalServingHost,
			Port:    servingOpts.LocalServingPort,
			CertDir: servingOpts.LocalServingCertDir,

			TLSOpts: []func(*tls.Config){
				func(cfg *tls.Config) {
					cfg.GetCertificate = finalGetCertificate
					cfg.MinVersion = tls.VersionTLS12
					// save cfg after changes to test against
					finalCfg = cfg
				},
			},
		}}
		server.Register("/somepath", &testHandler{})
		doneCh := genericStartServer(func(ctx context.Context) {
			Expect(server.Start(ctx)).To(Succeed())
		})

		Eventually(func() ([]byte, error) {
			resp, err := client.Get(fmt.Sprintf("https://%s/somepath", testHostPort))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		}).Should(Equal([]byte("gadzooks!")))
		Expect(finalCfg.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
		// We can't compare the functions directly, but we can compare their pointers
		if reflect.ValueOf(finalCfg.GetCertificate).Pointer() != reflect.ValueOf(finalGetCertificate).Pointer() {
			Fail("GetCertificate was not set properly, or overwritten")
		}
		cert, err := finalCfg.GetCertificate(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(cert).To(BeEquivalentTo(&finalCert))

		ctxCancel()
		Eventually(doneCh, "4s").Should(BeClosed())
	})
})

type testHandler struct {
}

func (t *testHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if _, err := resp.Write([]byte("gadzooks!")); err != nil {
		panic("unable to write http response!")
	}
}
