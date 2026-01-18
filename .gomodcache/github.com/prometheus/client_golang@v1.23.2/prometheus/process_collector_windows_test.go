// Copyright 2019 The Prometheus Authors
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
	"bytes"
	"os"
	"regexp"
	"testing"

	"github.com/prometheus/common/expfmt"
)

func TestWindowsProcessCollector(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(NewProcessCollector(ProcessCollectorOpts{})); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(NewProcessCollector(ProcessCollectorOpts{
		PidFn:        func() (int, error) { return os.Getpid(), nil },
		Namespace:    "foobar",
		ReportErrors: true, // No errors expected, just to see if none are reported.
	})); err != nil {
		t.Fatal(err)
	}

	mfs, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	for _, mf := range mfs {
		if _, err := expfmt.MetricFamilyToText(&buf, mf); err != nil {
			t.Fatal(err)
		}
	}

	for _, re := range []*regexp.Regexp{
		regexp.MustCompile("\nprocess_cpu_seconds_total [0-9]"),
		regexp.MustCompile("\nprocess_max_fds [1-9]"),
		regexp.MustCompile("\nprocess_open_fds [1-9]"),
		regexp.MustCompile("\nprocess_virtual_memory_max_bytes (-1|[1-9])"),
		regexp.MustCompile("\nprocess_virtual_memory_bytes [1-9]"),
		regexp.MustCompile("\nprocess_resident_memory_bytes [1-9]"),
		regexp.MustCompile("\nprocess_start_time_seconds [0-9.]{10,}"),
		regexp.MustCompile("\nfoobar_process_cpu_seconds_total [0-9]"),
		regexp.MustCompile("\nfoobar_process_max_fds [1-9]"),
		regexp.MustCompile("\nfoobar_process_open_fds [1-9]"),
		regexp.MustCompile("\nfoobar_process_virtual_memory_max_bytes (-1|[1-9])"),
		regexp.MustCompile("\nfoobar_process_virtual_memory_bytes [1-9]"),
		regexp.MustCompile("\nfoobar_process_resident_memory_bytes [1-9]"),
		regexp.MustCompile("\nfoobar_process_start_time_seconds [0-9.]{10,}"),
	} {
		if !re.Match(buf.Bytes()) {
			t.Errorf("want body to match %s\n%s", re, buf.String())
		}
	}
}

func TestWindowsDescribeAndCollectAlignment(t *testing.T) {
	collector := &processCollector{
		pidFn:     getPIDFn(),
		cpuTotal:  NewDesc("cpu_total", "Total CPU usage", nil, nil),
		openFDs:   NewDesc("open_fds", "Number of open file descriptors", nil, nil),
		maxFDs:    NewDesc("max_fds", "Maximum file descriptors", nil, nil),
		vsize:     NewDesc("vsize", "Virtual memory size", nil, nil),
		maxVsize:  NewDesc("max_vsize", "Maximum virtual memory size", nil, nil),
		rss:       NewDesc("rss", "Resident Set Size", nil, nil),
		startTime: NewDesc("start_time", "Process start time", nil, nil),
		inBytes:   NewDesc("in_bytes", "Input bytes", nil, nil),
		outBytes:  NewDesc("out_bytes", "Output bytes", nil, nil),
	}

	// Collect and get descriptors
	descCh := make(chan *Desc, 15)
	collector.describe(descCh)
	close(descCh)

	definedDescs := make(map[string]bool)
	for desc := range descCh {
		definedDescs[desc.String()] = true
	}

	// Collect and get metrics
	metricsCh := make(chan Metric, 15)
	collector.processCollect(metricsCh)
	close(metricsCh)

	collectedMetrics := make(map[string]bool)
	for metric := range metricsCh {
		collectedMetrics[metric.Desc().String()] = true
	}

	// Verify that all described metrics are collected
	for desc := range definedDescs {
		if !collectedMetrics[desc] {
			t.Errorf("Metric %s described but not collected", desc)
		}
	}

	// Verify that no extra metrics are collected
	for desc := range collectedMetrics {
		if !definedDescs[desc] {
			t.Errorf("Metric %s collected but not described", desc)
		}
	}
}
