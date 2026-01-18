// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"fmt"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

// uncheckedCollector wraps a Collector but its Describe method yields no Desc.
type uncheckedCollector struct {
	c Collector
}

func (u uncheckedCollector) Describe(_ chan<- *Desc) {}
func (u uncheckedCollector) Collect(c chan<- Metric) {
	u.c.Collect(c)
}

func toMetricFamilies(cs ...Collector) []*dto.MetricFamily {
	reg := NewRegistry()
	reg.MustRegister(cs...)
	out, err := reg.Gather()
	if err != nil {
		panic(err)
	}
	return out
}

func TestWrapRegisterer(t *testing.T) {
	now := time.Now()
	nowFn := func() time.Time { return now }
	simpleCnt := NewCounter(CounterOpts{
		Name: "simpleCnt",
		Help: "helpSimpleCnt",
		now:  nowFn,
	})
	simpleCnt.Inc()

	simpleGge := NewGauge(GaugeOpts{
		Name: "simpleGge",
		Help: "helpSimpleGge",
	})
	simpleGge.Set(3.14)

	preCnt := NewCounter(CounterOpts{
		Name: "pre_simpleCnt",
		Help: "helpSimpleCnt",
		now:  nowFn,
	})
	preCnt.Inc()

	barLabeledCnt := NewCounter(CounterOpts{
		Name:        "simpleCnt",
		Help:        "helpSimpleCnt",
		ConstLabels: Labels{"foo": "bar"},
		now:         nowFn,
	})
	barLabeledCnt.Inc()

	bazLabeledCnt := NewCounter(CounterOpts{
		Name:        "simpleCnt",
		Help:        "helpSimpleCnt",
		ConstLabels: Labels{"foo": "baz"},
		now:         nowFn,
	})
	bazLabeledCnt.Inc()

	labeledPreCnt := NewCounter(CounterOpts{
		Name:        "pre_simpleCnt",
		Help:        "helpSimpleCnt",
		ConstLabels: Labels{"foo": "bar"},
		now:         nowFn,
	})
	labeledPreCnt.Inc()

	twiceLabeledPreCnt := NewCounter(CounterOpts{
		Name:        "pre_simpleCnt",
		Help:        "helpSimpleCnt",
		ConstLabels: Labels{"foo": "bar", "dings": "bums"},
		now:         nowFn,
	})
	twiceLabeledPreCnt.Inc()

	barLabeledUncheckedCollector := uncheckedCollector{barLabeledCnt}

	scenarios := map[string]struct {
		prefix      string // First wrap with this prefix.
		labels      Labels // Then wrap the result with these labels.
		labels2     Labels // If any, wrap the prefix-wrapped one again.
		preRegister []Collector
		toRegister  []struct { // If there are any labels2, register every other with that one.
			collector         Collector
			registrationFails bool
		}
		gatherFails bool
		output      []Collector
	}{
		"wrap nothing": {
			prefix: "pre_",
			labels: Labels{"foo": "bar"},
		},
		"wrap with nothing": {
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, false}},
			output: []Collector{simpleGge, simpleCnt},
		},
		"wrap counter with prefix": {
			prefix:      "pre_",
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, false}},
			output: []Collector{simpleGge, preCnt},
		},
		"wrap counter with label pair": {
			labels:      Labels{"foo": "bar"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, false}},
			output: []Collector{simpleGge, barLabeledCnt},
		},
		"wrap counter with label pair and prefix": {
			prefix:      "pre_",
			labels:      Labels{"foo": "bar"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, false}},
			output: []Collector{simpleGge, labeledPreCnt},
		},
		"wrap counter with invalid prefix": {
			prefix:      "1\x801",
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, true}},
			output: []Collector{simpleGge},
		},
		"wrap counter with invalid label": {
			preRegister: []Collector{simpleGge},
			labels:      Labels{"\x80": "bar"},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, true}},
			output: []Collector{simpleGge},
		},
		"counter registered twice but wrapped with different label values": {
			labels:  Labels{"foo": "bar"},
			labels2: Labels{"foo": "baz"},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, false}, {simpleCnt, false}},
			output: []Collector{barLabeledCnt, bazLabeledCnt},
		},
		"counter registered twice but wrapped with different inconsistent label values": {
			labels:  Labels{"foo": "bar"},
			labels2: Labels{"bar": "baz"},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, false}, {simpleCnt, true}},
			output: []Collector{barLabeledCnt},
		},
		"wrap counter with prefix and two labels": {
			prefix:      "pre_",
			labels:      Labels{"foo": "bar", "dings": "bums"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{simpleCnt, false}},
			output: []Collector{simpleGge, twiceLabeledPreCnt},
		},
		"wrap labeled counter with prefix and another label": {
			prefix:      "pre_",
			labels:      Labels{"dings": "bums"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{barLabeledCnt, false}},
			output: []Collector{simpleGge, twiceLabeledPreCnt},
		},
		"wrap labeled counter with prefix and inconsistent label": {
			prefix:      "pre_",
			labels:      Labels{"foo": "bums"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{barLabeledCnt, true}},
			output: []Collector{simpleGge},
		},
		"wrap labeled counter with prefix and the same label again": {
			prefix:      "pre_",
			labels:      Labels{"foo": "bar"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{barLabeledCnt, true}},
			output: []Collector{simpleGge},
		},
		"wrap labeled unchecked collector with prefix and another label": {
			prefix:      "pre_",
			labels:      Labels{"dings": "bums"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{barLabeledUncheckedCollector, false}},
			output: []Collector{simpleGge, twiceLabeledPreCnt},
		},
		"wrap labeled unchecked collector with prefix and inconsistent label": {
			prefix:      "pre_",
			labels:      Labels{"foo": "bums"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{barLabeledUncheckedCollector, false}},
			gatherFails: true,
			output:      []Collector{simpleGge},
		},
		"wrap labeled unchecked collector with prefix and the same label again": {
			prefix:      "pre_",
			labels:      Labels{"foo": "bar"},
			preRegister: []Collector{simpleGge},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{barLabeledUncheckedCollector, false}},
			gatherFails: true,
			output:      []Collector{simpleGge},
		},
		"wrap labeled unchecked collector with prefix and another label resulting in collision with pre-registered counter": {
			prefix:      "pre_",
			labels:      Labels{"dings": "bums"},
			preRegister: []Collector{twiceLabeledPreCnt},
			toRegister: []struct {
				collector         Collector
				registrationFails bool
			}{{barLabeledUncheckedCollector, false}},
			gatherFails: true,
			output:      []Collector{twiceLabeledPreCnt},
		},
	}

	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			reg := NewPedanticRegistry()
			for _, c := range s.preRegister {
				if err := reg.Register(c); err != nil {
					t.Fatal("error registering with unwrapped registry:", err)
				}
			}
			preReg := WrapRegistererWithPrefix(s.prefix, reg)
			lReg := WrapRegistererWith(s.labels, preReg)
			l2Reg := WrapRegistererWith(s.labels2, preReg)
			for i, tr := range s.toRegister {
				var err error
				if i%2 != 0 && len(s.labels2) != 0 {
					err = l2Reg.Register(tr.collector)
				} else {
					err = lReg.Register(tr.collector)
				}
				if tr.registrationFails && err == nil {
					t.Fatalf("registration with wrapping registry unexpectedly succeeded for collector #%d", i)
				}
				if !tr.registrationFails && err != nil {
					t.Fatalf("registration with wrapping registry failed for collector #%d: %s", i, err)
				}
			}
			wantMF := toMetricFamilies(s.output...)
			gotMF, err := reg.Gather()
			if s.gatherFails && err == nil {
				t.Fatal("gathering unexpectedly succeeded")
			}
			if !s.gatherFails && err != nil {
				t.Fatal("gathering failed:", err)
			}
			assertEqualMFs(t, wantMF, gotMF)
		})
	}
}

