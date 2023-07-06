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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var registry *prometheus.Registry

func init() {
	registry = prometheus.NewRegistry()
	registry.MustRegister(provisioningDuration, deprovisioningDuration)
}

type PrometheusPusher interface {
	Add() error
}

var _ PrometheusPusher = (*PortForwardedPrometheusPusher)(nil)
var _ PrometheusPusher = (*NoOpPrometheusPusher)(nil)

const (
	pushGatewayPort = "9091"
	pushGatewayName = "prometheus-pushgateway"
)

var pushGatewayLabels = map[string]string{
	"app.kubernetes.io/instance": pushGatewayName,
	"app.kubernetes.io/name":     pushGatewayName,
}

type PortForwardedPrometheusPusher struct {
	ctx       context.Context
	config    *rest.Config
	k8sClient client.Client
	pusher    PrometheusPusher
}

func NewPortForwardedPrometheusPusher(ctx context.Context, config *rest.Config, k8sClient client.Client) *PortForwardedPrometheusPusher {
	return &PortForwardedPrometheusPusher{
		ctx:       ctx,
		config:    config,
		k8sClient: k8sClient,
		pusher:    push.New(fmt.Sprintf("http://localhost:%s", pushGatewayPort), "karpenter_scale_testing").Gatherer(registry),
	}
}

func (p *PortForwardedPrometheusPusher) Add() error {
	GinkgoHelper()

	ExpectPortForwardFor(p.ctx, p.config, p.k8sClient, func() {
		Expect(p.pusher.Add()).To(Succeed())
	}, pushGatewayLabels, pushGatewayPort)
	return nil
}

type NoOpPrometheusPusher struct{}

func (o *NoOpPrometheusPusher) Add() error {
	return nil
}
