package logging_test

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/smithy-go/logging"
)

func TestNewStandardLogger(t *testing.T) {
	var buffer bytes.Buffer
	logger := logging.NewStandardLogger(&buffer)
	const matchStr = `SDK \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} %s foo bar baz\n`

	logger.Logf(logging.Debug, "foo %s baz", "bar")
	match := regexp.MustCompile(fmt.Sprintf(matchStr, "DEBUG"))
	if !match.Match(buffer.Bytes()) {
		t.Error("log entry did not match expected")
	}

	logger.Logf(logging.Warn, "foo %s baz", "bar")
	match = regexp.MustCompile(fmt.Sprintf(matchStr, "WARN"))
	if !match.Match(buffer.Bytes()) {
		t.Error("log entry did not match expected")
	}
}

func TestNop(t *testing.T) {
	logging.Nop{}.Logf(logging.Debug, "foo")
}

func TestWithContext(t *testing.T) {
	l := &mockContextLogger{}
	expectContextStrValue := "bar"
	nl := logging.WithContext(context.WithValue(context.Background(), "foo", expectContextStrValue), l)

	v, ok := nl.(*mockContextLogger)
	if !ok {
		t.Fatalf("expect %T, got %T", &mockContextLogger{}, nl)
	}

	if v.ctx == nil {
		t.Fatal("expect context to not be nil")
	}

	ctxValue := v.ctx.Value("foo")
	str, ok := ctxValue.(string)
	if !ok {
		t.Fatalf("expect string, got %T", str)
	}

	if str != expectContextStrValue {
		t.Errorf("expect %v, got %v", expectContextStrValue, str)
	}
}

type mockContextLogger struct {
	ctx context.Context
}

func (m mockContextLogger) WithContext(ctx context.Context) logging.Logger {
	m.ctx = ctx
	return &m
}

func (m mockContextLogger) Logf(level logging.Classification, format string, v ...interface{}) {
	return
}
