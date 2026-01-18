//go:build test_performance

/*
Copyright The Kubernetes Authors.

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
	"math"
	"math/rand"
	"os"
	"runtime/pprof"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	fakecr "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/operator/logging"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
)

func init() {
	log.SetLogger(logging.NopLogger)
}

const MinPodsPerSec = 100.0
const PrintStats = false

//nolint:gosec
var r = rand.New(rand.NewSource(42))

// To run the benchmarks use:
// `go test -tags=test_performance -run=XXX -bench=.`
//
// to get something statistically significant for comparison we need to run them several times and then
// compare the results between the old performance and the new performance.
// ```sh
//
//	go test -tags=test_performance -run=XXX -bench=. -count=10 | tee /tmp/old
//	# make your changes to the code
//	go test -tags=test_performance -run=XXX -bench=. -count=10 | tee /tmp/new
//	benchstat /tmp/old /tmp/new
//
// ```
func BenchmarkScheduling1(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(1))
}
func BenchmarkScheduling50(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(50))
}
func BenchmarkScheduling100(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(100))
}
func BenchmarkScheduling500(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(500))
}
func BenchmarkScheduling1000(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(1000))
}
func BenchmarkScheduling2000(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(2000))
}
func BenchmarkScheduling5000(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(5000))
}
func BenchmarkScheduling10000(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(10000))
}
func BenchmarkScheduling20000(b *testing.B) {
	benchmarkScheduler(b, makeDiversePods(20000))
}
func BenchmarkRespectPreferences(b *testing.B) {
	benchmarkScheduler(b, makePreferencePods(4000))
}
func BenchmarkIgnorePreferences(b *testing.B) {
	benchmarkScheduler(b, makePreferencePods(4000), scheduling.IgnorePreferences)
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
	lo.Must0(pprof.StartCPUProfile(cpuf))
	defer pprof.StopCPUProfile()

	heapf, err := os.Create("schedule.heapprofile")
	if err != nil {
		t.Fatalf("error creating heap profile: %s", err)
	}
	defer func() { lo.Must0(pprof.WriteHeapProfile(heapf)) }()

	totalPods := 0
	totalNodes := 0
	var totalTime time.Duration
	fmt.Fprintf(tw, "============== Generic Pods ==============\n")
	for _, podCount := range []int{1, 50, 100, 500, 1000, 1500, 2000, 5000, 10000, 20000} {
		start := time.Now()
		res := testing.Benchmark(func(b *testing.B) {
			benchmarkScheduler(b, makeDiversePods(podCount))
		})
		totalTime += time.Since(start) / time.Duration(res.N)
		nodeCount := res.Extra["nodes"]
		fmt.Fprintf(tw, "%s\t%d pods\t%d nodes\t%s per scheduling\t%s per pod\n", fmt.Sprintf("%d Pods", podCount), podCount, int(nodeCount), time.Duration(res.NsPerOp()), time.Duration(res.NsPerOp()/int64(podCount)))
		totalPods += podCount
		totalNodes += int(nodeCount)
	}
	fmt.Fprintf(tw, "============== Preference Pods ==============\n")
	for _, opt := range []scheduling.Options{nil, scheduling.IgnorePreferences} {
		start := time.Now()
		podCount := 4000
		res := testing.Benchmark(func(b *testing.B) {
			benchmarkScheduler(b, makePreferencePods(podCount), opt)
		})
		totalTime += time.Since(start) / time.Duration(res.N)
		nodeCount := res.Extra["nodes"]
		fmt.Fprintf(tw, "%s\t%d pods\t%d nodes\t%s per scheduling\t%s per pod\n", lo.Ternary(opt == nil, "PreferencePolicy=Respect", "PreferencePolicy=Ignore"), podCount, int(nodeCount), time.Duration(res.NsPerOp()), time.Duration(res.NsPerOp()/int64(podCount)))
		totalPods += podCount
		totalNodes += int(nodeCount)
	}
	fmt.Fprintf(tw, "\nscheduled %d against %d nodes in total in %s %f pods/sec\n", totalPods, totalNodes, totalTime, float64(totalPods)/totalTime.Seconds())
	tw.Flush()
}

func benchmarkScheduler(b *testing.B, pods []*corev1.Pod, opts ...scheduling.Options) {
	ctx = options.ToContext(injection.WithControllerName(context.Background(), "provisioner"), test.Options())
	scheduler, err := setupScheduler(ctx, pods, append(opts, scheduling.NumConcurrentReconciles(5))...)
	if err != nil {
		b.Fatalf("creating scheduler, %s", err)
	}

	b.ResetTimer()
	// Pack benchmark
	start := time.Now()
	podsScheduledInRound1 := 0
	nodesInRound1 := 0
	for i := 0; i < b.N; i++ {
		results, err := scheduler.Solve(ctx, pods)
		if err != nil {
			b.Fatalf("expected scheduler to schedule all pods without error, got %s", err)
		}
		if len(results.PodErrors) > 0 {
			b.Fatalf("expected all pods to schedule, got %d pods that didn't", len(results.PodErrors))
		}
		if i == 0 {
			minPods := math.MaxInt64
			maxPods := 0
			var podCounts []int
			for _, n := range results.NewNodeClaims {
				podCounts = append(podCounts, len(n.Pods))
				podsScheduledInRound1 += len(n.Pods)
				nodesInRound1 = len(results.NewNodeClaims)
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
				fmt.Printf("400 instance types %d pods resulted in %d nodes with pods per node min=%d max=%d mean=%f stddev=%f\n",
					len(pods), nodesInRound1, minPods, maxPods, meanPodsPerNode, stddev)
			}
		}
	}
	duration := time.Since(start)
	podsPerSec := float64(len(pods)) / (duration.Seconds() / float64(b.N))
	b.ReportMetric(podsPerSec, "pods/sec")
	b.ReportMetric(float64(podsScheduledInRound1), "pods")
	b.ReportMetric(float64(nodesInRound1), "nodes")
}

func setupScheduler(ctx context.Context, pods []*corev1.Pod, opts ...scheduling.Options) (*scheduling.Scheduler, error) {
	nodePool := test.NodePool(v1.NodePool{
		Spec: v1.NodePoolSpec{
			Limits: v1.Limits{
				corev1.ResourceCPU:    resource.MustParse("10000000"),
				corev1.ResourceMemory: resource.MustParse("10000000Gi"),
			},
		},
	})

	// Apply limits to both of the NodePools
	cloudProvider = fake.NewCloudProvider()
	instanceTypes := fake.InstanceTypes(400)
	cloudProvider.InstanceTypes = instanceTypes

	client := fakecr.NewFakeClient()
	clock := &clock.RealClock{}
	cluster = state.NewCluster(clock, client, cloudProvider)
	topology, err := scheduling.NewTopology(ctx, client, cluster, nil, []*v1.NodePool{nodePool}, map[string][]*cloudprovider.InstanceType{
		nodePool.Name: instanceTypes,
	}, pods, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating topology, %w", err)
	}

	return scheduling.NewScheduler(
		ctx,
		client,
		[]*v1.NodePool{nodePool},
		cluster,
		nil,
		topology,
		map[string][]*cloudprovider.InstanceType{nodePool.Name: instanceTypes},
		nil,
		events.NewRecorder(&record.FakeRecorder{}),
		clock,
		opts...,
	), nil
}

func makeDiversePods(count int) []*corev1.Pod {
	var pods []*corev1.Pod
	numTypes := 5
	pods = append(pods, makeGenericPods(count/numTypes)...)
	pods = append(pods, makeTopologySpreadPods(count/numTypes, corev1.LabelTopologyZone)...)
	pods = append(pods, makeTopologySpreadPods(count/numTypes, corev1.LabelHostname)...)
	pods = append(pods, makePodAffinityPods(count/numTypes, corev1.LabelTopologyZone)...)
	pods = append(pods, makePodAntiAffinityPods(count/numTypes, corev1.LabelHostname)...)

	// fill out due to count being not evenly divisible with generic pods
	nRemaining := count - len(pods)
	pods = append(pods, makeGenericPods(nRemaining)...)
	return pods
}

func makePodAntiAffinityPods(count int, key string) []*corev1.Pod {
	var pods []*corev1.Pod
	// all of these pods have anti-affinity to each other
	labels := map[string]string{
		"app": "nginx",
	}
	for i := 0; i < count; i++ {
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					UID:    uuid.NewUUID(), // set the UUID so the cached data is properly stored in the scheduler
				},
				PodAntiRequirements: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
						TopologyKey:   key,
					},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    randomCPU(),
						corev1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}
func makePodAffinityPods(count int, key string) []*corev1.Pod {
	var pods []*corev1.Pod
	for i := 0; i < count; i++ {
		// We use self-affinity here because using affinity that relies on other pod
		// domains doens't guarantee that all pods can schedule. In the case where you are not
		// using self-affinity and the domain doesn't exist, scheduling will fail for all pods with
		// affinities against this domain
		labels := randomAffinityLabels()
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					UID:    uuid.NewUUID(), // set the UUID so the cached data is properly stored in the scheduler
				},
				PodRequirements: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
						TopologyKey:   key,
					},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    randomCPU(),
						corev1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}

func makeTopologySpreadPods(count int, key string) []*corev1.Pod {
	var pods []*corev1.Pod
	for i := 0; i < count; i++ {
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: randomLabels(),
					UID:    uuid.NewUUID(), // set the UUID so the cached data is properly stored in the scheduler
				},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       key,
						WhenUnsatisfiable: corev1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: randomLabels(),
						},
					},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    randomCPU(),
						corev1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}

func makeGenericPods(count int) []*corev1.Pod {
	var pods []*corev1.Pod
	for i := 0; i < count; i++ {
		pods = append(pods, test.Pod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: randomLabels(),
					UID:    uuid.NewUUID(), // set the UUID so the cached data is properly stored in the scheduler
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    randomCPU(),
						corev1.ResourceMemory: randomMemory(),
					},
				}}))
	}
	return pods
}

func makePreferencePods(count int) []*corev1.Pod {
	pods := test.Pods(count, test.PodOptions{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "nginx",
			},
		},
		NodePreferences: []corev1.NodeSelectorRequirement{
			// This is a preference that can be satisfied
			{
				Key:      corev1.LabelTopologyZone,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"test-zone-1"},
			},
		},
		PodAntiPreferences: []corev1.WeightedPodAffinityTerm{
			// This is a preference that can't be satisfied
			{
				Weight: 10,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						"app": "nginx",
					}},
					TopologyKey: corev1.LabelTopologyZone,
				},
			},
			// This is a preference that can be satisfied
			{
				Weight: 1,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						"app": "nginx",
					}},
					TopologyKey: corev1.LabelHostname,
				},
			},
		},
		ResourceRequirements: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    randomCPU(),
				corev1.ResourceMemory: randomMemory(),
			},
		},
	})
	for _, p := range pods {
		p.UID = uuid.NewUUID() // set the UUID so the cached data is properly stored in the scheduler
	}
	return pods
}

func randomAffinityLabels() map[string]string {
	return map[string]string{
		"my-affininity": randomLabelValue(),
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

func randomCPU() resource.Quantity {
	cpu := []int{100, 250, 500, 1000, 1500}
	return resource.MustParse(fmt.Sprintf("%dm", cpu[r.Intn(len(cpu))]))
}
