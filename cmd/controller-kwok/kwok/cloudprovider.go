package kwok

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/docker/pkg/namesgenerator"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/providers/pricing"
)

//go:embed describe-instance-types.json
var usEastInstanceTypes []byte
var instanceTypesOutput ec2.DescribeInstanceTypesOutput

const kwokProviderPrefix = "kwok://"

var kwokZones = []string{"zone-1", "zone-2", "zone-3"}

func init() {
	dec := json.NewDecoder(bytes.NewReader(usEastInstanceTypes))
	if err := dec.Decode(&instanceTypesOutput); err != nil {
		log.Fatalf("deserializing instance types, %s", err)
	}
}

func NewCloudProvider(client kubernetes.Interface) *CloudProvider {
	p := pricing.NewProvider(nil, nil, nil, "")
	return &CloudProvider{
		pricing:    p,
		kubeClient: client,
	}
}

type CloudProvider struct {
	pricing       *pricing.Provider
	kubeClient    kubernetes.Interface
	populateTypes sync.Once
	instanceTypes []*cloudprovider.InstanceTypes
}

func (c CloudProvider) Create(ctx context.Context, machine *v1alpha5.Machine) (*v1alpha5.Machine, error) {
	node, err := c.toNode(machine)
	if err != nil {
		return nil, fmt.Errorf("translating machine to node, %w", err)
	}
	_, err = c.kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating node, %w", err)
	}
	return c.toMachine(node)
}

func (c CloudProvider) Delete(ctx context.Context, machine *v1alpha5.Machine) error {
	err := c.kubeClient.CoreV1().Nodes().Delete(ctx, machine.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleting node, %w", err)
	}
	return nil
}

func (c CloudProvider) Get(ctx context.Context, providerID string) (*v1alpha5.Machine, error) {
	nodeName := strings.Replace(providerID, kwokProviderPrefix, "", -1)
	node, err := c.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("finding node, %w", err)
	}
	return c.toMachine(node)
}

func (c CloudProvider) List(ctx context.Context) ([]*v1alpha5.Machine, error) {
	nodes, err := c.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes, %w", err)
	}
	var machines []*v1alpha5.Machine
	for i, node := range nodes.Items {
		if !strings.HasPrefix(node.Spec.ProviderID, kwokProviderPrefix) {
			continue
		}
		m, err := c.toMachine(&nodes.Items[i])
		if err != nil {
			return nil, fmt.Errorf("converting machine, %w", err)
		}
		machines = append(machines, m)
	}

	return machines, nil
}

func (c CloudProvider) GetInstanceTypes(ctx context.Context, provisioner *v1alpha5.Provisioner) ([]*cloudprovider.InstanceType, error) {
	var ret []*cloudprovider.InstanceType
	for _, it := range instanceTypesOutput.InstanceTypes {
		offerings := c.offerings(it)
		ret = append(ret, &cloudprovider.InstanceType{
			Name:         *it.InstanceType,
			Requirements: requirements(it, offerings),
			Offerings:    offerings,
			Capacity:     computeCapacity(ctx, it),
			Overhead: &cloudprovider.InstanceTypeOverhead{
				KubeReserved:      nil,
				SystemReserved:    nil,
				EvictionThreshold: nil,
			},
		})
	}

	return ret, nil
}

func (c CloudProvider) offerings(it *ec2.InstanceTypeInfo) cloudprovider.Offerings {
	var ret cloudprovider.Offerings
	for _, zone := range kwokZones {
		if odPrice, ok := c.pricing.OnDemandPrice(*it.InstanceType); ok {
			ret = append(ret, cloudprovider.Offering{
				CapacityType: v1alpha5.CapacityTypeOnDemand,
				Zone:         zone,
				Price:        odPrice,
				Available:    true,
			})
			// just supply a 50% discount for spot since we don't have static spot pricing
			ret = append(ret, cloudprovider.Offering{
				CapacityType: v1alpha5.CapacityTypeSpot,
				Zone:         zone,
				Price:        odPrice * 0.5,
				Available:    true,
			})
		}
	}
	return ret
}

