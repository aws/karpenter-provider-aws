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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	knativeapis "knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"

	coreapis "github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter-core/pkg/utils/functional"

	"github.com/aws/karpenter/pkg/cloudprovider/amifamily"
	awscontext "github.com/aws/karpenter/pkg/context"
)

const (
	// MaxInstanceTypes defines the number of instance type options to pass to CreateFleet
	MaxInstanceTypes = 20
)

func init() {
	v1alpha5.NormalizedLabels = lo.Assign(v1alpha5.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": v1.LabelTopologyZone})
	coreapis.Resources = lo.Assign(coreapis.Resources, apis.Resources)
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
		logging.FromContext(ctx).Fatalf("Unable to detect the IP of the kube-dns service, %s", err)
	}
	logging.FromContext(ctx).Debugf("Discovered DNS IP %s", kubeDNSIP)
	ec2api := ec2.New(ctx.Session)
	if err := checkEC2Connectivity(ctx, ec2api); err != nil {
		logging.FromContext(ctx).Fatalf("Checking EC2 API connectivity, %s", err)
	}
	subnetProvider := NewSubnetProvider(ec2api)
	instanceTypeProvider := NewInstanceTypeProvider(ctx, ctx.Session, ec2api, subnetProvider, ctx.UnavailableOfferingsCache, ctx.StartAsync)
	cloudprovider := &CloudProvider{
		instanceTypeProvider: instanceTypeProvider,
		instanceProvider: NewInstanceProvider(ctx, ec2api, instanceTypeProvider, subnetProvider,
			NewLaunchTemplateProvider(
				ctx,
				ec2api,
				ctx.KubernetesInterface,
				amifamily.New(ctx.KubeClient, ssm.New(ctx.Session), ec2api, cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval), cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)),
				NewSecurityGroupProvider(ec2api),
				lo.Must(getCABundle(ctx, ctx.RESTConfig)),
				ctx.StartAsync,
				kubeDNSIP,
			),
		),
		kubeClient: ctx.KubeClient,
	}
	v1alpha5.ValidateHook = cloudprovider.Validate
	v1alpha5.DefaultHook = cloudprovider.Default

	return cloudprovider
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
func (c *CloudProvider) Create(ctx context.Context, nodeRequest *cloudprovider.NodeRequest) (*v1.Node, error) {
	aws, err := c.getProvider(ctx, nodeRequest.Template.Provider, nodeRequest.Template.ProviderRef)
	if err != nil {
		return nil, err
	}
	return c.instanceProvider.Create(ctx, aws, nodeRequest)
}

func (c *CloudProvider) LivenessProbe(req *http.Request) error {
	if err := c.instanceTypeProvider.LivenessProbe(req); err != nil {
		return err
	}
	return nil
}

// GetInstanceTypes returns all available InstanceTypes
func (c *CloudProvider) GetInstanceTypes(ctx context.Context, provisioner *v1alpha5.Provisioner) ([]cloudprovider.InstanceType, error) {
	aws, err := c.getProvider(ctx, provisioner.Spec.Provider, provisioner.Spec.ProviderRef)
	if err != nil {
		return nil, err
	}
	instanceTypes, err := c.instanceTypeProvider.Get(ctx, aws, provisioner.Spec.KubeletConfiguration)
	if err != nil {
		return nil, err
	}

	// if the provisioner is not supplying a list of instance types or families, perform some filtering to get instance
	// types that are suitable for general workloads
	if c.useOpinionatedInstanceFilter(provisioner.Spec.Requirements...) {
		instanceTypes = lo.Filter(instanceTypes, func(it cloudprovider.InstanceType, _ int) bool {
			cit, ok := it.(*InstanceType)
			if !ok {
				return true
			}

			// c3, m3 and r3 aren't current generation but are fine for general workloads
			if functional.HasAnyPrefix(*cit.InstanceType, "c3", "m3", "r3") {
				return true
			}

			// filter out all non-current generation
			if cit.CurrentGeneration != nil && !*cit.CurrentGeneration {
				return false
			}

			// t2 is current generation but has different bursting behavior and u- isn't widely available
			if functional.HasAnyPrefix(*cit.InstanceType, "t2", "u-") {
				return false
			}
			return true
		})
	}
	return instanceTypes, nil
}

