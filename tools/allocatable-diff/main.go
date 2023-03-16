package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	awscontext "github.com/aws/karpenter/pkg/context"
)

var clusterName string
var outFile string

func init() {
	flag.StringVar(&clusterName, "cluster-name", "", "cluster name to use when passing subnets into GetInstanceTypes()")
	flag.StringVar(&outFile, "out-file", "allocatable-diff.csv", "file to output the generated data")
	flag.Parse()
}

func main() {
	if clusterName == "" {
		log.Fatalf("cluster name cannot be empty")
	}
	restConfig := config.GetConfigOrDie()
	kubeClient := lo.Must(client.New(restConfig, client.Options{}))
	kubernetesInterface := kubernetes.NewForConfigOrDie(restConfig)
	ctx := context.Background()
	ctx = settings.ToContext(ctx, &settings.Settings{ClusterName: clusterName, IsolatedVPC: true, VMMemoryOverheadPercent: 0})

	file := lo.Must(os.OpenFile(outFile, os.O_RDWR|os.O_CREATE, 0777))
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	nodeList := &v1.NodeList{}
	lo.Must0(kubeClient.List(ctx, nodeList))

	awsCtx := awscontext.NewOrDie(corecloudprovider.Context{
		Context:             ctx,
		Clock:               clock.RealClock{},
		RESTConfig:          restConfig,
		KubeClient:          kubeClient,
		KubernetesInterface: kubernetesInterface,
	})
	cloudProvider := cloudprovider.New(
		awsCtx,
		awsCtx.InstanceTypesProvider,
		awsCtx.InstanceProvider,
		awsCtx.KubeClient,
		awsCtx.AMIProvider,
	)
	raw := &runtime.RawExtension{}
	lo.Must0(raw.UnmarshalJSON(lo.Must(json.Marshal(&v1alpha1.AWS{
		SubnetSelector: map[string]string{
			"karpenter.sh/discovery": clusterName,
		},
	}))))
	instanceTypes := lo.Must(cloudProvider.GetInstanceTypes(ctx, &v1alpha5.Provisioner{
		Spec: v1alpha5.ProvisionerSpec{
			Provider: raw,
		},
	}))

	// Write the header information into the CSV
	lo.Must0(w.Write([]string{"Instance Type", "Expected Capacity", "", "", "Expected Allocatable", "", "", "Actual Capacity", "", "", "Actual Allocatable", ""}))
	lo.Must0(w.Write([]string{"", "Memory (Mi)", "CPU (m)", "Storage (Mi)", "Memory (Mi)", "CPU (m)", "Storage (Mi)", "Memory (Mi)", "CPU (m)", "Storage (Mi)", "Memory (Mi)", "CPU (m)", "Storage (Mi)"}))

	nodeList.Items = lo.Filter(nodeList.Items, func(n v1.Node, _ int) bool {
		return n.Labels["karpenter.sh/provisioner-name"] != "" && n.Status.Allocatable.Memory().Value() != 0
	})
	sort.Slice(nodeList.Items, func(i, j int) bool {
		return nodeList.Items[i].Labels[v1.LabelInstanceTypeStable] < nodeList.Items[j].Labels[v1.LabelInstanceTypeStable]
	})
	for _, node := range nodeList.Items {
		instanceType, ok := lo.Find(instanceTypes, func(i *corecloudprovider.InstanceType) bool {
			return i.Name == node.Labels[v1.LabelInstanceTypeStable]
		})
		if !ok {
			log.Fatalf("retrieving instance type for instance %s", node.Labels[v1.LabelInstanceTypeStable])
		}
		allocatable := instanceType.Allocatable()

		// Write the details of the expected instance and the actual instance into a CSV line format
		lo.Must0(w.Write([]string{
			instanceType.Name,
			fmt.Sprintf("%d", instanceType.Capacity.Memory().ScaledValue(resource.Mega)),
			fmt.Sprintf("%d", instanceType.Capacity.Cpu().MilliValue()),
			fmt.Sprintf("%d", instanceType.Capacity.StorageEphemeral().ScaledValue(resource.Mega)),
			fmt.Sprintf("%d", allocatable.Memory().ScaledValue(resource.Mega)),
			fmt.Sprintf("%d", allocatable.Cpu().MilliValue()),
			fmt.Sprintf("%d", allocatable.StorageEphemeral().ScaledValue(resource.Mega)),
			fmt.Sprintf("%d", node.Status.Capacity.Memory().ScaledValue(resource.Mega)),
			fmt.Sprintf("%d", node.Status.Capacity.Cpu().MilliValue()),
			fmt.Sprintf("%d", node.Status.Capacity.StorageEphemeral().ScaledValue(resource.Mega)),
			fmt.Sprintf("%d", node.Status.Allocatable.Memory().ScaledValue(resource.Mega)),
			fmt.Sprintf("%d", node.Status.Allocatable.Cpu().MilliValue()),
			fmt.Sprintf("%d", node.Status.Allocatable.StorageEphemeral().ScaledValue(resource.Mega)),
		}))
	}
}
