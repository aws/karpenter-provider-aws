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

package certs_test

import (
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"sigs.k8s.io/controller-runtime/pkg/internal/testing/certs"
)

var _ = Describe("TinyCA", func() {
	var ca *certs.TinyCA

	BeforeEach(func() {
		var err error
		ca, err = certs.NewTinyCA()
		Expect(err).NotTo(HaveOccurred(), "should be able to initialize the CA")
	})

	Describe("the CA certs themselves", func() {
		It("should be retrievable as a cert pair", func() {
			Expect(ca.CA.Key).NotTo(BeNil(), "should have a key")
			Expect(ca.CA.Cert).NotTo(BeNil(), "should have a cert")
		})

		It("should be usable for signing & verifying", func() {
			Expect(ca.CA.Cert.KeyUsage&x509.KeyUsageCertSign).NotTo(BeEquivalentTo(0), "should be usable for cert signing")
			Expect(ca.CA.Cert.KeyUsage&x509.KeyUsageDigitalSignature).NotTo(BeEquivalentTo(0), "should be usable for signature verifying")
		})
	})

	It("should produce unique serials among all generated certificates of all types", func() {
		By("generating a few cert pairs for both serving and client auth")
		firstCerts, err := ca.NewServingCert()
		Expect(err).NotTo(HaveOccurred())
		secondCerts, err := ca.NewClientCert(certs.ClientInfo{Name: "user"})
		Expect(err).NotTo(HaveOccurred())
		thirdCerts, err := ca.NewServingCert()
		Expect(err).NotTo(HaveOccurred())

		By("checking that they have different serials")
		serials := []*big.Int{
			firstCerts.Cert.SerialNumber,
			secondCerts.Cert.SerialNumber,
			thirdCerts.Cert.SerialNumber,
		}
		// quick uniqueness check of numbers: sort, then you only have to compare sequential entries
		sort.Slice(serials, func(i, j int) bool {
			return serials[i].Cmp(serials[j]) == -1
		})
		Expect(serials[1].Cmp(serials[0])).NotTo(Equal(0), "serials shouldn't be equal")
		Expect(serials[2].Cmp(serials[1])).NotTo(Equal(0), "serials shouldn't be equal")
	})

	Describe("Generated serving certs", func() {
		It("should be valid for short enough to avoid production usage, but long enough for long-running tests", func() {
			cert, err := ca.NewServingCert()
			Expect(err).NotTo(HaveOccurred(), "should be able to generate the serving certs")

			duration := time.Until(cert.Cert.NotAfter)
			Expect(duration).To(BeNumerically("<=", 168*time.Hour), "not-after should be short-ish (<= 1 week)")
			Expect(duration).To(BeNumerically(">=", 2*time.Hour), "not-after should be enough for long tests (couple of hours)")
		})

		Context("when encoding names", func() {
			var cert certs.CertPair
			BeforeEach(func() {
				By("generating a serving cert with IPv4 & IPv6 addresses, and DNS names")
				var err error
				// IPs are in the "example & docs" blocks for IPv4 (TEST-NET-1) & IPv6
				cert, err = ca.NewServingCert("192.0.2.1", "localhost", "2001:db8::")
				Expect(err).NotTo(HaveOccurred(), "should be able to create the serving certs")
			})

			It("should encode all non-IP names as DNS SANs", func() {
				Expect(cert.Cert.DNSNames).To(ConsistOf("localhost"))
			})

			It("should encode all IP names as IP SANs", func() {
				// NB(directxman12): this is non-exhaustive because we also
				// convert DNS SANs to IPs too (see test below)
				Expect(cert.Cert.IPAddresses).To(ContainElements(
					// normalize the elements with To16 so we can compare them to the output of
					// of ParseIP safely (the alternative is a custom matcher that calls Equal,
					// but this is easier)
					WithTransform(net.IP.To16, Equal(net.ParseIP("192.0.2.1"))),
					WithTransform(net.IP.To16, Equal(net.ParseIP("2001:db8::"))),
				))
			})

			It("should add the corresponding IP address(es) (as IP SANs) for DNS names", func() {
				// NB(directxman12): we currently fail if the lookup fails.
				// I'm not certain this is the best idea (both the bailing on
				// error and the actual idea), so if this causes issues, you
				// might want to reconsider.

				localhostAddrs, err := net.LookupHost("localhost")
				Expect(err).NotTo(HaveOccurred(), "should be able to find IPs for localhost")
				localhostIPs := make([]interface{}, len(localhostAddrs))
				for i, addr := range localhostAddrs {
					// normalize the elements with To16 so we can compare them to the output of
					// of ParseIP safely (the alternative is a custom matcher that calls Equal,
					// but this is easier)
					localhostIPs[i] = WithTransform(net.IP.To16, Equal(net.ParseIP(addr)))
				}
				Expect(cert.Cert.IPAddresses).To(ContainElements(localhostIPs...))
			})
		})

		It("should assume a name of localhost (DNS SAN) if no names are given", func() {
			cert, err := ca.NewServingCert()
			Expect(err).NotTo(HaveOccurred(), "should be able to generate a serving cert with the default name")
			Expect(cert.Cert.DNSNames).To(ConsistOf("localhost"), "the default DNS name should be localhost")

		})

		It("should be usable for server auth, verifying, and enciphering", func() {
			cert, err := ca.NewServingCert()
			Expect(err).NotTo(HaveOccurred(), "should be able to generate a serving cert")

			Expect(cert.Cert.KeyUsage&x509.KeyUsageKeyEncipherment).NotTo(BeEquivalentTo(0), "should be usable for key enciphering")
			Expect(cert.Cert.KeyUsage&x509.KeyUsageDigitalSignature).NotTo(BeEquivalentTo(0), "should be usable for signature verifying")
			Expect(cert.Cert.ExtKeyUsage).To(ContainElement(x509.ExtKeyUsageServerAuth), "should be usable for server auth")

		})

		It("should be signed by the CA", func() {
			cert, err := ca.NewServingCert()
			Expect(err).NotTo(HaveOccurred(), "should be able to generate a serving cert")
			Expect(cert.Cert.CheckSignatureFrom(ca.CA.Cert)).To(Succeed())
		})
	})

	Describe("Generated client certs", func() {
		var cert certs.CertPair
		BeforeEach(func() {
			var err error
			cert, err = ca.NewClientCert(certs.ClientInfo{
				Name:   "user",
				Groups: []string{"group1", "group2"},
			})
			Expect(err).NotTo(HaveOccurred(), "should be able to create a client cert")
		})

		It("should be valid for short enough to avoid production usage, but long enough for long-running tests", func() {
			duration := time.Until(cert.Cert.NotAfter)
			Expect(duration).To(BeNumerically("<=", 168*time.Hour), "not-after should be short-ish (<= 1 week)")
			Expect(duration).To(BeNumerically(">=", 2*time.Hour), "not-after should be enough for long tests (couple of hours)")
		})

		It("should be usable for client auth, verifying, and enciphering", func() {
			Expect(cert.Cert.KeyUsage&x509.KeyUsageKeyEncipherment).NotTo(BeEquivalentTo(0), "should be usable for key enciphering")
			Expect(cert.Cert.KeyUsage&x509.KeyUsageDigitalSignature).NotTo(BeEquivalentTo(0), "should be usable for signature verifying")
			Expect(cert.Cert.ExtKeyUsage).To(ContainElement(x509.ExtKeyUsageClientAuth), "should be usable for client auth")
		})

		It("should encode the user name as the common name", func() {
			Expect(cert.Cert.Subject.CommonName).To(Equal("user"))
		})

		It("should encode the groups as the organization values", func() {
			Expect(cert.Cert.Subject.Organization).To(ConsistOf("group1", "group2"))
		})

		It("should be signed by the CA", func() {
			Expect(cert.Cert.CheckSignatureFrom(ca.CA.Cert)).To(Succeed())
		})
	})
})