func (c CloudProvider) IsMachineDrifted(ctx context.Context, machine *v1alpha5.Machine) (bool, error) {
	return false, nil
}

func (c CloudProvider) Name() string {
	return "kwok-provider"
}

func (c CloudProvider) toNode(machine *v1alpha5.Machine) (*v1.Node, error) {
	newName := strings.Replace(namesgenerator.GetRandomName(0), "_", "-", -1)

	var instanceTypeName string
	var instanceTypePrice float64
	for _, req := range machine.Spec.Requirements {
		if req.Key == v1.LabelInstanceTypeStable {
			// pick the cheapest OD instance type
			instanceTypeName = req.Values[0]
			instanceTypePrice, _ = c.pricing.OnDemandPrice(instanceTypeName)
			for _, it := range req.Values {
				if price, ok := c.pricing.OnDemandPrice(it); ok && price < instanceTypePrice {
					instanceTypePrice = price
					instanceTypeName = it
				}
			}
		}
	}

	its, err := c.GetInstanceTypes(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("listing instance types, %w", err)
	}
	var instanceType *cloudprovider.InstanceType
	for _, it := range its {
		if it.Name == instanceTypeName {
			instanceType = it
			break
		}
	}

	if instanceType == nil {
		return nil, fmt.Errorf("unable to find instance type %q", instanceTypeName)
	}

	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        newName,
			Labels:      addInstanceLabels(machine.Labels, instanceType, machine),
			Annotations: addKwokAnnotation(machine.Annotations),
		},
		Spec: v1.NodeSpec{
			ProviderID: kwokProviderPrefix + newName,
		},
		Status: v1.NodeStatus{
			Capacity:    instanceType.Capacity,
			Allocatable: instanceType.Allocatable(),
			Phase:       v1.NodePending,
		},
	}, nil
}

func addInstanceLabels(labels map[string]string, instanceType *cloudprovider.InstanceType, machine *v1alpha5.Machine) map[string]string {
	ret := make(map[string]string, len(labels))
	// start with labels on the machine
	for k, v := range labels {
		ret[k] = v
	}

	// add the derived machine requirement labels
	for _, r := range machine.Spec.Requirements {
		if len(r.Values) == 1 && r.Operator == v1.NodeSelectorOpIn {
			ret[r.Key] = r.Values[0]
		}
	}

	// ensure we have an instance type and then any instance type requiremnets
	ret[v1.LabelInstanceTypeStable] = instanceType.Name
	for _, r := range instanceType.Requirements {
		if r.Len() == 1 && r.Operator() == v1.NodeSelectorOpIn {
			ret[r.Key] = r.Values()[0]
		}
	}

	// no zone set by requirements, so just pick one
	if _, ok := ret[v1.LabelTopologyZone]; !ok {
		ret[v1.LabelTopologyZone] = randomChoice(kwokZones)
	}

	ret["kwok.x-k8s.io/node"] = "fake"
	return ret
}

func randomChoice(zones []string) string {
	i := rand.Intn(len(zones))
	return zones[i]
}

func addKwokAnnotation(annotations map[string]string) map[string]string {
	ret := make(map[string]string, len(annotations)+1)
	for k, v := range annotations {
		ret[k] = v
	}
	ret["kwok.x-k8s.io/node"] = "fake"
	return ret
}

func (c CloudProvider) toMachine(node *v1.Node) (*v1alpha5.Machine, error) {
	return &v1alpha5.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:        node.Name,
			Labels:      node.Labels,
			Annotations: addKwokAnnotation(node.Annotations),
		},
		Spec: v1alpha5.MachineSpec{
			Taints:             nil,
			StartupTaints:      nil,
			Requirements:       nil,
			Resources:          v1alpha5.ResourceRequirements{},
			Kubelet:            nil,
			MachineTemplateRef: nil,
		},
		Status: v1alpha5.MachineStatus{
			NodeName:    node.Name,
			ProviderID:  node.Spec.ProviderID,
			Capacity:    node.Status.Capacity,
			Allocatable: node.Status.Allocatable,
		},
	}, nil
}
