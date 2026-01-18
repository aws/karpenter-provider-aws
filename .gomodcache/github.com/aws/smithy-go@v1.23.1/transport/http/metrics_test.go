package http

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"testing"
	"time"

	"github.com/aws/smithy-go/metrics"
)

type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

func (m *mockClock) Advance(sec, ms time.Duration) {
	m.now = m.now.Add(sec*time.Second + ms*time.Millisecond)
}

func mockNow(start time.Time) (*mockClock, func()) {
	clock := &mockClock{start}
	now = clock.Now
	return clock, func() { now = time.Now }
}

type mockMeter struct {
	metrics.Meter
	counters   map[string]*mockUpDownCounter
	histograms map[string]*mockHistogram
}

func (m *mockMeter) Int64UpDownCounter(name string, opts ...metrics.InstrumentOption) (metrics.Int64UpDownCounter, error) {
	if m.counters == nil {
		m.counters = map[string]*mockUpDownCounter{}
	}
	c := &mockUpDownCounter{}
	m.counters[name] = c
	return c, nil
}

func (m *mockMeter) Float64Histogram(name string, opts ...metrics.InstrumentOption) (metrics.Float64Histogram, error) {
	if m.histograms == nil {
		m.histograms = map[string]*mockHistogram{}
	}
	h := &mockHistogram{}
	m.histograms[name] = h
	return h, nil
}

type mockUpDownCounter struct {
	value int64
}

func (m *mockUpDownCounter) Add(ctx context.Context, incr int64, opts ...metrics.RecordMetricOption) {
	m.value += incr
}

type mockHistogram struct {
	value float64
}

func (m *mockHistogram) Record(ctx context.Context, value float64, opts ...metrics.RecordMetricOption) {
	m.value = value
}

type mockClient struct {
	clock *mockClock
	trace *httptrace.ClientTrace
}

func (m *mockClient) Do(*http.Request) (*http.Response, error) {
	m.trace.DNSStart(httptrace.DNSStartInfo{})
	m.clock.Advance(1, 500)
	m.trace.DNSDone(httptrace.DNSDoneInfo{})

	m.trace.ConnectStart("", "")
	m.clock.Advance(0, 250)
	m.trace.ConnectDone("", "", nil)

	m.trace.TLSHandshakeStart()
	m.clock.Advance(0, 377)
	m.trace.TLSHandshakeDone(tls.ConnectionState{}, nil)

	m.trace.GotConn(httptrace.GotConnInfo{})

	m.clock.Advance(99, 99)
	m.trace.GotFirstResponseByte()

	m.clock.Advance(999, 999)

	// explicitly do NOT do this so we can verify the nonzero count
	// m.trace.PutIdleConn(nil)

	return &http.Response{}, nil
}

// There's not a whole lot we can actually test without doing an actual HTTP
// request, but we can at least check that we're calculating things correctly.
func TestHTTPMetrics(t *testing.T) {
	start := time.Unix(1, 15)
	clock, restore := mockNow(start)
	defer restore()

	client := &mockClient{}
	meter := &mockMeter{}

	ctx, instrumented, err := withMetrics(context.Background(), client, meter)
	if err != nil {
		t.Fatalf("withMetrics: %v", err)
	}

	trace := httptrace.ContextClientTrace(ctx)
	if trace == nil {
		t.Fatalf("there should be a trace but there isn't")
	}

	client.clock = clock
	client.trace = trace
	instrumented.Do(&http.Request{})

	expectHistogram(t, meter, "client.http.connections.dns_lookup_duration", 1.5)
	expectHistogram(t, meter, "client.http.connections.acquire_duration", 0.25)
	expectHistogram(t, meter, "client.http.connections.tls_handshake_duration", 0.377)

	elapsedTTFB := (1+99)*time.Second + (500+250+377+99)*time.Millisecond
	expectHistogram(t, meter, "client.http.time_to_first_byte", float64(elapsedTTFB)/1e9)

	elapsedDo := clock.now.Sub(start)
	expectHistogram(t, meter, "client.http.do_request_duration", float64(elapsedDo)/1e9)

	expectCounter(t, meter, "client.http.connections.usage", 1)

}

func expectHistogram(t *testing.T, meter *mockMeter, name string, expect float64) {
	t.Helper()

	histogram, ok := meter.histograms[name]
	if !ok {
		t.Errorf("missing histogram: %s", name)
		return
	}
	if expect != histogram.value {
		t.Errorf("%s: %v != %v", name, expect, histogram.value)
	}
}

func expectCounter(t *testing.T, meter *mockMeter, name string, expect int64) {
	t.Helper()

	counter, ok := meter.counters[name]
	if !ok {
		t.Errorf("missing counter: %s", name)
		return
	}
	if expect != counter.value {
		t.Errorf("%s: %v != %v", name, expect, counter.value)
	}
}
