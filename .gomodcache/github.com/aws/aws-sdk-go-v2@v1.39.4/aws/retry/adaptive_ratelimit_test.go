//go:build go1.17
// +build go1.17

package retry

import (
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/internal/sdk"
)

func TestAdaptiveRateLimit_cubic(t *testing.T) {
	cases := map[string][]struct {
		throttled           bool
		timestamp           time.Time
		expectCalcuatedRate float64
	}{
		"success": {
			{
				timestamp:           time.Unix(5, 0),
				expectCalcuatedRate: 7.,
			},
			{
				timestamp:           time.Unix(6, 0),
				expectCalcuatedRate: 9.64893600966,
			},
			{
				timestamp:           time.Unix(7, 0),
				expectCalcuatedRate: 10.000030849917364,
			},
			{
				timestamp:           time.Unix(8, 0),
				expectCalcuatedRate: 10.453284520772092,
			},
			{
				timestamp:           time.Unix(9, 0),
				expectCalcuatedRate: 13.408697022224185,
			},
			{
				timestamp:           time.Unix(10, 0),
				expectCalcuatedRate: 21.26626835427364,
			},
			{
				timestamp:           time.Unix(11, 0),
				expectCalcuatedRate: 36.425998516920465,
			},
		},
		"mixed": {
			{
				timestamp:           time.Unix(5, 0),
				expectCalcuatedRate: 7.,
			},
			{
				timestamp:           time.Unix(6, 0),
				expectCalcuatedRate: 9.64893600966,
			},
			{
				throttled:           true,
				timestamp:           time.Unix(7, 0),
				expectCalcuatedRate: 6.754255206761999,
			},
			{
				throttled:           true,
				timestamp:           time.Unix(8, 0),
				expectCalcuatedRate: 4.727978644733399,
			},
			{
				timestamp:           time.Unix(9, 0),
				expectCalcuatedRate: 4.670125557970046,
			},
			{
				timestamp:           time.Unix(10, 0),
				expectCalcuatedRate: 4.770870456867401,
			},
			{
				timestamp:           time.Unix(11, 0),
				expectCalcuatedRate: 6.011819748005445,
			},
			{
				timestamp:           time.Unix(12, 0),
				expectCalcuatedRate: 10.792973431384178,
			},
		},
	}

	for name, attempts := range cases {
		t.Run(name, func(t *testing.T) {
			cleanupTime := sdk.TestingUseReferenceTime(time.Unix(0, 0))
			defer cleanupTime()

			a := newAdaptiveRateLimit()
			a.lastMaxRate = 10.
			a.lastThrottleTime = time.Unix(5, 0)

			var calculatedRate float64
			for _, attempt := range attempts {
				timeString := attempt.timestamp.UnixMilli()
				t.Run(strconv.FormatInt(timeString, 10), func(t *testing.T) {
					cleanupTime := sdk.TestingUseReferenceTime(attempt.timestamp)
					defer cleanupTime()

					a.calculateTimeWindow()
					if attempt.throttled {
						calculatedRate = a.cubicThrottle(calculatedRate)
						a.lastThrottleTime = attempt.timestamp
						a.lastMaxRate = calculatedRate
					} else {
						calculatedRate = a.cubicSuccess(attempt.timestamp)
					}
					t.Logf("timeWindow: %v, lastThrottle: %v", a.timeWindow, a.lastThrottleTime.UnixMilli())

					if e, a := attempt.expectCalcuatedRate, calculatedRate; !floatEqual(e, a) {
						t.Errorf("expect %v calculated rate got %v", e, a)
					}
				})
			}
		})
	}
}

