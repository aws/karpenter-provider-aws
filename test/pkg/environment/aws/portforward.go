/*
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

package aws

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/transport/spdy"
)

// ExpectPortForwardFor starts a port-forwarded connection and runs the function f() on a single pod that matches the labels
func ExpectPortForwardFor(ctx context.Context, config *rest.Config, c client.Client, f func(), labels map[string]string, port string) {
	GinkgoHelper()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	podList := &v1.PodList{}
	Expect(c.List(ctx, podList, client.MatchingLabels(labels), client.Limit(1))).To(Succeed())
	Expect(podList.Items).To(HaveLen(1))
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", podList.Items[0].Namespace, podList.Items[0].Name)
	serverURL := url.URL{Scheme: "https", Path: path, Host: strings.TrimLeft(config.Host, "htps:/")}

	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	Expect(err).ToNot(HaveOccurred())

	readyChan := make(chan struct{}, 1)
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)
	forwarder, err := portforward.New(dialer, []string{port}, ctx.Done(), readyChan, new(bytes.Buffer), new(bytes.Buffer))
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		Expect(forwarder.ForwardPorts()).To(Succeed())
	}()

	// Run the function while the port-forwarded connection is alive
	select {
	// Kubernetes closes the readyChan when the port-forward is ready
	case <-readyChan:
		f()
	case <-ctx.Done():
	}

}
