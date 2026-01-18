// Copyright 2014 The Prometheus Authors
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
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildFQName(t *testing.T) {
	scenarios := []struct{ namespace, subsystem, name, result string }{
		{"a", "b", "c", "a_b_c"},
		{"", "b", "c", "b_c"},
		{"a", "", "c", "a_c"},
		{"", "", "c", "c"},
		{"a", "b", "", ""},
		{"a", "", "", ""},
		{"", "b", "", ""},
		{" ", "", "", ""},
	}

	for i, s := range scenarios {
		if want, got := s.result, BuildFQName(s.namespace, s.subsystem, s.name); want != got {
			t.Errorf("%d. want %s, got %s", i, want, got)
		}
	}
}

func TestWithExemplarsMetric(t *testing.T) {
	t.Run("histogram", func(t *testing.T) {
		// Create a constant histogram from values we got from a 3rd party telemetry system.
		h := MustNewConstHistogram(
			NewDesc("http_request_duration_seconds", "A histogram of the HTTP request durations.", nil, nil),
			4711, 403.34,
			// Four buckets, but we expect five as the +Inf bucket will be created if we see value outside of those buckets.
			map[float64]uint64{25: 121, 50: 2403, 100: 3221, 200: 4233},
		)

		m := &withExemplarsMetric{Metric: h, exemplars: []*dto.Exemplar{
			{Value: proto.Float64(2000.0)}, // Unordered exemplars.
			{Value: proto.Float64(500.0)},
			{Value: proto.Float64(42.0)},
			{Value: proto.Float64(157.0)},
			{Value: proto.Float64(100.0)},
			{Value: proto.Float64(89.0)},
			{Value: proto.Float64(24.0)},
			{Value: proto.Float64(25.1)},
		}}
		metric := dto.Metric{}
		if err := m.Write(&metric); err != nil {
			t.Fatal(err)
		}
		if want, got := 5, len(metric.GetHistogram().Bucket); want != got {
			t.Errorf("want %v, got %v", want, got)
		}

		expectedExemplarVals := []float64{24.0, 25.1, 89.0, 157.0, 500.0}
		for i, b := range metric.GetHistogram().Bucket {
			if b.Exemplar == nil {
				t.Errorf("Expected exemplar for bucket %v, got nil", i)
			}
			if want, got := expectedExemplarVals[i], *metric.GetHistogram().Bucket[i].Exemplar.Value; want != got {
				t.Errorf("%v: want %v, got %v", i, want, got)
			}
		}

		infBucket := metric.GetHistogram().Bucket[len(metric.GetHistogram().Bucket)-1]

		if want, got := math.Inf(1), infBucket.GetUpperBound(); want != got {
			t.Errorf("want %v, got %v", want, got)
		}

		if want, got := uint64(4711), infBucket.GetCumulativeCount(); want != got {
			t.Errorf("want %v, got %v", want, got)
		}
	})
}

