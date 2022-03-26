//go:build test_performance
// +build test_performance

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

package scheduling_test

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/test"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSchedulingPerformance(t *testing.T) {
	tw := tabwriter.NewWriter(os.Stdout, 8, 8, 2, ' ', 0)

	cpuf, err := os.Create("schedule.cpuprofile")
	if err != nil {
		t.Fatalf("error creating CPU profile: %s", err)
	}
	pprof.StartCPUProfile(cpuf)
	defer pprof.StopCPUProfile()

	totalPods := 0
	totalNodes := 0
	var totalTime time.Duration
	for _, instanceCount := range []int{400} {
		for _, podCount := range []int{10, 100, 500, 1000, 1500, 2000, 2500} {
			start := time.Now()
			res := testing.Benchmark(func(b *testing.B) { benchmarkScheduler(b, instanceCount, podCount) })
			totalTime += time.Since(start) / time.Duration(res.N)
			nodeCount := res.Extra["nodes"]
			fmt.Fprintf(tw, "%d instances %d pods\t%d nodes\t%s per scheduling\t%s per pod\n", instanceCount, podCount, int(nodeCount), time.Duration(res.NsPerOp()), time.Duration(res.NsPerOp()/int64(podCount)))
			totalPods += podCount
			totalNodes += int(nodeCount)
		}
	}
	fmt.Println("scheduled", totalPods, "against", totalNodes, "nodes in total in", totalTime, float64(totalPods)/totalTime.Seconds(), "pods/sec")
	tw.Flush()
}

func benchmarkScheduler(b *testing.B, instanceCount, podCount int) {
	// Setup Mocks
	ctx := context.Background()
	// disable logging
	ctx = logging.WithLogger(ctx, zap.NewNop().Sugar())
	instanceTypes := fake.InstanceTypes(instanceCount)
	var instanceTypeNames []string
	for _, it := range instanceTypes {
		instanceTypeNames = append(instanceTypeNames, it.Name())
	}
	cloudProvider := &fake.CloudProvider{InstanceTypes: instanceTypes}

	provisioner = &v1alpha5.Provisioner{
		ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
		Spec:       v1alpha5.ProvisionerSpec{},
	}

	kubeClient := testclient.NewClientBuilder().WithLists(&appsv1.DaemonSetList{}).Build()
	provisioners = provisioning.NewController(ctx, kubeClient, nil, cloudProvider)
	provisioners.Apply(ctx, provisioner)
	scheduler := scheduling.NewScheduler(kubeClient)

	pods := test.Pods(podCount, test.PodOptions{
		ResourceRequirements: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 1)),
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", 512)),
			},
		},
	})

	b.ResetTimer()
	// Pack benchmark
	for i := 0; i < b.N; i++ {
		nodes, err := scheduler.Solve(ctx, provisioner, instanceTypes, pods)
		if err != nil || len(nodes) == 0 {
			b.FailNow()
		}

		podCount := 0
		for _, n := range nodes {
			podCount += len(n.Pods)
		}
		if podCount != len(pods) {
			b.Fatalf("expected %d scheduled pods, got %d", len(pods), podCount)
		}
		b.ReportMetric(float64(len(nodes)), "nodes")
	}

}
