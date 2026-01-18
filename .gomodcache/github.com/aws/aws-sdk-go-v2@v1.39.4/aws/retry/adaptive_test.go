package retry

import "testing"

func TestAdaptiveMode_defaultOptions(t *testing.T) {
	a := NewAdaptiveMode()

	s, ok := a.retryer.(*Standard)
	if !ok || s == nil {
		t.Fatalf("expect nested retryer %T, got none", s)
	}

	if e, a := false, a.options.FailOnNoAttemptTokens; e != a {
		t.Errorf("expect %v default fast fail, got %v", e, a)
	}

	if e, a := DefaultMaxAttempts, s.options.MaxAttempts; e != a {
		t.Errorf("expect %v default max attempts, got %v", e, a)
	}
}

func TestAdaptiveMode_customOptions(t *testing.T) {
	a := NewAdaptiveMode(func(ao *AdaptiveModeOptions) {
		ao.FailOnNoAttemptTokens = true
		ao.StandardOptions = append(ao.StandardOptions, func(so *StandardOptions) {
			so.MaxAttempts = 10
		})
	})

	s, ok := a.retryer.(*Standard)
	if !ok || s == nil {
		t.Fatalf("expect nested retryer %T, got none", s)
	}

	if e, a := true, a.options.FailOnNoAttemptTokens; e != a {
		t.Errorf("expect %v custom fast fail, got %v", e, a)
	}

	if e, a := 10, s.options.MaxAttempts; e != a {
		t.Errorf("expect %v custom max attempts, got %v", e, a)
	}
}

func TestAdaptiveMode_copyOptions(t *testing.T) {
	origDefaultThrottles := DefaultThrottles
	defer func() {
		DefaultThrottles = origDefaultThrottles
	}()
	DefaultThrottles = append([]IsErrorThrottle{}, DefaultThrottles...)

	a := NewAdaptiveMode(func(ao *AdaptiveModeOptions) {
		ao.Throttles[0] = nil
	})

	if DefaultThrottles[0] == nil {
		t.Errorf("expect no change to global var")
	}

	if a.options.Throttles[0] != nil {
		t.Errorf("expect throttles to be changed")
	}
}
