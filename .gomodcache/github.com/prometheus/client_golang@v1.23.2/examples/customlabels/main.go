// Copyright 2022 The Prometheus Authors
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

// A simple example of how to add fixed custom labels to all metrics exported by the origin collector.
// For more details, see the documentation: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#WrapRegistererWith

package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

func main() {
	flag.Parse()

	// Create a new registry.
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// We should see the following metrics with an extra source label. But
	// other collectors registered above are expected not to have the extra
	// label.
	// See also https://prometheus.io/docs/instrumenting/writing_exporters/#target-labels-not-static-scraped-labels
	startFireKeeper(prometheus.WrapRegistererWith(prometheus.Labels{"component": "FireKeeper"}, reg))
	startSparkForge(prometheus.WrapRegistererWith(prometheus.Labels{"component": "SparkForge"}, reg))

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func startFireKeeper(reg prometheus.Registerer) {
	firesMaintained := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "fires_maintained_total",
		Help: "Total number of fires maintained",
	})

	sparksDistributed := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "sparks_distributed_total",
		Help: "Total number of sparks distributed",
	})

	go func() {
		for {
			time.Sleep(5 * time.Second)
			firesMaintained.Inc()
			log.Println("FireKeeper maintained a fire")
		}
	}()

	go func() {
		for {
			time.Sleep(7 * time.Second)
			sparksDistributed.Inc()
			log.Println("FireKeeper distributed a spark")
		}
	}()
}

func startSparkForge(reg prometheus.Registerer) {
	itemsForged := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "items_forged_total",
		Help: "Total number of items forged",
	})

	go func() {
		for {
			time.Sleep(6 * time.Second)
			itemsForged.Inc()
			log.Println("SparkForge forged an item")
		}
	}()
}