var _ = Describe("Certificate Pairs", func() {
	var pair certs.CertPair
	BeforeEach(func() {
		ca, err := certs.NewTinyCA()
		Expect(err).NotTo(HaveOccurred(), "should be able to generate a cert pair")

		pair = ca.CA
	})

	Context("when serializing just the public key", func() {
		It("should serialize into a CERTIFICATE PEM block", func() {
			bytes := pair.CertBytes()
			Expect(bytes).NotTo(BeEmpty(), "should produce some cert bytes")

			block, rest := pem.Decode(bytes)
			Expect(rest).To(BeEmpty(), "shouldn't have any data besides the PEM block")

			Expect(block).To(PointTo(MatchAllFields(Fields{
				"Type":    Equal("CERTIFICATE"),
				"Headers": BeEmpty(),
				"Bytes":   Equal(pair.Cert.Raw),
			})))
		})
	})

	Context("when serializing both parts", func() {
		var certBytes, keyBytes []byte
		BeforeEach(func() {
			var err error
			certBytes, keyBytes, err = pair.AsBytes()
			Expect(err).NotTo(HaveOccurred(), "should be able to serialize the pair")
		})

		It("should serialize the private key in PKCS8 form in a PRIVATE KEY PEM block", func() {
			Expect(keyBytes).NotTo(BeEmpty(), "should produce some key bytes")

			By("decoding & checking the PEM block")
			block, rest := pem.Decode(keyBytes)
			Expect(rest).To(BeEmpty(), "shouldn't have any data besides the PEM block")

			Expect(block.Type).To(Equal("PRIVATE KEY"))

			By("decoding & checking the PKCS8 data")
			Expect(x509.ParsePKCS8PrivateKey(block.Bytes)).NotTo(BeNil(), "should be able to parse back the private key")
		})

		It("should serialize the public key into a CERTIFICATE PEM block", func() {
			Expect(certBytes).NotTo(BeEmpty(), "should produce some cert bytes")

			block, rest := pem.Decode(certBytes)
			Expect(rest).To(BeEmpty(), "shouldn't have any data besides the PEM block")

			Expect(block).To(PointTo(MatchAllFields(Fields{
				"Type":    Equal("CERTIFICATE"),
				"Headers": BeEmpty(),
				"Bytes":   Equal(pair.Cert.Raw),
			})))
		})

	})
})
