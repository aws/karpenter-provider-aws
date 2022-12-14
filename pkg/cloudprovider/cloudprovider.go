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

package cloudprovider

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/amifamily"
	awscontext "github.com/aws/karpenter/pkg/context"

	coreapis "github.com/aws/karpenter-core/pkg/apis"
	corev1alpha1 "github.com/aws/karpenter-core/pkg/apis/v1alpha1"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
)

const (
	// MaxInstanceTypes defines the number of instance type options to pass to CreateFleet
	MaxInstanceTypes = 60
)

func init() {
	v1alpha5.NormalizedLabels = lo.Assign(v1alpha5.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": v1.LabelTopologyZone})
	coreapis.Settings = coreapis.Settings.Union(apis.Settings)
	lo.Must0(apis.AddToScheme(scheme.Scheme))
}

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	instanceTypeProvider *InstanceTypeProvider
	instanceProvider     *InstanceProvider
	kubeClient           k8sClient.Client
}

func New(ctx awscontext.Context) *CloudProvider {
	kubeDNSIP, err := kubeDNSIP(ctx, ctx.KubernetesInterface)
	if err != nil {
		logging.FromContext(ctx).Debugf("unable to detect the IP of the kube-dns service, %s", err)
	} else {
		logging.FromContext(ctx).With("kube-dns-ip", kubeDNSIP).Debugf("discovered kube dns")
	}
	ec2api := ec2.New(ctx.Session)
	if err := checkEC2Connectivity(ctx, ec2api); err != nil {
		logging.FromContext(ctx).Fatalf("Checking EC2 API connectivity, %s", err)
	}
	subnetProvider := NewSubnetProvider(ec2api)
	instanceTypeProvider := NewInstanceTypeProvider(ctx, ctx.Session, ec2api, subnetProvider, ctx.UnavailableOfferingsCache, ctx.StartAsync)
	return &CloudProvider{
		kubeClient:           ctx.KubeClient,
		instanceTypeProvider: instanceTypeProvider,
		instanceProvider: NewInstanceProvider(ctx, ec2api, instanceTypeProvider, subnetProvider,
			NewLaunchTemplateProvider(
				ctx,
				ec2api,
				ctx.KubernetesInterface,
				amifamily.New(ctx.KubeClient, ssm.New(ctx.Session), ec2api, cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval), cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)),
				NewSecurityGroupProvider(ec2api),
				lo.Must(getCABundle(ctx.RESTConfig)),
				ctx.StartAsync,
				kubeDNSIP,
			),
		),
	}
}

// checkEC2Connectivity makes a dry-run call to DescribeInstanceTypes.  If it fails, we provide an early indicator that we
// are having issues connecting to the EC2 API.
func checkEC2Connectivity(ctx context.Context, api *ec2.EC2) error {
	_, err := api.DescribeInstanceTypesWithContext(ctx, &ec2.DescribeInstanceTypesInput{DryRun: aws.Bool(true)})
	var aerr awserr.Error
	if errors.As(err, &aerr) && aerr.Code() == "DryRunOperation" {
		return nil
	}
	return err
}

// Create a node given the constraints.
func (c *CloudProvider) Create(ctx context.Context, machine *corev1alpha1.Machine) (*v1.Node, error) {
	nodeTemplate, err := c.resolveNodeTemplate(ctx, []byte(machine.
		Annotations[v1alpha5.ProviderCompatabilityAnnotationKey]), machine.
		Spec.MachineTemplateRef)
	if err != nil {
		return nil, fmt.Errorf("resolving node template, %w", err)
	}
	instanceTypes, err := c.resolveInstanceTypes(ctx, machine)
	if err != nil {
		return nil, fmt.Errorf("resolving instance types, %w", err)
	}
	if len(instanceTypes) == 0 {
		return nil, fmt.Errorf("all requested instance types were unavailable during launch")
	}
	return c.instanceProvider.Create(ctx, nodeTemplate, machine, instanceTypes)
}