func (c *CloudProvider) Delete(ctx context.Context, node *v1.Node) error {
	return c.instanceProvider.Terminate(ctx, node)
}

// Validate the provisioner
func (*CloudProvider) Validate(ctx context.Context, provisioner *v1alpha5.Provisioner) *knativeapis.FieldError {
	// The receiver is intentionally omitted here as when used by the webhook, Validate/Default are the only methods
	// called and we don't fully initialize the CloudProvider to prevent some network calls to EC2/Pricing.
	if provisioner.Spec.Provider == nil {
		return nil
	}
	provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
	if err != nil {
		return knativeapis.ErrGeneric(err.Error())
	}
	return provider.Validate()
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "aws"
}

// Default the provisioner
func (*CloudProvider) Default(ctx context.Context, provisioner *v1alpha5.Provisioner) {
	defaultLabels(provisioner)
}

func defaultLabels(provisioner *v1alpha5.Provisioner) {
	for key, value := range map[string]string{
		v1alpha5.LabelCapacityType: ec2.DefaultTargetCapacityTypeOnDemand,
		v1.LabelArchStable:         v1alpha5.ArchitectureAmd64,
	} {
		hasLabel := false
		if _, ok := provisioner.Spec.Labels[key]; ok {
			hasLabel = true
		}
		for _, requirement := range provisioner.Spec.Requirements {
			if requirement.Key == key {
				hasLabel = true
			}
		}
		if !hasLabel {
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
				Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value},
			})
		}
	}
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
	return kubeDNSIP, nil
}

func (c *CloudProvider) getProvider(ctx context.Context, provider *runtime.RawExtension, providerRef *v1alpha5.ProviderRef) (*v1alpha1.AWS, error) {
	if providerRef != nil {
		var ant v1alpha1.AWSNodeTemplate
		if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: providerRef.Name}, &ant); err != nil {
			return nil, fmt.Errorf("getting providerRef, %w", err)
		}
		return &ant.Spec.AWS, nil
	}
	aws, err := v1alpha1.Deserialize(provider)
	if err != nil {
		return nil, err
	}
	return aws, nil
}

func (c *CloudProvider) useOpinionatedInstanceFilter(provisionerRequirements ...v1.NodeSelectorRequirement) bool {
	var instanceRequirements []v1.NodeSelectorRequirement
	requirementKeys := []string{v1.LabelInstanceTypeStable, v1alpha1.LabelInstanceFamily, v1alpha1.LabelInstanceCategory, v1alpha1.LabelInstanceGeneration}

	for _, r := range provisionerRequirements {
		if lo.Contains(requirementKeys, r.Key) {
			instanceRequirements = append(instanceRequirements, r)
		}
	}
	// no provisioner instance type filtering, so use our opinionated list
	if len(instanceRequirements) == 0 {
		return true
	}

	for _, req := range instanceRequirements {
		switch req.Operator {
		case v1.NodeSelectorOpIn, v1.NodeSelectorOpExists, v1.NodeSelectorOpDoesNotExist:
			// v1.NodeSelectorOpIn: provisioner supplies its own list of instance types/families, so use that instead of filtering
			// v1.NodeSelectorOpExists: provisioner explicitly is asking for no filtering
			// v1.NodeSelectorOpDoesNotExist: this shouldn't match any instance type at provisioning time, but avoid filtering anyway
			return false
		case v1.NodeSelectorOpNotIn, v1.NodeSelectorOpGt, v1.NodeSelectorOpLt:
			// provisioner further restricts instance types/families, so we can possibly use our list and it will
			// be filtered more
		}
	}

	// provisioner requirements haven't prevented us from filtering
	return true
}
