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
	"strings"

	"github.com/aws/karpenter/pkg/apis"
	awssettings "github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
	"github.com/aws/karpenter/pkg/utils"

	"github.com/aws/karpenter-core/pkg/utils/resources"

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

	"github.com/aws/karpenter/pkg/cloudprovider/amifamily"
	awscontext "github.com/aws/karpenter/pkg/context"

	"github.com/aws/karpenter-core/pkg/scheduling"

	coreapis "github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
)

const (
	// MaxInstanceTypes defines the number of instance type options to pass to CreateFleet
	MaxInstanceTypes = 60
)

func init() {
	v1alpha5.NormalizedLabels = lo.Assign(v1alpha5.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": v1.LabelTopologyZone})
	coreapis.Settings = append(coreapis.Settings, apis.Settings...)
}

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	instanceTypeProvider *InstanceTypeProvider
	instanceProvider     *InstanceProvider
	kubeClient           k8sClient.Client
	amiProvider          *amifamily.AMIProvider
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
	subnetProvider := subnet.NewProvider(ec2api)
	instanceTypeProvider := NewInstanceTypeProvider(ctx, ctx.Session, ec2api, subnetProvider, ctx.UnavailableOfferingsCache, ctx.StartAsync)
	amiProvider := amifamily.NewAMIProvider(ctx.KubeClient, ctx.KubernetesInterface, ssm.New(ctx.Session), ec2api,
		cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval), cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval), cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval))
	amiResolver := amifamily.New(ctx.KubeClient, amiProvider)
	return &CloudProvider{
		kubeClient:           ctx.KubeClient,
		instanceTypeProvider: instanceTypeProvider,
		amiProvider:          amiProvider,
		instanceProvider: NewInstanceProvider(
			ctx,
			aws.StringValue(ctx.Session.Config.Region),
			ec2api,
			ctx.UnavailableOfferingsCache,
			instanceTypeProvider,
			subnetProvider,
			NewLaunchTemplateProvider(
				ctx,
				ec2api,
				amiResolver,
				securitygroup.NewProvider(ec2api),
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

// Create a machine given the constraints.
func (c *CloudProvider) Create(ctx context.Context, machine *v1alpha5.Machine) (*v1alpha5.Machine, error) {
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
	instance, err := c.instanceProvider.Create(ctx, nodeTemplate, machine, instanceTypes)
	if err != nil {
		return nil, fmt.Errorf("creating instance, %w", err)
	}
	// Resolves instance details into a machine
	created, err := c.instanceToMachine(ctx, instance, instanceTypes)
	if err != nil {
		return nil, fmt.Errorf("resolving instance details into machine, %w", err)
	}
	return created, nil
}

// TODO @joinnis: Remove provisionerName from this call signature once we decouple provisioner from GetInstanceTypes
func (c *CloudProvider) Get(ctx context.Context, machineName, provisionerName string) (*v1alpha5.Machine, error) {
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner); err != nil {
		return nil, fmt.Errorf("getting machine provisioner, %w", err)
	}
	instanceTypes, err := c.GetInstanceTypes(ctx, provisioner)
	if err != nil {
		return nil, fmt.Errorf("resolving instance types, %w", err)
	}
	if len(instanceTypes) == 0 {
		return nil, fmt.Errorf("all requested instance types were unavailable during launch")
	}
	instance, err := c.instanceProvider.Get(ctx, machineName)
	if err != nil {
		return nil, fmt.Errorf("getting instance, %w", err)
	}
	// Resolves instance details into a machine
	machine, err := c.instanceToMachine(ctx, instance, instanceTypes)
	if err != nil {
		return nil, fmt.Errorf("resolving instance details into machine, %w", err)
	}
	return machine, nil
}

func (c *CloudProvider) LivenessProbe(req *http.Request) error {
	if err := c.instanceTypeProvider.LivenessProbe(req); err != nil {
		return err
	}
	return nil
}

// GetInstanceTypes returns all available InstanceTypes
func (c *CloudProvider) GetInstanceTypes(ctx context.Context, provisioner *v1alpha5.Provisioner) ([]*cloudprovider.InstanceType, error) {
	var rawProvider []byte
	if provisioner.Spec.Provider != nil {
		rawProvider = provisioner.Spec.Provider.Raw
	}
	nodeTemplate, err := c.resolveNodeTemplate(ctx, rawProvider, provisioner.Spec.ProviderRef)
	if err != nil {
		return nil, err
	}
	// TODO, break this coupling
	instanceTypes, err := c.instanceTypeProvider.List(ctx, provisioner.Spec.KubeletConfiguration, nodeTemplate)
	if err != nil {
		return nil, err
	}
	return instanceTypes, nil
}

// TODO @joinnis: Migrate this delete call to use tag-based deleting when machine migration is done
func (c *CloudProvider) Delete(ctx context.Context, machine *v1alpha5.Machine) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("machine", machine.Name))
	id, err := utils.ParseInstanceID(machine.Status.ProviderID)
	if err != nil {
		return fmt.Errorf("getting instance ID, %w", err)
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("id", id))
	return c.instanceProvider.DeleteByID(ctx, id)
}