func (c *CloudProvider) LivenessProbe(req *http.Request) error {
	if err := c.instanceTypeProvider.LivenessProbe(req); err != nil {
		return err
	}
	return nil
}

// GetInstanceTypes returns all available InstanceTypes
func (c *CloudProvider) GetInstanceTypes(ctx context.Context, provisioner *v1alpha5.Provisioner) ([]*cloudprovider.InstanceType, error) {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	var err error
	if provisioner.Spec.ProviderRef != nil {
		nodeTemplate, err = c.resolveNodeTemplate(ctx, nil, &v1.ObjectReference{
			APIVersion: provisioner.Spec.ProviderRef.APIVersion,
			Kind:       provisioner.Spec.ProviderRef.Kind,
			Name:       provisioner.Spec.ProviderRef.Name,
		})
	} else {
		nodeTemplate, err = c.resolveNodeTemplate(ctx, provisioner.Spec.Provider.Raw, nil)
	}
	if err != nil {
		return nil, err
	}
	// TODO, break this coupling
	instanceTypes, err := c.instanceTypeProvider.Get(ctx, provisioner.Spec.KubeletConfiguration, nodeTemplate)
	if err != nil {
		return nil, err
	}
	return instanceTypes, nil
}

func (c *CloudProvider) IsMachineDrifted(_ context.Context, _ *corev1alpha1.Machine) (bool, error) {
	return false, nil
}

func (c *CloudProvider) Delete(ctx context.Context, node *v1.Node) error {
	return c.instanceProvider.Terminate(ctx, node)
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "aws"
}

func getCABundle(restConfig *rest.Config) (*string, error) {
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
	return ptr.String(base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)), nil
}

func kubeDNSIP(ctx context.Context, kubernetesInterface kubernetes.Interface) (net.IP, error) {
	if kubernetesInterface == nil {
		return nil, fmt.Errorf("no K8s client provided")
	}
	dnsService, err := kubernetesInterface.CoreV1().Services("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	kubeDNSIP := net.ParseIP(dnsService.Spec.ClusterIP)
	if kubeDNSIP == nil {
		return nil, fmt.Errorf("parsing cluster IP")
	}
	return kubeDNSIP, nil
}

func (c *CloudProvider) resolveNodeTemplate(ctx context.Context, raw []byte, objRef *v1.ObjectReference) (*v1alpha1.AWSNodeTemplate, error) {
	nodeTemplate := &v1alpha1.AWSNodeTemplate{}
	if objRef != nil {
		if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: objRef.Name}, nodeTemplate); err != nil {
			return nil, fmt.Errorf("getting providerRef, %w", err)
		}
		return nodeTemplate, nil
	}
	aws, err := v1alpha1.DeserializeProvider(raw)
	if err != nil {
		return nil, err
	}
	nodeTemplate.Spec.AWS = lo.FromPtr(aws)
	return nodeTemplate, nil
}

func (c *CloudProvider) resolveInstanceTypes(ctx context.Context, machine *corev1alpha1.Machine) ([]*cloudprovider.InstanceType, error) {
	provisionerName, ok := machine.Labels[v1alpha5.ProvisionerNameLabelKey]
	if !ok {
		return nil, fmt.Errorf("finding provisioner owner")
	}
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner); err != nil {
		return nil, fmt.Errorf("getting provisioner owner, %w", err)
	}
	instanceTypes, err := c.GetInstanceTypes(ctx, provisioner)
	if err != nil {
		return nil, fmt.Errorf("getting instance types, %w", err)
	}
	reqs := scheduling.NewNodeSelectorRequirements(machine.Spec.Requirements...)
	return lo.Filter(instanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
		return reqs.Get(v1.LabelInstanceTypeStable).Has(i.Name) && len(i.Offerings.Requirements(reqs).Available()) > 0
	}), nil
}
