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

package addr_test

import (
	"net"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/internal/testing/addr"
)

var _ = Describe("SuggestAddress", func() {
	It("returns a free port and an address to bind to", func() {
		port, host, err := addr.Suggest("")

		Expect(err).NotTo(HaveOccurred())
		Expect(host).To(Or(Equal("127.0.0.1"), Equal("::1")))
		Expect(port).NotTo(Equal(0))

		addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		Expect(err).NotTo(HaveOccurred())
		l, err := net.ListenTCP("tcp", addr)
		defer func() {
			Expect(l.Close()).To(Succeed())
		}()
		Expect(err).NotTo(HaveOccurred())
	})

	It("supports an explicit listenHost", func() {
		port, host, err := addr.Suggest("localhost")

		Expect(err).NotTo(HaveOccurred())
		Expect(host).To(Or(Equal("127.0.0.1"), Equal("::1")))
		Expect(port).NotTo(Equal(0))

		addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		Expect(err).NotTo(HaveOccurred())
		l, err := net.ListenTCP("tcp", addr)
		defer func() {
			Expect(l.Close()).To(Succeed())
		}()
		Expect(err).NotTo(HaveOccurred())
	})

	It("supports a 0.0.0.0 listenHost", func() {
		port, host, err := addr.Suggest("0.0.0.0")

		Expect(err).NotTo(HaveOccurred())
		Expect(host).To(Equal("0.0.0.0"))
		Expect(port).NotTo(Equal(0))

		addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		Expect(err).NotTo(HaveOccurred())
		l, err := net.ListenTCP("tcp", addr)
		defer func() {
			Expect(l.Close()).To(Succeed())
		}()
		Expect(err).NotTo(HaveOccurred())
	})
})