func TestWithExemplarsNativeHistogramMetric(t *testing.T) {
	t.Run("native histogram single exemplar", func(t *testing.T) {
		// Create a constant histogram from values we got from a 3rd party telemetry system.
		h := MustNewConstNativeHistogram(
			NewDesc("http_request_duration_seconds", "A histogram of the HTTP request durations.", nil, nil),
			10, 12.1, map[int]int64{1: 7, 2: 1, 3: 2}, map[int]int64{}, 0, 2, 0.2, time.Date(
				2009, 11, 17, 20, 34, 58, 651387237, time.UTC))
		m := &withExemplarsMetric{Metric: h, exemplars: []*dto.Exemplar{
			{Value: proto.Float64(2000.0), Timestamp: timestamppb.New(time.Date(2009, 11, 17, 20, 34, 58, 3243244, time.UTC))},
		}}
		metric := dto.Metric{}
		if err := m.Write(&metric); err != nil {
			t.Fatal(err)
		}
		if want, got := 1, len(metric.GetHistogram().Exemplars); want != got {
			t.Errorf("want %v, got %v", want, got)
		}

		for _, b := range metric.GetHistogram().Bucket {
			if b.Exemplar != nil {
				t.Error("Not expecting exemplar for bucket")
			}
		}
	})
	t.Run("native histogram multiple exemplar", func(t *testing.T) {
		// Create a constant histogram from values we got from a 3rd party telemetry system.
		h := MustNewConstNativeHistogram(
			NewDesc("http_request_duration_seconds", "A histogram of the HTTP request durations.", nil, nil),
			10, 12.1, map[int]int64{1: 7, 2: 1, 3: 2}, map[int]int64{}, 0, 2, 0.2, time.Date(
				2009, 11, 17, 20, 34, 58, 651387237, time.UTC))
		m := &withExemplarsMetric{Metric: h, exemplars: []*dto.Exemplar{
			{Value: proto.Float64(2000.0), Timestamp: timestamppb.New(time.Date(2009, 11, 17, 20, 34, 58, 3243244, time.UTC))},
			{Value: proto.Float64(1000.0), Timestamp: timestamppb.New(time.Date(2009, 11, 17, 20, 34, 59, 3243244, time.UTC))},
		}}
		metric := dto.Metric{}
		if err := m.Write(&metric); err != nil {
			t.Fatal(err)
		}
		if want, got := 2, len(metric.GetHistogram().Exemplars); want != got {
			t.Errorf("want %v, got %v", want, got)
		}

		for _, b := range metric.GetHistogram().Bucket {
			if b.Exemplar != nil {
				t.Error("Not expecting exemplar for bucket")
			}
		}
	})
	t.Run("native histogram exemplar without timestamp", func(t *testing.T) {
		// Create a constant histogram from values we got from a 3rd party telemetry system.
		h := MustNewConstNativeHistogram(
			NewDesc("http_request_duration_seconds", "A histogram of the HTTP request durations.", nil, nil),
			10, 12.1, map[int]int64{1: 7, 2: 1, 3: 2}, map[int]int64{}, 0, 2, 0.2, time.Date(
				2009, 11, 17, 20, 34, 58, 651387237, time.UTC))
		m := MustNewMetricWithExemplars(h, Exemplar{
			Value: 1000.0,
		})
		metric := dto.Metric{}
		if err := m.Write(&metric); err != nil {
			t.Fatal(err)
		}
		if want, got := 1, len(metric.GetHistogram().Exemplars); want != got {
			t.Errorf("want %v, got %v", want, got)
		}
		if got := metric.GetHistogram().Exemplars[0].Timestamp; got == nil {
			t.Errorf("Got nil timestamp")
		}

		for _, b := range metric.GetHistogram().Bucket {
			if b.Exemplar != nil {
				t.Error("Not expecting exemplar for bucket")
			}
		}
	})
	t.Run("nativehistogram metric exemplars should be available in both buckets and exemplars", func(t *testing.T) {
		now := time.Now()
		tcs := []struct {
			Name                         string
			Count                        uint64
			Sum                          float64
			PositiveBuckets              map[int]int64
			NegativeBuckets              map[int]int64
			ZeroBucket                   uint64
			NativeHistogramSchema        int32
			NativeHistogramZeroThreshold float64
			CreatedTimestamp             time.Time
			Bucket                       []*dto.Bucket
			Exemplars                    []Exemplar
			Want                         *dto.Metric
		}{
			{
				Name:  "test_metric",
				Count: 6,
				Sum:   7.4,
				PositiveBuckets: map[int]int64{
					0: 1, 2: 2, 4: 2,
				},
				NegativeBuckets: map[int]int64{},
				ZeroBucket:      1,

				NativeHistogramSchema:        2,
				NativeHistogramZeroThreshold: 2.938735877055719e-39,
				CreatedTimestamp:             now,
				Bucket: []*dto.Bucket{
					{
						CumulativeCount: PointOf(uint64(6)),
						UpperBound:      PointOf(float64(1)),
					},
					{
						CumulativeCount: PointOf(uint64(8)),
						UpperBound:      PointOf(float64(2)),
					},
					{
						CumulativeCount: PointOf(uint64(11)),
						UpperBound:      PointOf(float64(5)),
					},
					{
						CumulativeCount: PointOf(uint64(13)),
						UpperBound:      PointOf(float64(10)),
					},
				},
				Exemplars: []Exemplar{
					{
						Timestamp: now,
						Value:     10,
					},
				},
				Want: &dto.Metric{
					Histogram: &dto.Histogram{
						SampleCount:   proto.Uint64(6),
						SampleSum:     proto.Float64(7.4),
						Schema:        proto.Int32(2),
						ZeroThreshold: proto.Float64(2.938735877055719e-39),
						ZeroCount:     proto.Uint64(1),
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(5)},
						},
						PositiveDelta: []int64{1, -1, 2, -2, 2},
						Exemplars: []*dto.Exemplar{
							{
								Value:     PointOf(float64(10)),
								Timestamp: timestamppb.New(now),
							},
						},
						Bucket: []*dto.Bucket{
							{
								CumulativeCount: PointOf(uint64(6)),
								UpperBound:      PointOf(float64(1)),
							},
							{
								CumulativeCount: PointOf(uint64(8)),
								UpperBound:      PointOf(float64(2)),
							},
							{
								CumulativeCount: PointOf(uint64(11)),
								UpperBound:      PointOf(float64(5)),
							},
							{
								CumulativeCount: PointOf(uint64(13)),
								UpperBound:      PointOf(float64(10)),
								Exemplar: &dto.Exemplar{
									Timestamp: timestamppb.New(now),
									Value:     PointOf(float64(10)),
								},
							},
						},
						CreatedTimestamp: timestamppb.New(now),
					},
				},
			},
		}

		for _, tc := range tcs {
			m, err := newNativeHistogramWithClassicBuckets(NewDesc(tc.Name, "None", []string{}, map[string]string{}), tc.Count, tc.Sum, tc.PositiveBuckets, tc.NegativeBuckets, tc.ZeroBucket, tc.NativeHistogramSchema, tc.NativeHistogramZeroThreshold, tc.CreatedTimestamp, tc.Bucket)
			if err != nil {
				t.Fail()
			}
			metricWithExemplar, err := NewMetricWithExemplars(m, tc.Exemplars[0])
			if err != nil {
				t.Fail()
			}
			got := &dto.Metric{}
			err = metricWithExemplar.Write(got)
			if err != nil {
				t.Fail()
			}

			if !proto.Equal(tc.Want, got) {
				t.Errorf("want histogram %q, got %q", tc.Want, got)
			}

		}
	})
}

