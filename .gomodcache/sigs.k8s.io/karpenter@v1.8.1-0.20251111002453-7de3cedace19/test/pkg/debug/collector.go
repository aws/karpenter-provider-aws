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

package debug

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/samber/lo"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	StageE2E        = "E2E"
	StageBeforeEach = "BeforeEach"
	StageAfterEach  = "AfterEach"
)

type TimeIntervalCollector struct {
	starts map[string]time.Time
	ends   map[string]time.Time
	// used for ordering on Collect
	Stages             []string
	suiteTimeIntervals map[string][]TimeInterval
}

func NewTimestampCollector() *TimeIntervalCollector {
	return &TimeIntervalCollector{
		starts:             map[string]time.Time{},
		ends:               map[string]time.Time{},
		Stages:             []string{},
		suiteTimeIntervals: map[string][]TimeInterval{},
	}
}

func (t *TimeIntervalCollector) Reset() {
	t.starts = map[string]time.Time{}
	t.ends = map[string]time.Time{}
	t.Stages = []string{}
}

// Record adds the current starts/ends/stages as a list of time intervals,
// and adds it to the existingTimestamps, then resets the starts/ends/stages.
func (t *TimeIntervalCollector) Record(name string) {
	intervals := t.translate()
	caser := cases.Title(language.AmericanEnglish)
	sanitized := strings.ReplaceAll(caser.String(name), " ", "")
	t.suiteTimeIntervals[sanitized] = intervals
	t.Reset()
}

// Start will add a timestamp with the given stage and add it to the list
// If there is no End associated with a Start, the interval's inferred End
// is at the start of the AfterEach.
func (t *TimeIntervalCollector) Start(stage string) time.Time {
	t.starts[stage] = time.Now()
	t.Stages = append(t.Stages, stage)
	return time.Now()
}

// Finalize will automatically add End time entries for Start entries
// without a corresponding set End. This is useful for when the test
// fails, since deferring time recording is tough to do.
func (t *TimeIntervalCollector) Finalize() {
	for stage := range t.starts {
		// If it's one of the enum stages, don't add, as these are added automatically.
		if stage == StageE2E || stage == StageBeforeEach || stage == StageAfterEach {
			continue
		}
		_, ok := t.ends[stage]
		if ok {
			continue
		}
		t.ends[stage] = time.Now()
	}
}

// End will mark the interval's end time.
// If there is no End associated with a Start, the interval's inferred End
// is at the start of the AfterEach.
func (t *TimeIntervalCollector) End(stage string) {
	t.ends[stage] = time.Now()
}

// translate takes the starts and ends in the existing TimeIntervalCollector
// and adds the lists of intervals into the suiteTimeIntervals to be used
// later for csv printing.
func (t *TimeIntervalCollector) translate() []TimeInterval {
	intervals := []TimeInterval{}
	for _, stage := range t.Stages {
		end, ok := t.ends[stage]
		if !ok {
			end = time.Now()
		}
		intervals = append(intervals, TimeInterval{
			Start: t.starts[stage],
			End:   end,
			Stage: stage,
		})
	}
	return intervals
}

type TimeInterval struct {
	Start time.Time
	End   time.Time
	Stage string
}

func (t TimeInterval) String() []string {
	return []string{t.Stage, t.Start.UTC().Format(time.RFC3339), t.End.UTC().Format(time.RFC3339)}
}

// PrintTestTimes returns a list of tables.
// Each table has a list of Timestamps, where each timestamp is a list of strings.
func PrintTestTimes(times map[string][]TimeInterval) map[string][][]string {
	ret := map[string][][]string{}
	for name, interval := range times {
		ret[name] = lo.Map(interval, func(t TimeInterval, _ int) []string {
			return t.String()
		})
	}
	return ret
}

// WriteTimestamps will create a temp directory and a .csv file for each suite test
// If the OUTPUT_DIR environment variable is set, we'll print the csvs to that directory.
func WriteTimestamps(outputDir string, timestamps *TimeIntervalCollector) error {
	var err error
	if outputDir == "" {
		outputDir, err = os.MkdirTemp("/tmp", "")
		if err != nil {
			return err
		}
	}
	for name, table := range PrintTestTimes(timestamps.suiteTimeIntervals) {
		file, err := os.CreateTemp(outputDir, fmt.Sprintf("*-%s.csv", name))
		if err != nil {
			return err
		}
		defer file.Close()

		w := csv.NewWriter(file)

		// Write the header
		header := []string{"Stage", "Start", "End"}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
		if err := w.WriteAll(table); err != nil { // calls Flush internally
			return fmt.Errorf("failed to flush writer: %w", err)
		}

		fmt.Println("-------- SUCCESS ---------")
		fmt.Printf("Printed CSV TO %s\n", file.Name())
	}
	return nil
}
