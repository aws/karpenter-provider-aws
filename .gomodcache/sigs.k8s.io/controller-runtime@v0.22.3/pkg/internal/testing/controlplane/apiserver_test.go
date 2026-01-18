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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/url"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"

	. "sigs.k8s.io/controller-runtime/pkg/internal/testing/controlplane"
	"sigs.k8s.io/controller-runtime/pkg/internal/testing/process"
)

var _ = Describe("APIServer", func() {
	var server *APIServer
	BeforeEach(func() {
		server = &APIServer{
			EtcdURL: &url.URL{},
		}
	})
	JustBeforeEach(func() {
		Expect(PrepareAPIServer(server)).To(Succeed())
	})
	Describe("setting up serving hosts & ports", func() {
		Context("when URL is set", func() {
			BeforeEach(func() {
				server.URL = &url.URL{Scheme: "http", Host: "localhost:8675", Path: "/some-path"}
			})

			Context("when insecure serving is also set", func() {
				BeforeEach(func() {
					server.InsecureServing = &process.ListenAddr{
						Address: "localhost",
						Port:    "1234",
					}
				})

				It("should override the existing insecure serving", func() {
					Expect(server.InsecureServing).To(Equal(&process.ListenAddr{
						Address: "localhost",
						Port:    "8675",
					}))
				})
			})

			It("should set insecure serving off of that", func() {
				Expect(server.InsecureServing).To(Equal(&process.ListenAddr{
					Address: "localhost",
					Port:    "8675",
				}))
			})

			It("should keep URL as-is", func() {
				Expect(server.URL.String()).To(Equal("http://localhost:8675/some-path"))
			})
		})

		Context("when URL is not set but InsecureServing is set", func() {
			BeforeEach(func() {
				server.InsecureServing = &process.ListenAddr{}
			})

			Context("when host and port are set", func() {
				BeforeEach(func() {
					server.InsecureServing.Address = "localhost"
					server.InsecureServing.Port = "8675"
				})
				It("should set URL from InsecureServing", func() {
					Expect(server.URL.String()).To(Equal("http://localhost:8675"))
				})

				It("should leave InsecureServing as-is if address and port are filled out", func() {
					Expect(server.InsecureServing).To(Equal(&process.ListenAddr{
						Address: "localhost",
						Port:    "8675",
					}))
				})
			})

			Context("when address and port are not filled out", func() {
				BeforeEach(func() {
					server.InsecureServing = &process.ListenAddr{}
				})
				It("should default an insecure port", func() {
					Expect(server.InsecureServing.Port).NotTo(BeEmpty())
				})
				It("should set URL from InsecureServing", func() {
					Expect(server.URL.String()).To(Equal("http://" + server.InsecureServing.Address + ":" + server.InsecureServing.Port))
				})
			})
		})

		Context("when neither URL or InsecureServing are set", func() {
			It("should not default either of them", func() {
				Expect(server.URL).To(BeNil(), "no URL should be set")
				Expect(server.InsecureServing).To(BeNil(), "no insecure serving details should be set")
			})
		})

		Context("when SecureServing host & port are set", func() {
			BeforeEach(func() {
				server.Address = "localhost"
				server.Port = "8675"
			})

			It("should leave SecureServing as-is", func() {
				Expect(server.SecureServing.Address).To(Equal("localhost"))
				Expect(server.SecureServing.Port).To(Equal("8675"))
			})
		})

		Context("when SecureServing is not set", func() {
			It("should be defaulted with a random port", func() {
				Expect(server.Port).NotTo(BeEquivalentTo(0))
			})
		})
	})

	It("should default authn if not set", func() {
		Expect(server.Authn).NotTo(BeNil())
	})

	Describe("argument defaulting", func() {
		// NB(directxman12): most of the templating vs configure logic is tested
		// in arguments/arguments_test.go, so just test secure vs insecure port logic here

		Context("when insecure serving is set, on a binary that supports it", func() {
			BeforeEach(func() {
				server.InsecureServing = &process.ListenAddr{
					Address: "localhost",
					Port:    "8675",
				}
				server.Path = "./testdata/fake-1.19-apiserver.sh"
			})
			It("should set the insecure-port and insecure-bind-address fields from insecureserving", func() {
				Expect(APIServerArguments(server)).To(ContainElements(
					"--insecure-port=8675",
					"--insecure-bind-address=localhost",
				))
			})
		})

		Context("when insecureserving is disabled, on binaries with no insecure-port flag", func() {
			BeforeEach(func() {
				server.Path = "./testdata/fake-1.20-apiserver.sh"
			})
			It("should not try to explicitly disable the insecure port", func() {
				Expect(APIServerArguments(server)).NotTo(ContainElement(HavePrefix("--insecure-port")))
			})
		})

		Context("when insecureserving is disabled, on binaries with an insecure-port flag", func() {
			BeforeEach(func() {
				server.Path = "./testdata/fake-1.19-apiserver.sh"
			})
			It("should explicitly disable the insecure port", func() {
				Expect(APIServerArguments(server)).To(ContainElement("--insecure-port=0"))
			})
		})

		Context("when given legacy-style template arguments", func() {
			BeforeEach(func() {
				server.Args = []string{"--foo=bar", "--baz={{ .Port }}"}
			})
			It("should use the passed in args with the minimal required defaults", func() {
				Expect(APIServerArguments(server)).To(ConsistOf(
					"--foo=bar",
					MatchRegexp(`--baz=\d+`),
					"--service-cluster-ip-range=10.0.0.0/24",
					MatchRegexp("--client-ca-file=.+"),
					"--authorization-mode=RBAC",
				))
			})
		})
	})

	// These tests assume that 'localhost' resolves to 127.0.0.1. It can resolve
	// to other addresses as well (e.g. ::1 on IPv6), but it must always resolve
	// to 127.0.0.1.
	Describe(("generated certificates"), func() {
		getCertificate := func() *x509.Certificate {
			// Read the cert file
			certFile := path.Join(server.CertDir, "apiserver.crt")
			certBytes, err := os.ReadFile(certFile)
			Expect(err).NotTo(HaveOccurred(), "should be able to read the cert file")

			// Decode and parse it
			block, remainder := pem.Decode(certBytes)
			Expect(block).NotTo(BeNil(), "should be able to decode the cert file")
			Expect(remainder).To(BeEmpty(), "should not have any extra data in the cert file")
			Expect(block.Type).To(Equal("CERTIFICATE"), "should be a certificate block")

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).NotTo(HaveOccurred(), "should be able to parse the cert file")

			return cert
		}

		Context("when SecureServing are not set", func() {
			It("should have localhost/127.0.0.1 in the certificate altnames", func() {
				cert := getCertificate()

				Expect(cert.Subject.CommonName).To(Equal("localhost"))
				Expect(cert.DNSNames).To(ConsistOf("localhost"))
				expectedIPAddresses := []net.IP{
					net.ParseIP("127.0.0.1").To4(),
					net.ParseIP(server.SecureServing.ListenAddr.Address).To4(),
				}
				Expect(cert.IPAddresses).To(ContainElements(expectedIPAddresses))
			})
		})

		Context("when SecureServing host & port are set", func() {
			BeforeEach(func() {
				server.SecureServing = SecureServing{
					ListenAddr: process.ListenAddr{
						Address: "1.2.3.4",
						Port:    "5678",
					},
				}
			})

			It("should have the host in the certificate altnames", func() {
				cert := getCertificate()

				Expect(cert.Subject.CommonName).To(Equal("localhost"))
				Expect(cert.DNSNames).To(ConsistOf("localhost"))
				expectedIPAddresses := []net.IP{
					net.ParseIP("127.0.0.1").To4(),
					net.ParseIP(server.SecureServing.ListenAddr.Address).To4(),
				}
				Expect(cert.IPAddresses).To(ContainElements(expectedIPAddresses))
			})
		})
	})

	Describe("setting up auth", func() {
		var auth *fakeAuthn
		BeforeEach(func() {
			auth = &fakeAuthn{
				setFlag: true,
			}
			server.Authn = auth
		})
		It("should configure with the cert dir", func() {
			Expect(auth.workDir).To(Equal(server.CertDir))
		})
		It("should pass its args to be configured", func() {
			Expect(server.Configure().Get("configure-called").Get(nil)).To(ConsistOf("true"))
		})

		Context("when configuring auth errors out", func() {
			It("should fail to configure", func() {
				server := &APIServer{
					EtcdURL: &url.URL{},
					SecureServing: SecureServing{
						Authn: auth,
					},
				}
				auth.configureErr = errors.New("Oh no")
				Expect(PrepareAPIServer(server)).NotTo(Succeed())
			})
		})
	})

	Describe("managing", func() {
		// some of these tests are combined for speed reasons -- starting the apiserver
		// takes a while, relatively speaking

		var (
			auth *fakeAuthn
			etcd *Etcd
		)
		BeforeEach(func() {
			etcd = &Etcd{}
			Expect(etcd.Start()).To(Succeed())
			server.EtcdURL = etcd.URL

			auth = &fakeAuthn{}
			server.Authn = auth
		})
		AfterEach(func() {
			Expect(etcd.Stop()).To(Succeed())
		})

		Context("after starting", func() {
			BeforeEach(func() {
				Expect(server.Start()).To(Succeed())
			})

			It("should stop successfully, and stop auth", func() {
				Expect(server.Stop()).To(Succeed())
				Expect(auth.stopCalled).To(BeTrue())
			})
		})

		It("should fail to start when auth fails to start", func() {
			auth.startErr = errors.New("Oh no")
			Expect(server.Start()).NotTo(Succeed())
		})

		It("should start successfully & start auth", func() {
			Expect(server.Start()).To(Succeed())
			defer func() { Expect(server.Stop()).To(Succeed()) }()
			Expect(auth.startCalled).To(BeTrue())
		})
	})
})

type fakeAuthn struct {
	workDir string

	startCalled bool
	stopCalled  bool
	setFlag     bool

	configureErr error
	startErr     error
}

func (f *fakeAuthn) Configure(workDir string, args *process.Arguments) error {
	f.workDir = workDir
	if f.setFlag {
		args.Set("configure-called", "true")
	}
	return f.configureErr
}
func (f *fakeAuthn) Start() error {
	f.startCalled = true
	return f.startErr
}
func (f *fakeAuthn) AddUser(user User, baseCfg *rest.Config) (*rest.Config, error) {
	return nil, nil
}
func (f *fakeAuthn) Stop() error {
	f.stopCalled = true
	return nil
}
