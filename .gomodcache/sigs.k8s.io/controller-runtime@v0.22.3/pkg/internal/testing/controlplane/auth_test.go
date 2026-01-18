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

package controlplane_test

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	kcert "k8s.io/client-go/util/cert"

	cp "sigs.k8s.io/controller-runtime/pkg/internal/testing/controlplane"
	"sigs.k8s.io/controller-runtime/pkg/internal/testing/process"
)

var _ = Describe("Cert Authentication", func() {
	var authn *cp.CertAuthn
	BeforeEach(func() {
		var err error
		authn, err = cp.NewCertAuthn()
		Expect(err).NotTo(HaveOccurred(), "should be able to create the cert authn")
	})
	Context("when starting", func() {
		It("should write the verifying CA to the configured directory", func() {
			By("setting up a temp dir")
			dir, err := os.MkdirTemp("", "envtest_controlplane_*")
			Expect(err).NotTo(HaveOccurred(), "should be able to provision a temp dir")
			if dir != "" {
				defer os.RemoveAll(dir)
			}

			By("configuring to use that dir")
			Expect(authn.Configure(dir, process.EmptyArguments())).To(Succeed())

			By("starting and checking the dir")
			Expect(authn.Start()).To(Succeed())
			defer func() { Expect(authn.Stop()).To(Succeed()) }() // not strictly necessary, but future-proof

			_, err = os.Stat(filepath.Join(dir, "client-cert-auth-ca.crt"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error out if we haven't been configured yet", func() {
			// NB(directxman12): no configure here intentionally
			Expect(authn.Start()).NotTo(Succeed())
		})
	})
	Context("when configuring", func() {
		It("should have set up the API server to use the written file for client cert auth", func() {
			args := process.EmptyArguments()
			Expect(authn.Configure("/tmp/____doesnotexist", args)).To(Succeed())
			Expect(args.Get("client-ca-file").Get(nil)).To(ConsistOf("/tmp/____doesnotexist/client-cert-auth-ca.crt"))
		})
	})

	Describe("creating users", func() {
		user := cp.User{Name: "someuser", Groups: []string{"group1", "group2"}}

		Context("before starting", func() {
			It("should yield a REST config that contains certs valid for the to-be-written CA", func() {
				cfg, err := authn.AddUser(user, &rest.Config{})
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())

				Expect(cfg.CertData).NotTo(BeEmpty())
				Expect(cfg.KeyData).NotTo(BeEmpty())

				// double-check the cert (assume the key is fine if it's present
				// and the cert is also present, cause it's more annoying to verify
				// and we have separate tinyca & integration tests.
				By("parsing the config's cert & key data")
				certs, err := tls.X509KeyPair(cfg.CertData, cfg.KeyData)
				Expect(err).NotTo(HaveOccurred(), "config cert/key data should be valid key pair")
				cert, err := x509.ParseCertificate(certs.Certificate[0]) // re-parse cause .Leaf isn't saved
				Expect(err).NotTo(HaveOccurred())

				By("starting and loading the CA cert")
				dir, err := os.MkdirTemp("", "envtest_controlplane_*")
				Expect(err).NotTo(HaveOccurred(), "should be able to provision a temp dir")
				if dir != "" {
					defer os.RemoveAll(dir)
				}
				Expect(authn.Configure(dir, process.EmptyArguments())).To(Succeed())
				Expect(authn.Start()).To(Succeed())
				caCerts, err := kcert.CertsFromFile(filepath.Join(dir, "client-cert-auth-ca.crt"))
				Expect(err).NotTo(HaveOccurred(), "should be able to read the CA cert file))))")
				Expect(cert.CheckSignatureFrom(caCerts[0])).To(Succeed(), "the config's cert should be signed by the written CA")
			})

			It("should copy the configuration from the base CA without modifying it", func() {
				By("creating a user and checking the output config")
				base := &rest.Config{Burst: 30}
				cfg, err := authn.AddUser(user, base)
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())
				Expect(cfg.Burst).To(Equal(30))

				By("mutating the base and verifying the cfg doesn't change")
				base.Burst = 8675
				Expect(cfg.Burst).To(Equal(30))
			})
		})

		Context("after starting", func() {
			var dir string
			BeforeEach(func() {
				By("setting up a temp dir & starting with it")
				var err error
				dir, err = os.MkdirTemp("", "envtest_controlplane_*")
				Expect(err).NotTo(HaveOccurred(), "should be able to provision a temp dir")
				Expect(authn.Configure(dir, process.EmptyArguments())).To(Succeed())
				Expect(authn.Start()).To(Succeed())
			})
			AfterEach(func() {
				if dir != "" {
					defer os.RemoveAll(dir)
				}
			})

			It("should yield a REST config that contains certs valid for the written CA", func() {
				cfg, err := authn.AddUser(user, &rest.Config{})
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())

				Expect(cfg.CertData).NotTo(BeEmpty())
				Expect(cfg.KeyData).NotTo(BeEmpty())

				// double-check the cert (assume the key is fine if it's present
				// and the cert is also present, cause it's more annoying to verify
				// and we have separate tinyca & integration tests.
				By("parsing the config's cert & key data")
				certs, err := tls.X509KeyPair(cfg.CertData, cfg.KeyData)
				Expect(err).NotTo(HaveOccurred(), "config cert/key data should be valid key pair")
				cert, err := x509.ParseCertificate(certs.Certificate[0]) // re-parse cause .Leaf isn't saved
				Expect(err).NotTo(HaveOccurred())

				By("loading the CA cert")
				caCerts, err := kcert.CertsFromFile(filepath.Join(dir, "client-cert-auth-ca.crt"))
				Expect(err).NotTo(HaveOccurred(), "should be able to read the CA cert file))))")
				Expect(cert.CheckSignatureFrom(caCerts[0])).To(Succeed(), "the config's cert should be signed by the written CA")
			})

			It("should copy the configuration from the base CA without modifying it", func() {
				By("creating a user and checking the output config")
				base := &rest.Config{Burst: 30}
				cfg, err := authn.AddUser(user, base)
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())
				Expect(cfg.Burst).To(Equal(30))

				By("mutating the base and verifying the cfg doesn't change")
				base.Burst = 8675
				Expect(cfg.Burst).To(Equal(30))
			})
		})
	})
})