func assertEqualMFs(t *testing.T, wantMF, gotMF []*dto.MetricFamily) {
	t.Helper()

	if len(wantMF) != len(gotMF) {
		t.Fatalf("Expected %d metricFamilies, got %d", len(wantMF), len(gotMF))
	}
	for i := range gotMF {
		if !proto.Equal(gotMF[i], wantMF[i]) {
			var want, got []string

			for i, mf := range wantMF {
				want = append(want, fmt.Sprintf("%3d: %s", i, mf))
			}
			for i, mf := range gotMF {
				got = append(got, fmt.Sprintf("%3d: %s", i, mf))
			}

			t.Fatalf(
				"unexpected output of gathering:\n\nWANT:\n%s\n\nGOT:\n%s\n",
				strings.Join(want, "\n"),
				strings.Join(got, "\n"),
			)
		}
	}
}

func TestNil(t *testing.T) {
	// A wrapped nil registerer should be treated as a no-op, and not panic.
	c := NewCounter(CounterOpts{Name: "test"})
	err := WrapRegistererWith(Labels{"foo": "bar"}, nil).Register(c)
	if err != nil {
		t.Fatal("registering failed:", err)
	}
}

func TestWrapCollector(t *testing.T) {
	t.Run("can be registered and un-registered", func(t *testing.T) {
		inner := NewPedanticRegistry()
		g := NewGauge(GaugeOpts{Name: "testing"})
		g.Set(42)
		err := inner.Register(g)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		wrappedWithLabels := WrapCollectorWith(Labels{"lbl": "1"}, inner)
		wrappedWithPrefix := WrapCollectorWithPrefix("prefix", inner)
		reg := NewPedanticRegistry()
		err = reg.Register(wrappedWithLabels)
		if err != nil {
			t.Fatal("registering failed:", err)
		}
		err = reg.Register(wrappedWithPrefix)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		gathered, err := reg.Gather()
		if err != nil {
			t.Fatal("gathering failed:", err)
		}

		lg := NewGauge(GaugeOpts{Name: "testing", ConstLabels: Labels{"lbl": "1"}})
		lg.Set(42)
		pg := NewGauge(GaugeOpts{Name: "prefixtesting"})
		pg.Set(42)
		expected := toMetricFamilies(lg, pg)
		assertEqualMFs(t, expected, gathered)

		if !reg.Unregister(wrappedWithLabels) {
			t.Fatal("unregistering failed")
		}
		if !reg.Unregister(wrappedWithPrefix) {
			t.Fatal("unregistering failed")
		}

		gathered, err = reg.Gather()
		if err != nil {
			t.Fatal("gathering failed:", err)
		}
		if len(gathered) != 0 {
			t.Fatalf("expected 0 metric families, got %d", len(gathered))
		}
	})

	t.Run("can wrap same collector twice", func(t *testing.T) {
		inner := NewPedanticRegistry()
		g := NewGauge(GaugeOpts{Name: "testing"})
		g.Set(42)
		err := inner.Register(g)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		wrapped := WrapCollectorWith(Labels{"lbl": "1"}, inner)
		reg := NewPedanticRegistry()
		err = reg.Register(wrapped)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		wrapped2 := WrapCollectorWith(Labels{"lbl": "2"}, inner)
		err = reg.Register(wrapped2)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		gathered, err := reg.Gather()
		if err != nil {
			t.Fatal("gathering failed:", err)
		}

		lg := NewGauge(GaugeOpts{Name: "testing", ConstLabels: Labels{"lbl": "1"}})
		lg.Set(42)
		lg2 := NewGauge(GaugeOpts{Name: "testing", ConstLabels: Labels{"lbl": "2"}})
		lg2.Set(42)
		expected := toMetricFamilies(lg, lg2)
		assertEqualMFs(t, expected, gathered)
	})

	t.Run("can be registered again after un-registering", func(t *testing.T) {
		inner := NewPedanticRegistry()
		g := NewGauge(GaugeOpts{Name: "testing"})
		g.Set(42)
		err := inner.Register(g)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		wrapped := WrapCollectorWith(Labels{"lbl": "1"}, inner)
		reg := NewPedanticRegistry()
		err = reg.Register(wrapped)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		if !reg.Unregister(wrapped) {
			t.Fatal("unregistering failed")
		}
		err = reg.Register(wrapped)
		if err != nil {
			t.Fatal("registering failed:", err)
		}

		gathered, err := reg.Gather()
		if err != nil {
			t.Fatal("gathering failed:", err)
		}

		lg := NewGauge(GaugeOpts{Name: "testing", ConstLabels: Labels{"lbl": "1"}})
		lg.Set(42)
		expected := toMetricFamilies(lg)
		assertEqualMFs(t, expected, gathered)
	})
}