func TestAdaptiveRateLimit_Update(t *testing.T) {
	cases := map[string][]struct {
		throttled            bool
		timestamp            time.Time
		expectMeasuredTxRate float64
		expectFillRate       float64
	}{
		"base": {
			{
				timestamp:            time.UnixMilli(200),
				expectMeasuredTxRate: 0.,
				expectFillRate:       0.5,
			},
			{
				timestamp:            time.UnixMilli(400),
				expectMeasuredTxRate: 0.,
				expectFillRate:       0.5,
			},
			{
				timestamp:            time.UnixMilli(600),
				expectMeasuredTxRate: 4.8,
				expectFillRate:       0.5,
			},
			{
				timestamp:            time.UnixMilli(800),
				expectMeasuredTxRate: 4.8,
				expectFillRate:       0.5,
			},
			{
				timestamp:            time.UnixMilli(1000),
				expectMeasuredTxRate: 4.16,
				expectFillRate:       0.5,
			},
			{
				timestamp:            time.UnixMilli(1200),
				expectMeasuredTxRate: 4.16,
				expectFillRate:       0.6912,
			},
			{
				timestamp:            time.UnixMilli(1400),
				expectMeasuredTxRate: 4.16,
				expectFillRate:       1.0976,
			},
			{
				timestamp:            time.UnixMilli(1600),
				expectMeasuredTxRate: 5.632,
				expectFillRate:       1.6384,
			},
			{
				timestamp:            time.UnixMilli(1800),
				expectMeasuredTxRate: 5.632,
				expectFillRate:       2.3328,
			},
			{
				throttled:            true,
				timestamp:            time.UnixMilli(2000),
				expectMeasuredTxRate: 4.3264,
				expectFillRate:       3.02848,
			},
			{
				timestamp:            time.UnixMilli(2200),
				expectMeasuredTxRate: 4.3264,
				expectFillRate:       3.4866391734702598,
			},
			{
				timestamp:            time.UnixMilli(2400),
				expectMeasuredTxRate: 4.3264,
				expectFillRate:       3.8218744160402554,
			},
			{
				timestamp:            time.UnixMilli(2600),
				expectMeasuredTxRate: 5.665280,
				expectFillRate:       4.053385727709987,
			},
			{
				timestamp:            time.UnixMilli(2800),
				expectMeasuredTxRate: 5.665280,
				expectFillRate:       4.200373108479455,
			},
			{
				timestamp:            time.UnixMilli(3000),
				expectMeasuredTxRate: 4.333056,
				expectFillRate:       4.282036558348658,
			},
			{
				throttled:            true,
				timestamp:            time.UnixMilli(3200),
				expectMeasuredTxRate: 4.333056,
				expectFillRate:       2.99742559084406,
			},
			{
				timestamp:            time.UnixMilli(3400),
				expectMeasuredTxRate: 4.333056,
				expectFillRate:       3.4522263943863463,
			},
		},
	}

	for name, attempts := range cases {
		t.Run(name, func(t *testing.T) {
			cleanupTime := sdk.TestingUseReferenceTime(time.Unix(0, 0))
			defer cleanupTime()

			a := newAdaptiveRateLimit()

			for _, attempt := range attempts {
				timeString := attempt.timestamp.UnixMilli()
				t.Run(strconv.FormatInt(timeString, 10), func(t *testing.T) {
					cleanupTime := sdk.TestingUseReferenceTime(attempt.timestamp)
					defer cleanupTime()

					a.Update(attempt.throttled)

					if e, a := attempt.expectMeasuredTxRate, a.measuredTxRate; !floatEqual(e, a) {
						t.Errorf("expect %v measured TX rate got %v", e, a)
					}
					if e, a := attempt.expectFillRate, a.fillRate; !floatEqual(e, a) {
						t.Errorf("expect %v token bucket rate got %v", e, a)
					}
				})
			}
		})
	}
}

func TestSecondsFloat64(t *testing.T) {
	actual := secondsFloat64(5 * time.Second)
	if e, a := 5.0, actual; e != a {
		t.Errorf("expect %v float64 seconds, got %v", e, a)
	}

	actual = secondsFloat64(0)
	if e, a := 0., actual; e != a {
		t.Errorf("expect %v float64 seconds, got %v", e, a)
	}
}

const epsilon float64 = 0.000000000001

// floatEqual compares two float values to determine if they are "equal"
// within the range of epsilon.
func floatEqual(a, b float64) bool {
	return (a-b) < epsilon && (b-a) < epsilon
}
