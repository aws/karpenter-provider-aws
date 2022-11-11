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
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/samber/lo"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/config/settings"
)

func (env *Environment) ExpectInstance(nodeName string) Assertion {
	return Expect(env.GetInstance(nodeName))
}

func (env *Environment) ExpectIPv6ClusterDNS() string {
	dnsService, err := env.Environment.KubeClient.CoreV1().Services("kube-system").Get(env.Context, "kube-dns", metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	kubeDNSIP := net.ParseIP(dnsService.Spec.ClusterIP)
	Expect(kubeDNSIP.To4()).To(BeNil())
	return kubeDNSIP.String()
}

func (env *Environment) GetInstance(nodeName string) ec2.Instance {
	node := env.Environment.GetNode(nodeName)
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	instance, err := env.EC2API.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, instance.Reservations).To(HaveLen(1))
	ExpectWithOffset(1, instance.Reservations[0].Instances).To(HaveLen(1))
	return *instance.Reservations[0].Instances[0]
}

func (env *Environment) ExpectInstanceStopped(nodeName string) {
	node := env.Environment.GetNode(nodeName)
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	_, err := env.EC2API.StopInstances(&ec2.StopInstancesInput{
		Force:       aws.Bool(true),
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) ExpectInstanceTerminated(nodeName string) {
	node := env.Environment.GetNode(nodeName)
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) GetVolume(volumeID *string) ec2.Volume {
	dvo, err := env.EC2API.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: []*string{volumeID}})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, len(dvo.Volumes)).To(Equal(1))
	return *dvo.Volumes[0]
}

func (env *Environment) ExpectQueueExists() {
	exists, err := env.SQSProvider.QueueExists(env.Context)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, exists).To(BeTrue())
}

func (env *Environment) ExpectMessagesCreated(msgs ...interface{}) {
	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	var err error
	for _, msg := range msgs {
		wg.Add(1)
		go func(m interface{}) {
			defer wg.Done()
			defer GinkgoRecover()
			_, e := env.SQSProvider.SendMessage(env.Environment.Context, m)
			if e != nil {
				mu.Lock()
				err = multierr.Append(err, e)
				mu.Unlock()
			}
		}(msg)
	}
	wg.Wait()
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) ExpectWindowsUserData() string {
	caBundle, err := getCABundle(env.Context, env.Config)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	kubeDNS, err := kubeDNSIP(env.Context, env.KubeClient)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return fmt.Sprintf(`<powershell>
[string]$EKSBootstrapScriptFile = "$env:ProgramFiles\Amazon\EKS\Start-EKSBootstrap.ps1"
& $EKSBootstrapScriptFile -EKSClusterName "%s" -APIServerEndpoint "%s" -Base64ClusterCA "%s" -DNSClusterIP "%s" -ContainerRuntime "docker"
</powershell>`,
		settings.FromContext(env.Context).ClusterName, settings.FromContext(env.Context).ClusterEndpoint, lo.FromPtr(caBundle), kubeDNS.String())
}

func getCABundle(ctx context.Context, restConfig *rest.Config) (*string, error) {
	// Discover CA Bundle from the REST client. We could alternatively
	// have used the simpler client-go InClusterConfig() method.
	// However, that only works when Karpenter is running as a Pod
	// within the same cluster it's managing.
	transportConfig, err := restConfig.TransportConfig()
	if err != nil {
		return nil, fmt.Errorf("discovering caBundle, loading transport config, %w", err)
	}
	_, err = transport.TLSConfigFor(transportConfig) // fills in CAData!
	if err != nil {
		return nil, fmt.Errorf("discovering caBundle, loading TLS config, %w", err)
	}
	logging.FromContext(ctx).Debugf("Discovered caBundle, length %d", len(transportConfig.TLS.CAData))
	return lo.ToPtr(base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)), nil
}

func kubeDNSIP(ctx context.Context, kubernetesInterface kubernetes.Interface) (net.IP, error) {
	if kubernetesInterface == nil {
		return nil, fmt.Errorf("no K8s client provided")
	}
	dnsService, err := kubernetesInterface.CoreV1().Services("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ret := net.ParseIP(dnsService.Spec.ClusterIP)
	if ret == nil {
		return nil, fmt.Errorf("parsing cluster IP")
	}
	return ret, nil
}