func (c *CloudProvider) IsMachineDrifted(ctx context.Context, machine *v1alpha5.Machine) (bool, error) {
	// Not needed when GetInstanceTypes removes provisioner dependency
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: machine.Labels[v1alpha5.ProvisionerNameLabelKey]}, provisioner); err != nil {
		return false, k8sClient.IgnoreNotFound(fmt.Errorf("getting provisioner, %w", err))
	}
	if provisioner.Spec.ProviderRef == nil {
		return false, nil
	}
	nodeTemplate, err := c.resolveNodeTemplate(ctx, nil, provisioner.Spec.ProviderRef)
	if err != nil {
		return false, k8sClient.IgnoreNotFound(fmt.Errorf("resolving node template, %w", err))
	}
	amiDrifted, err := c.isAMIDrifted(ctx, machine, provisioner, nodeTemplate)
	if err != nil {
		return false, err
	}
	return amiDrifted, nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "aws"
}

func (c *CloudProvider) isAMIDrifted(ctx context.Context, machine *v1alpha5.Machine, provisioner *v1alpha5.Provisioner, nodeTemplate *v1alpha1.AWSNodeTemplate) (bool, error) {
	instanceTypes, err := c.GetInstanceTypes(ctx, provisioner)
	if err != nil {
		return false, fmt.Errorf("getting instanceTypes, %w", err)
	}
	nodeInstanceType, found := lo.Find(instanceTypes, func(instType *cloudprovider.InstanceType) bool {
		return instType.Name == machine.Labels[v1.LabelInstanceTypeStable]
	})
	if !found {
		return false, fmt.Errorf(`finding node instance type "%s"`, machine.Labels[v1.LabelInstanceTypeStable])
	}
	if nodeTemplate.Spec.LaunchTemplateName != nil {
		return false, nil
	}
	amis, err := c.amiProvider.Get(ctx, nodeTemplate, []*cloudprovider.InstanceType{nodeInstanceType},
		amifamily.GetAMIFamily(nodeTemplate.Spec.AMIFamily, &amifamily.Options{}))
	if err != nil {
		return false, fmt.Errorf("getting amis, %w", err)
	}
	// Get InstanceID to fetch from EC2
	instanceID, err := utils.ParseInstanceID(machine.Status.ProviderID)
	if err != nil {
		return false, err
	}
	instance, err := c.instanceProvider.GetByID(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("getting instance, %w", err)
	}
	return !lo.Contains(lo.Keys(amis), *instance.ImageId), nil
}

func (c *CloudProvider) resolveNodeTemplate(ctx context.Context, raw []byte, objRef *v1alpha5.ProviderRef) (*v1alpha1.AWSNodeTemplate, error) {
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

func (c *CloudProvider) resolveInstanceTypes(ctx context.Context, machine *v1alpha5.Machine) ([]*cloudprovider.InstanceType, error) {
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
		return reqs.Compatible(i.Requirements) == nil &&
			len(i.Offerings.Requirements(reqs).Available()) > 0 &&
			resources.Fits(resources.Merge(machine.Spec.Resources.Requests, i.Overhead.Total()), i.Capacity)
	}), nil
}

// TODO @joinnis: Remove name resolution when Machine migration is completed
func (c *CloudProvider) instanceToMachine(ctx context.Context, instance *ec2.Instance, instanceTypes []*cloudprovider.InstanceType) (*v1alpha5.Machine, error) {
	machine := &v1alpha5.Machine{}
	instanceType, found := lo.Find(instanceTypes, func(i *cloudprovider.InstanceType) bool {
		return aws.StringValue(instance.InstanceType) == i.Name
	})
	if !found {
		// This should never happen since the launched instance should always come from supported instance types
		return nil, fmt.Errorf("launched instance type not included in instance types set")
	}
	labels := map[string]string{}
	for key, req := range instanceType.Requirements {
		if req.Len() == 1 {
			labels[key] = req.Values()[0]
		}
	}
	labels[v1alpha1.LabelInstanceAMIID] = aws.StringValue(instance.ImageId)
	labels[v1.LabelTopologyZone] = aws.StringValue(instance.Placement.AvailabilityZone)
	labels[v1alpha5.LabelCapacityType] = getCapacityType(instance)
	labels[v1alpha5.MachineNameLabelKey] = machine.Name

	machine.Name = lo.Ternary(
		awssettings.FromContext(ctx).NodeNameConvention == awssettings.ResourceName,
		aws.StringValue(instance.InstanceId),
		strings.ToLower(aws.StringValue(instance.PrivateDnsName)),
	)
	machine.Labels = labels
	machine.Status.ProviderID = fmt.Sprintf("aws:///%s/%s", aws.StringValue(instance.Placement.AvailabilityZone), aws.StringValue(instance.InstanceId))

	machine.Status.Capacity = v1.ResourceList{}
	for k, v := range instanceType.Capacity {
		if !resources.IsZero(v) {
			machine.Status.Capacity[k] = v
		}
	}
	machine.Status.Allocatable = v1.ResourceList{}
	for k, v := range instanceType.Allocatable() {
		if !resources.IsZero(v) {
			machine.Status.Allocatable[k] = v
		}
	}
	return machine, nil
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
