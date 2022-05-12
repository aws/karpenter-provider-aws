//go:build test_performance

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
	"github.com/aws/karpenter/pkg/test"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
	"math/rand"
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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const MinPodsPerSec = 100.0
const PrintStats = false

var r = rand.New(rand.NewSource(42))

func BenchmarkScheduling1(b *testing.B) {
	benchmarkScheduler(b, 400, 1)
}
func BenchmarkScheduling50(b *testing.B) {
	benchmarkScheduler(b, 400, 50)
}
func BenchmarkScheduling100(b *testing.B) {
	benchmarkScheduler(b, 400, 100)
}
func BenchmarkScheduling500(b *testing.B) {
	benchmarkScheduler(b, 400, 500)
}
func BenchmarkScheduling1000(b *testing.B) {
	benchmarkScheduler(b, 400, 1000)
}
func BenchmarkScheduling2000(b *testing.B) {
	benchmarkScheduler(b, 400, 2000)
}
func BenchmarkScheduling5000(b *testing.B) {
	benchmarkScheduler(b, 400, 5000)
}

// TestSchedulingProfile is used to gather profiling metrics, benchmarking is primarily done with standard
// Go benchmark functions
// go test -tags=test_performance -run=SchedulingProfile
func TestSchedulingProfile(t *testing.T) {
	tw := tabwriter.NewWriter(os.Stdout, 8, 8, 2, ' ', 0)

	cpuf, err := os.Create("schedule.cpuprofile")
	if err != nil {
		t.Fatalf("error creating CPU profile: %s", err)
	}
	pprof.StartCPUProfile(cpuf)
	defer pprof.StopCPUProfile()

	heapf, err := os.Create("schedule.heapprofile")
	if err != nil {
		t.Fatalf("error creating heap profile: %s", err)
	}
	defer pprof.WriteHeapProfile(heapf)

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
	provisioners = provisioning.NewController(ctx, kubeClient, nil, recorder, cloudProvider)
	provisioners.Apply(ctx, provisioner)
	scheduler := scheduling.NewScheduler(kubeClient, recorder)

	pods := makeDiversePods(podCount)

	b.ResetTimer()
	// Pack benchmark
	start := time.Now()
	podsScheduledInRound1 := 0
	nodesInRound1 := 0
	for i := 0; i < b.N; i++ {
		nodes, err := scheduler.Solve(ctx, &provisioner.Spec.Constraints, instanceTypes, pods)
		if err != nil {
			b.FailNow()
		}
		if i == 0 {

			minPods := math.MaxInt64
			maxPods := 0
			var podCounts []int
			for _, n := range nodes {
				podCounts = append(podCounts, len(n.Pods))
				podsScheduledInRound1 += len(n.Pods)
				nodesInRound1 = len(nodes)
				if len(n.Pods) > maxPods {
					maxPods = len(n.Pods)
				}
				if len(n.Pods) < minPods {
					minPods = len(n.Pods)
				}
			}
			if PrintStats {
				meanPodsPerNode := float64(podsScheduledInRound1) / float64(nodesInRound1)
				variance := 0.0
				for _, pc := range podCounts {
					variance += math.Pow(float64(pc)-meanPodsPerNode, 2.0)
				}
				variance /= float64(nodesInRound1)
				stddev := math.Sqrt(variance)
				fmt.Printf("%d instance types %d pods resulted in %d nodes with pods per node min=%d max=%d mean=%f stddev=%f\n",
					instanceCount, podCount, nodesInRound1, minPods, maxPods, meanPodsPerNode, stddev)
			}
		}
	}
	duration := time.Since(start)
	podsPerSec := float64(len(pods)) / (duration.Seconds() / float64(b.N))
	b.ReportMetric(podsPerSec, "pods/sec")
	b.ReportMetric(float64(podsScheduledInRound1), "pods")
	b.ReportMetric(float64(nodesInRound1), "nodes")

	// we don't care if it takes a bit of time to schedule a few pods as there is some setup time required for sorting
	// instance types, computing topologies, etc.  We want to ensure that the larger batches of pods don't become too
	// slow.
	if len(pods) > 100 {
		if podsPerSec < MinPodsPerSec {
			b.Fatalf("scheduled %f pods/sec, expected at least %f", podsPerSec, MinPodsPerSec)
		}
	}
}

func makeDiversePods(count int) []*v1.Pod {
	var pods []*v1.Pod
	pods = append(pods, makeGenericPods(count/7)...)
	pods = append(pods, makeTopologySpreadPods(count/7, v1.LabelTopologyZone)...)
	pods = append(pods, makeTopologySpreadPods(count/7, v1.LabelHostname)...)
	pods = append(pods, makePodAffinityPods(count/7, v1.LabelHostname)...)
	pods = append(pods, makePodAffinityPods(count/7, v1.LabelTopologyZone)...)
	// We intentionally don't do anti-affinity by zone as that creates tons of unschedulable pods.
	//pods = append(pods, makePodAntiAffinityPods(count/7, v1.LabelTopologyZone)...)

	// fill out due to count being not evenly divisible with generic pods
	nRemaining := count - len(pods)
	pods = append(pods, makeGenericPods(nRemaining)...)
	return pods
}

func makePodAntiAffinityPods(count int, key string) []*v1.Pod {
	var pods []*v1.Pod
	for i := 0; i < count; i++ {
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: randomAntiAffinityLabels()},
				PodAntiRequirements: []v1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: randomAntiAffinityLabels()},
						TopologyKey:   key,
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    randomCpu(),
						v1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}
func makePodAffinityPods(count int, key string) []*v1.Pod {
	var pods []*v1.Pod
	for i := 0; i < count; i++ {
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: randomAffinityLabels()},
				PodRequirements: []v1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: randomAffinityLabels()},
						TopologyKey:   key,
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    randomCpu(),
						v1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}

func makeTopologySpreadPods(count int, key string) []*v1.Pod {
	var pods []*v1.Pod
	for i := 0; i < count; i++ {
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: randomLabels()},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       key,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: randomLabels(),
						},
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    randomCpu(),
						v1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}

func makeGenericPods(count int) []*v1.Pod {
	var pods []*v1.Pod
	for i := 0; i < count; i++ {
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: randomLabels()},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    randomCpu(),
						v1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}

func randomAffinityLabels() map[string]string {
	return map[string]string{
		"my-affininity": randomLabelValue(),
	}
}
func randomAntiAffinityLabels() map[string]string {
	return map[string]string{
		"my-anti-affininity": randomLabelValue(),
	}
}
func randomLabels() map[string]string {
	return map[string]string{
		"my-label": randomLabelValue(),
	}
}

func randomLabelValue() string {
	labelValues := []string{"a", "b", "c", "d", "e", "f", "g"}
	return labelValues[r.Intn(len(labelValues))]
}

func randomMemory() resource.Quantity {
	mem := []int{100, 256, 512, 1024, 2048, 4096}
	return resource.MustParse(fmt.Sprintf("%dMi", mem[r.Intn(len(mem))]))
}

func randomCpu() resource.Quantity {
	cpu := []int{100, 250, 500, 1000, 1500}
	return resource.MustParse(fmt.Sprintf("%dm", cpu[r.Intn(len(cpu))]))
}
