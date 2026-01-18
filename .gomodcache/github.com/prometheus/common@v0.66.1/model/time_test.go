// Copyright 2013 The Prometheus Authors
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

package model

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComparators(t *testing.T) {
	t1a := TimeFromUnix(0)
	t1b := TimeFromUnix(0)
	t2 := TimeFromUnix(2*second - 1)

	require.Truef(t, t1a.Equal(t1b), "Expected %s to be equal to %s", t1a, t1b)
	require.Falsef(t, t1a.Equal(t2), "Expected %s to not be equal to %s", t1a, t2)

	require.Truef(t, t1a.Before(t2), "Expected %s to be before %s", t1a, t2)
	require.Falsef(t, t1a.Before(t1b), "Expected %s to not be before %s", t1a, t1b)

	require.Truef(t, t2.After(t1a), "Expected %s to be after %s", t2, t1a)
	require.Falsef(t, t1b.After(t1a), "Expected %s to not be after %s", t1b, t1a)
}

func TestTimeConversions(t *testing.T) {
	unixSecs := int64(1136239445)
	unixNsecs := int64(123456789)
	unixNano := unixSecs*1e9 + unixNsecs

	t1 := time.Unix(unixSecs, unixNsecs-unixNsecs%nanosPerTick)
	t2 := time.Unix(unixSecs, unixNsecs)

	ts := TimeFromUnixNano(unixNano)
	require.Truef(t, ts.Time().Equal(t1), "Expected %s, got %s", t1, ts.Time())

	// Test available precision.
	ts = TimeFromUnixNano(t2.UnixNano())
	require.Truef(t, ts.Time().Equal(t1), "Expected %s, got %s", t1, ts.Time())

	require.Equalf(t, ts.UnixNano(), unixNano-unixNano%nanosPerTick, "Expected %d, got %d", unixNano, ts.UnixNano())
}

func TestDuration(t *testing.T) {
	duration := time.Second + time.Minute + time.Hour
	goTime := time.Unix(1136239445, 0)

	ts := TimeFromUnix(goTime.Unix())
	require.Truef(t, goTime.Add(duration).Equal(ts.Add(duration).Time()), "Expected %s to be equal to %s", goTime.Add(duration), ts.Add(duration))

	earlier := ts.Add(-duration)
	delta := ts.Sub(earlier)
	require.Equalf(t, delta, duration, "Expected %s to be equal to %s", delta, duration)
}

func TestParseDuration(t *testing.T) {
	type testCase struct {
		in              string
		out             time.Duration
		expectedString  string
		allowedNegative bool
	}

	baseCases := []testCase{
		{
			in:              "0",
			out:             0,
			expectedString:  "0s",
			allowedNegative: false,
		},
		{
			in:              "0w",
			out:             0,
			expectedString:  "0s",
			allowedNegative: false,
		},
		{
			in:              "0s",
			out:             0,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "324ms",
			out:             324 * time.Millisecond,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "3s",
			out:             3 * time.Second,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "5m",
			out:             5 * time.Minute,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "1h",
			out:             time.Hour,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "4d",
			out:             4 * 24 * time.Hour,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "4d1h",
			out:             4*24*time.Hour + time.Hour,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "14d",
			out:             14 * 24 * time.Hour,
			expectedString:  "2w",
			allowedNegative: false,
		},
		{
			in:              "3w",
			out:             3 * 7 * 24 * time.Hour,
			expectedString:  "",
			allowedNegative: false,
		},
		{
			in:              "3w2d1h",
			out:             3*7*24*time.Hour + 2*24*time.Hour + time.Hour,
			expectedString:  "23d1h",
			allowedNegative: false,
		},
		{
			in:              "10y",
			out:             10 * 365 * 24 * time.Hour,
			expectedString:  "",
			allowedNegative: false,
		},
	}

	negativeCases := []testCase{
		{
			in:              "-3s",
			out:             -3 * time.Second,
			expectedString:  "",
			allowedNegative: true,
		},
		{
			in:              "-5m",
			out:             -5 * time.Minute,
			expectedString:  "",
			allowedNegative: true,
		},
		{
			in:              "-1h",
			out:             -1 * time.Hour,
			expectedString:  "",
			allowedNegative: true,
		},
		{
			in:              "-2d",
			out:             -2 * 24 * time.Hour,
			expectedString:  "",
			allowedNegative: true,
		},
		{
			in:              "-1w",
			out:             -7 * 24 * time.Hour,
			expectedString:  "",
			allowedNegative: true,
		},
		{
			in:              "-3w2d1h",
			out:             -(3*7*24*time.Hour + 2*24*time.Hour + time.Hour),
			expectedString:  "-23d1h",
			allowedNegative: true,
		},
		{
			in:              "-10y",
			out:             -10 * 365 * 24 * time.Hour,
			expectedString:  "",
			allowedNegative: true,
		},
	}

	for _, c := range baseCases {
		c.allowedNegative = true
		negativeCases = append(negativeCases, c)
	}

	allCases := append(baseCases, negativeCases...)

	for _, c := range allCases {
		var (
			d   Duration
			err error
		)

		if c.allowedNegative {
			d, err = ParseDurationAllowNegative(c.in)
		} else {
			d, err = ParseDuration(c.in)
		}

		if err != nil {
			t.Errorf("Unexpected error on input %q", c.in)
		}

		if time.Duration(d) != c.out {
			t.Errorf("Expected %v but got %v", c.out, d)
		}

		expectedString := c.expectedString
		if expectedString == "" {
			expectedString = c.in
		}

		if d.String() != expectedString {
			t.Errorf("Expected duration string %q but got %q", c.in, d.String())
		}
	}
}

