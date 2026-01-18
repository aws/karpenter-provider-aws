package priorityqueue

import (
	"sync"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/internal/metrics"
)

func newFakeMetricsProvider() *fakeMetricsProvider {
	return &fakeMetricsProvider{
		depth:                   make(map[string]map[int]int),
		adds:                    make(map[string]int),
		latency:                 make(map[string][]float64),
		workDuration:            make(map[string][]float64),
		unfinishedWorkSeconds:   make(map[string]float64),
		longestRunningProcessor: make(map[string]float64),
		retries:                 make(map[string]int),
		mu:                      sync.Mutex{},
	}
}

var _ metrics.MetricsProviderWithPriority = &fakeMetricsProvider{}

type fakeMetricsProvider struct {
	depth                   map[string]map[int]int
	adds                    map[string]int
	latency                 map[string][]float64
	workDuration            map[string][]float64
	unfinishedWorkSeconds   map[string]float64
	longestRunningProcessor map[string]float64
	retries                 map[string]int
	mu                      sync.Mutex
}

func (f *fakeMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	panic("Should never be called. Expected NewDepthMetricWithPriority to be called instead")
}

func (f *fakeMetricsProvider) NewDepthMetricWithPriority(name string) metrics.DepthMetricWithPriority {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.depth[name] = map[int]int{}
	return &fakeGaugeMetric{m: &f.depth, mu: &f.mu, name: name}
}

func (f *fakeMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adds[name] = 0
	return &fakeCounterMetric{m: &f.adds, mu: &f.mu, name: name}
}

func (f *fakeMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.latency[name] = []float64{}
	return &fakeHistogramMetric{m: &f.latency, mu: &f.mu, name: name}
}

func (f *fakeMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workDuration[name] = []float64{}
	return &fakeHistogramMetric{m: &f.workDuration, mu: &f.mu, name: name}
}

func (f *fakeMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unfinishedWorkSeconds[name] = 0
	return &fakeSettableGaugeMetric{m: &f.unfinishedWorkSeconds, mu: &f.mu, name: name}
}

func (f *fakeMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.longestRunningProcessor[name] = 0
	return &fakeSettableGaugeMetric{m: &f.longestRunningProcessor, mu: &f.mu, name: name}
}

func (f *fakeMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retries[name] = 0
	return &fakeCounterMetric{m: &f.retries, mu: &f.mu, name: name}
}

type fakeGaugeMetric struct {
	m    *map[string]map[int]int
	mu   *sync.Mutex
	name string
}

func (fg *fakeGaugeMetric) Inc(priority int) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	(*fg.m)[fg.name][priority]++
}

func (fg *fakeGaugeMetric) Dec(priority int) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	(*fg.m)[fg.name][priority]--
}

type fakeCounterMetric struct {
	m    *map[string]int
	mu   *sync.Mutex
	name string
}

func (fc *fakeCounterMetric) Inc() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	(*fc.m)[fc.name]++
}

type fakeHistogramMetric struct {
	m    *map[string][]float64
	mu   *sync.Mutex
	name string
}

func (fh *fakeHistogramMetric) Observe(v float64) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	(*fh.m)[fh.name] = append((*fh.m)[fh.name], v)
}

type fakeSettableGaugeMetric struct {
	m    *map[string]float64
	mu   *sync.Mutex
	name string
}

func (fs *fakeSettableGaugeMetric) Set(v float64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	(*fs.m)[fs.name] = v
}
