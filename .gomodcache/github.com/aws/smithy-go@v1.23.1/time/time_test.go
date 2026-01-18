package time

import (
	"math"
	"strconv"
	"testing"
	"time"
)

func TestDateTime(t *testing.T) {
	cases := map[string]struct {
		TimeString      string
		TimeValue       time.Time
		SymmetricString bool
	}{
		"no offset": {
			TimeString: "1985-04-12T23:20:50.52Z",
			TimeValue: time.Date(1985, 4, 12, 23, 20, 50, int(520*time.Millisecond),
				time.UTC),
			SymmetricString: true,
		},
		"no offset, no Z": {
			TimeString: "1985-04-12T23:20:50.524",
			TimeValue: time.Date(1985, 4, 12, 23, 20, 50, int(524*time.Millisecond),
				time.UTC),
			SymmetricString: false,
		},
		"with negative offset": {
			TimeString: "1985-04-12T23:20:50.52-07:00",
			TimeValue: time.Date(1985, 4, 12, 23, 20, 50, int(520*time.Millisecond),
				time.FixedZone("-0700", -7*60*60)),
			SymmetricString: false,
		},
		"with positive offset": {
			TimeString: "1985-04-12T23:20:50.52+07:00",
			TimeValue: time.Date(1985, 4, 12, 23, 20, 50, int(520*time.Millisecond),
				time.FixedZone("-0700", +7*60*60)),
			SymmetricString: false,
		},
		"UTC serialize": {
			TimeString: "1985-04-13T06:20:50.52Z",
			TimeValue: time.Date(1985, 4, 12, 23, 20, 50, int(520*time.Millisecond),
				time.FixedZone("-0700", -7*60*60)),
			SymmetricString: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			formattedTimeValue := FormatDateTime(c.TimeValue)

			// Round Trip time value ensure format and parse are compatible.
			parsedTimeValue, err := ParseDateTime(formattedTimeValue)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			parsedTimeString, err := ParseDateTime(c.TimeString)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			// Ensure parsing date time from string matches expected time value.
			if c.SymmetricString {
				if e, a := c.TimeString, formattedTimeValue; e != a {
					t.Errorf("expected %v, got %v", e, a)
				}
			}
			if e, a := c.TimeValue, parsedTimeValue; !e.Equal(a) {
				t.Errorf("expected %v, got %v", e, a)
			}
			if e, a := c.TimeValue, parsedTimeString; !e.Equal(a) {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}
}

func TestHTTPDate(t *testing.T) {
	refTime := time.Date(2014, 4, 29, 18, 30, 38, 0, time.UTC)

	httpDate := FormatHTTPDate(refTime)
	if e, a := "Tue, 29 Apr 2014 18:30:38 GMT", httpDate; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	parseTime, err := ParseHTTPDate(httpDate)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if e, a := refTime, parseTime; !e.Equal(a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	// UTC serialize date time.
	refTime = time.Date(2014, 4, 29, 18, 30, 38, 0, time.FixedZone("-700", -7*60*60))
	httpDate = FormatHTTPDate(refTime)
	if e, a := "Wed, 30 Apr 2014 01:30:38 GMT", httpDate; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestParseHTTPDate(t *testing.T) {
	cases := map[string]struct {
		date    string
		expect  time.Time
		wantErr bool
	}{
		"with leading zero on day": {
			date:   "Fri, 05 Feb 2021 19:12:15 GMT",
			expect: time.Date(2021, 2, 5, 19, 12, 15, 0, time.UTC),
		},
		"without leading zero on day": {
			date:   "Fri, 5 Feb 2021 19:12:15 GMT",
			expect: time.Date(2021, 2, 5, 19, 12, 15, 0, time.UTC),
		},
		"with double digit day": {
			date:   "Fri, 15 Feb 2021 19:12:15 GMT",
			expect: time.Date(2021, 2, 15, 19, 12, 15, 0, time.UTC),
		},
		"RFC850": {
			date:   "Friday, 05-Feb-21 19:12:15 UTC",
			expect: time.Date(2021, 2, 5, 19, 12, 15, 0, time.UTC),
		},
		"ANSIC with leading zero on day": {
			date:   "Fri Feb 05 19:12:15 2021",
			expect: time.Date(2021, 2, 5, 19, 12, 15, 0, time.UTC),
		},
		"ANSIC without leading zero on day": {
			date:   "Fri Feb 5 19:12:15 2021",
			expect: time.Date(2021, 2, 5, 19, 12, 15, 0, time.UTC),
		},
		"ANSIC with double digit day": {
			date:   "Fri Feb 15 19:12:15 2021",
			expect: time.Date(2021, 2, 15, 19, 12, 15, 0, time.UTC),
		},
		"invalid time format": {
			date:    "1985-04-12T23:20:50.52Z",
			wantErr: true,
		},
		"shortened year with double digit day": {
			date:   "Thu, 11 Feb 21 11:04:03 GMT",
			expect: time.Date(2021, 2, 11, 11, 04, 03, 0, time.UTC),
		},
		"shortened year without leading zero day": {
			date:   "Thu, 5 Feb 21 11:04:03 GMT",
			expect: time.Date(2021, 2, 5, 11, 04, 03, 0, time.UTC),
		},
		"shortened year with leading zero day": {
			date:   "Thu, 05 Feb 21 11:04:03 GMT",
			expect: time.Date(2021, 2, 5, 11, 04, 03, 0, time.UTC),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := ParseHTTPDate(tt.date)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if result.IsZero() {
				t.Fatalf("expected non-zero timestamp")
			}
			if tt.expect != result {
				t.Fatalf("expected %q, got %q", tt.expect, result)
			}
		})
	}
}

func TestEpochSeconds(t *testing.T) {
	cases := []struct {
		reference    time.Time
		expectedUnix float64
		expectedTime time.Time
	}{
		{
			reference:    time.Date(2018, 1, 9, 20, 51, 21, 123399936, time.UTC),
			expectedUnix: 1515531081.123,
			expectedTime: time.Date(2018, 1, 9, 20, 51, 21, 1.23e8, time.UTC),
		},
		{
			reference:    time.Date(2018, 1, 9, 20, 51, 21, 1e8, time.UTC),
			expectedUnix: 1515531081.1,
			expectedTime: time.Date(2018, 1, 9, 20, 51, 21, 1e8, time.UTC),
		},
		{
			reference:    time.Date(2018, 1, 9, 20, 51, 21, 123567891, time.UTC),
			expectedUnix: 1515531081.123,
			expectedTime: time.Date(2018, 1, 9, 20, 51, 21, 1.23e8, time.UTC),
		},
		{
			reference:    time.Unix(0, math.MaxInt64).UTC(),
			expectedUnix: 9223372036.854,
			expectedTime: time.Date(2262, 04, 11, 23, 47, 16, 8.54e8, time.UTC),
		},
		{
			reference:    time.Date(2018, 1, 9, 20, 51, 21, 123567891, time.FixedZone("-0700", -7*60*60)),
			expectedUnix: 1515556281.123,
			expectedTime: time.Date(2018, 1, 10, 03, 51, 21, 1.23e8, time.UTC),
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			epochSeconds := FormatEpochSeconds(tt.reference)
			if e, a := tt.expectedUnix, epochSeconds; e != a {
				t.Errorf("expected %v, got %v", e, a)
			}

			parseTime := ParseEpochSeconds(epochSeconds)

			if e, a := tt.expectedTime, parseTime; !e.Equal(a) {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}

	// Check an additional edge that higher precision values are truncated to milliseconds
	if e, a := time.Date(2018, 1, 9, 20, 51, 21, 1.23e8, time.UTC), ParseEpochSeconds(1515531081.12356); !e.Equal(a) {
		t.Errorf("expected %v, got %v", e, a)
	}
}