func TestDuration_UnmarshalText(t *testing.T) {
	cases := []struct {
		in  string
		out time.Duration

		expectedString string
	}{
		{
			in:             "0",
			out:            0,
			expectedString: "0s",
		}, {
			in:             "0w",
			out:            0,
			expectedString: "0s",
		}, {
			in:  "0s",
			out: 0,
		}, {
			in:  "324ms",
			out: 324 * time.Millisecond,
		}, {
			in:  "3s",
			out: 3 * time.Second,
		}, {
			in:  "5m",
			out: 5 * time.Minute,
		}, {
			in:  "1h",
			out: time.Hour,
		}, {
			in:  "4d",
			out: 4 * 24 * time.Hour,
		}, {
			in:  "4d1h",
			out: 4*24*time.Hour + time.Hour,
		}, {
			in:             "14d",
			out:            14 * 24 * time.Hour,
			expectedString: "2w",
		}, {
			in:  "3w",
			out: 3 * 7 * 24 * time.Hour,
		}, {
			in:             "3w2d1h",
			out:            3*7*24*time.Hour + 2*24*time.Hour + time.Hour,
			expectedString: "23d1h",
		}, {
			in:  "10y",
			out: 10 * 365 * 24 * time.Hour,
		},
	}

	for _, c := range cases {
		var d Duration
		err := d.UnmarshalText([]byte(c.in))
		if err != nil {
			t.Errorf("Unexpected error on input %q", c.in)
		}
		if time.Duration(d) != c.out {
			t.Errorf("Expected %v but got %v", c.out, d)
		}
		expectedString := c.expectedString
		if c.expectedString == "" {
			expectedString = c.in
		}
		text, _ := d.MarshalText() // MarshalText returns hardcoded nil
		if string(text) != expectedString {
			t.Errorf("Expected duration string %q but got %q", c.in, d.String())
		}
	}
}

func TestDuration_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		in  string
		out time.Duration

		expectedString string
	}{
		{
			in:             `"0"`,
			out:            0,
			expectedString: `"0s"`,
		},
		{
			in:             `"0w"`,
			out:            0,
			expectedString: `"0s"`,
		},
		{
			in:  `"0s"`,
			out: 0,
		},
		{
			in:  `"324ms"`,
			out: 324 * time.Millisecond,
		},
		{
			in:  `"3s"`,
			out: 3 * time.Second,
		},
		{
			in:  `"5m"`,
			out: 5 * time.Minute,
		},
		{
			in:  `"1h"`,
			out: time.Hour,
		},
		{
			in:  `"4d"`,
			out: 4 * 24 * time.Hour,
		},
		{
			in:  `"4d1h"`,
			out: 4*24*time.Hour + time.Hour,
		},
		{
			in:             `"14d"`,
			out:            14 * 24 * time.Hour,
			expectedString: `"2w"`,
		},
		{
			in:  `"3w"`,
			out: 3 * 7 * 24 * time.Hour,
		},
		{
			in:             `"3w2d1h"`,
			out:            3*7*24*time.Hour + 2*24*time.Hour + time.Hour,
			expectedString: `"23d1h"`,
		},
		{
			in:  `"10y"`,
			out: 10 * 365 * 24 * time.Hour,
		},
		{
			in:  `"289y"`,
			out: 289 * 365 * 24 * time.Hour,
		},
	}

	for _, c := range cases {
		var d Duration
		err := json.Unmarshal([]byte(c.in), &d)
		if err != nil {
			t.Errorf("Unexpected error on input %q", c.in)
		}
		if time.Duration(d) != c.out {
			t.Errorf("Expected %v but got %v", c.out, d)
		}
		expectedString := c.expectedString
		if c.expectedString == "" {
			expectedString = c.in
		}
		bytes, err := json.Marshal(d)
		if err != nil {
			t.Errorf("Unexpected error on marshal of %v: %s", d, err)
		}
		if string(bytes) != expectedString {
			t.Errorf("Expected duration string %q but got %q", c.in, d.String())
		}
	}
}

func TestParseBadDuration(t *testing.T) {
	cases := []string{
		"1",
		"1y1m1d",
		"1.5d",
		"d",
		"294y",
		"200y10400w",
		"107675d",
		"2584200h",
		"",
	}

	for _, c := range cases {
		_, err := ParseDuration(c)
		if err == nil {
			t.Errorf("Expected error on input %s", c)
		}
	}
}

func TestTimeJSON(t *testing.T) {
	tests := []struct {
		in  Time
		out string
	}{
		{Time(1), `0.001`},
		{Time(-1), `-0.001`},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			b, err := test.in.MarshalJSON()
			require.NoErrorf(t, err, "Error marshaling time: %v", err)

			if string(b) != test.out {
				t.Errorf("Mismatch in marshal expected=%s actual=%s", test.out, b)
			}

			var tm Time
			err = tm.UnmarshalJSON(b)
			require.NoErrorf(t, err, "Error Unmarshaling time: %v", err)

			require.Truef(t, test.in.Equal(tm), "Mismatch after Unmarshal expected=%v actual=%v", test.in, tm)
		})
	}
}

func BenchmarkParseDuration(b *testing.B) {
	const data = "30s"

	for i := 0; i < b.N; i++ {
		_, err := ParseDuration(data)
		require.NoError(b, err)
	}
}
