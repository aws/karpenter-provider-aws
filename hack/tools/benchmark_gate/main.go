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

package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/samber/lo"
	benchparse "golang.org/x/tools/benchmark/parse"
)

// allocs/op and B/op are near-deterministic for a fixed input, but at large offering counts
// map growth can cauuse small run-to-run jitter
const (
	allocsTolerance = 0.005 // 0.5%
	bytesTolerance  = 0.01  // 1%
)

type metrics struct {
	ns     float64
	bytes  int64
	allocs int64
}

// parse reads a `go test -benchmem` output file into a map of benchmark name -> metrics
func parse(path string) (map[string]metrics, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	set, err := benchparse.ParseSet(f)
	if err != nil {
		return nil, err
	}
	return lo.MapValues(set, func(samples []*benchparse.Benchmark, _ string) metrics {
		return lo.Reduce(samples, func(acc metrics, s *benchparse.Benchmark, i int) metrics {
			m := metrics{ns: s.NsPerOp, bytes: int64(s.AllocedBytesPerOp), allocs: int64(s.AllocsPerOp)}
			if i == 0 {
				return m
			}
			return metrics{ns: min(acc.ns, m.ns), bytes: min(acc.bytes, m.bytes), allocs: min(acc.allocs, m.allocs)}
		}, metrics{})
	}), nil
}

func pct(old, new float64) float64 {
	if old == 0 {
		if new == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return (new - old) / old * 100
}

func run(basePath, headPath string) (int, error) {
	base, err := parse(basePath)
	if err != nil {
		return 1, err
	}
	head, err := parse(headPath)
	if err != nil {
		return 1, err
	}
	if len(head) == 0 {
		return 1, fmt.Errorf("no benchmarks parsed from head output %q", headPath)
	}

	names := make([]string, 0, len(head))
	for name := range head {
		names = append(names, name)
	}
	sort.Strings(names)

	// Check for benchmarks present at base but absent at head
	var dropped []string
	for name := range base {
		if _, ok := head[name]; !ok {
			dropped = append(dropped, name)
		}
	}
	sort.Strings(dropped)

	var b strings.Builder
	b.WriteString("## Instance-type microbenchmark gate\n\n")
	b.WriteString("Gate keys on **allocs/op** and **B/op** (deterministic). ns/op is advisory only.\n\n")
	b.WriteString("| Benchmark | Status | allocs/op | Δallocs | B/op | ΔB/op | ns/op |\n")
	b.WriteString("|---|---|--:|--:|--:|--:|--:|\n")

	var regressions []string
	for _, name := range names {
		h := head[name]
		bs, ok := base[name]
		if !ok {
			fmt.Fprintf(&b, "| `%s` | new | %d | - | %d | - | %.0f |\n", name, h.allocs, h.bytes, h.ns)
			continue
		}
		allocsRegressed := float64(h.allocs) > float64(bs.allocs)*(1+allocsTolerance)
		bytesRegressed := float64(h.bytes) > float64(bs.bytes)*(1+bytesTolerance)
		status := "ok"
		if allocsRegressed || bytesRegressed {
			status = "REGRESSED"
			regressions = append(regressions, fmt.Sprintf(
				"%s: allocs/op %d→%d (%+.1f%%), B/op %d→%d (%+.1f%%)",
				name, bs.allocs, h.allocs, pct(float64(bs.allocs), float64(h.allocs)),
				bs.bytes, h.bytes, pct(float64(bs.bytes), float64(h.bytes))))
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d | %+.1f%% | %d | %+.1f%% | %.0f |\n",
			name, status, h.allocs, pct(float64(bs.allocs), float64(h.allocs)),
			h.bytes, pct(float64(bs.bytes), float64(h.bytes)), h.ns)
	}
	for _, name := range dropped {
		bs := base[name]
		fmt.Fprintf(&b, "| `%s` | DROPPED | %d | - | %d | - | %.0f |\n", name, bs.allocs, bs.bytes, bs.ns)
	}

	table := b.String()
	fmt.Println(table)
	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		if f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
			_, _ = f.WriteString(table + "\n")
			_ = f.Close()
		}
	}

	failed := false
	if len(regressions) > 0 {
		failed = true
		fmt.Fprintln(os.Stderr, "\nFAILED: allocation/byte regression detected:")
		for _, r := range regressions {
			fmt.Fprintf(os.Stderr, "  - %s\n", r)
		}
	}
	if len(dropped) > 0 {
		failed = true
		fmt.Fprintf(os.Stderr, "\nFAILED: %d benchmark(s) present at base but missing at head (renamed or removed — coverage dropped):\n", len(dropped))
		for _, name := range dropped {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
	}
	if failed {
		return 1, nil
	}
	fmt.Println("\nPASS: no allocs/op or B/op regressions.")
	return 0, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: benchmark_gate BASE.txt HEAD.txt")
		os.Exit(1)
	}
	code, err := run(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	}
	os.Exit(code)
}