func PointOf[T any](value T) *T {
	return &value
}

// newNativeHistogramWithClassicBuckets returns a Metric representing
// a native histogram that also has classic buckets. This is for testing purposes.
func newNativeHistogramWithClassicBuckets(
	desc *Desc,
	count uint64,
	sum float64,
	positiveBuckets, negativeBuckets map[int]int64,
	zeroBucket uint64,
	schema int32,
	zeroThreshold float64,
	createdTimestamp time.Time,
	// DummyNativeHistogram also defines buckets in the metric for testing
	buckets []*dto.Bucket,
	labelValues ...string,
) (Metric, error) {
	if desc.err != nil {
		fmt.Println("error", desc.err)
		return nil, desc.err
	}
	if err := validateLabelValues(labelValues, len(desc.variableLabels.names)); err != nil {
		return nil, err
	}
	if schema > nativeHistogramSchemaMaximum || schema < nativeHistogramSchemaMinimum {
		return nil, errors.New("invalid native histogram schema")
	}
	if err := validateCount(sum, count, negativeBuckets, positiveBuckets, zeroBucket); err != nil {
		return nil, err
	}

	NegativeSpan, NegativeDelta := makeBucketsFromMap(negativeBuckets)
	PositiveSpan, PositiveDelta := makeBucketsFromMap(positiveBuckets)
	ret := &constNativeHistogram{
		desc: desc,
		Histogram: dto.Histogram{
			CreatedTimestamp: timestamppb.New(createdTimestamp),
			Schema:           &schema,
			ZeroThreshold:    &zeroThreshold,
			SampleCount:      &count,
			SampleSum:        &sum,

			NegativeSpan:  NegativeSpan,
			NegativeDelta: NegativeDelta,

			PositiveSpan:  PositiveSpan,
			PositiveDelta: PositiveDelta,

			ZeroCount: proto.Uint64(zeroBucket),

			// DummyNativeHistogram also defines buckets in the metric
			Bucket: buckets,
		},
		labelPairs: MakeLabelPairs(desc, labelValues),
	}
	if *ret.ZeroThreshold == 0 && *ret.ZeroCount == 0 && len(ret.PositiveSpan) == 0 && len(ret.NegativeSpan) == 0 {
		ret.PositiveSpan = []*dto.BucketSpan{{
			Offset: proto.Int32(0),
			Length: proto.Uint32(0),
		}}
	}
	return ret, nil
}
