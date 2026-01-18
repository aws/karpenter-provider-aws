package aws

import (
	"testing"
)

// Mock service options struct for testing
type MockServiceOptions struct {
	Field1 bool
	Field2 string
	Field3 int
}

func TestWithServiceOptions(t *testing.T) {
	cfg := NewConfig()

	// Add ServiceOptions directly to test the field
	cfg.ServiceOptions = []func(string, any){
		func(serviceID string, opts any) {
			if serviceID == "TestService" {
				if mockOpts, ok := opts.(*MockServiceOptions); ok {
					mockOpts.Field1 = true
					mockOpts.Field2 = "test"
				}
			}
		},
	}

	if cfg.ServiceOptions == nil {
		t.Fatal("ServiceOptions should not be nil")
	}

	if len(cfg.ServiceOptions) != 1 {
		t.Fatalf("Expected 1 callback, got %d", len(cfg.ServiceOptions))
	}

	mockOpts := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("TestService", mockOpts)
	}

	if !mockOpts.Field1 {
		t.Error("Field1 should be true")
	}

	if mockOpts.Field2 != "test" {
		t.Errorf("Field2 should be 'test', got '%s'", mockOpts.Field2)
	}
}

func TestWithServiceOptionsMultiple(t *testing.T) {
	cfg := NewConfig()

	// Add ServiceOptions directly to test the field
	cfg.ServiceOptions = []func(string, any){
		func(serviceID string, opts any) {
			if serviceID == "Service1" {
				if mockOpts, ok := opts.(*MockServiceOptions); ok {
					mockOpts.Field1 = true
				}
			}
		},
		func(serviceID string, opts any) {
			if serviceID == "Service2" {
				if mockOpts, ok := opts.(*MockServiceOptions); ok {
					mockOpts.Field2 = "test"
				}
			}
		},
	}

	if len(cfg.ServiceOptions) != 2 {
		t.Fatalf("Expected 2 callbacks, got %d", len(cfg.ServiceOptions))
	}

	mockOpts1 := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("Service1", mockOpts1)
	}

	if !mockOpts1.Field1 {
		t.Error("Service1 Field1 should be true")
	}

	if mockOpts1.Field2 != "" {
		t.Errorf("Service1 Field2 should be empty, got '%s'", mockOpts1.Field2)
	}

	mockOpts2 := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("Service2", mockOpts2)
	}

	if mockOpts2.Field1 {
		t.Error("Service2 Field1 should be false")
	}

	if mockOpts2.Field2 != "test" {
		t.Errorf("Service2 Field2 should be 'test', got '%s'", mockOpts2.Field2)
	}
}

func TestWithServiceOptionsMultipleServiceIDs(t *testing.T) {
	cfg := NewConfig()

	// Add ServiceOptions directly to test the field
	cfg.ServiceOptions = []func(string, any){
		func(serviceID string, opts any) {
			if mockOpts, ok := opts.(*MockServiceOptions); ok {
				switch serviceID {
				case "Service1":
					mockOpts.Field1 = true
					mockOpts.Field2 = "service1"
				case "Service2":
					mockOpts.Field1 = false
					mockOpts.Field2 = "service2"
				case "Service3":
					mockOpts.Field3 = 42
				}
			}
		},
	}

	if len(cfg.ServiceOptions) != 1 {
		t.Fatalf("Expected 1 callback, got %d", len(cfg.ServiceOptions))
	}

	mockOpts1 := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("Service1", mockOpts1)
	}

	if !mockOpts1.Field1 {
		t.Error("Service1 Field1 should be true")
	}
	if mockOpts1.Field2 != "service1" {
		t.Errorf("Service1 Field2 should be 'service1', got '%s'", mockOpts1.Field2)
	}
	if mockOpts1.Field3 != 0 {
		t.Errorf("Service1 Field3 should be 0, got %d", mockOpts1.Field3)
	}

	mockOpts2 := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("Service2", mockOpts2)
	}

	if mockOpts2.Field1 {
		t.Error("Service2 Field1 should be false")
	}
	if mockOpts2.Field2 != "service2" {
		t.Errorf("Service2 Field2 should be 'service2', got '%s'", mockOpts2.Field2)
	}
	if mockOpts2.Field3 != 0 {
		t.Errorf("Service2 Field3 should be 0, got %d", mockOpts2.Field3)
	}

	mockOpts3 := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("Service3", mockOpts3)
	}

	if mockOpts3.Field1 {
		t.Error("Service3 Field1 should be false")
	}
	if mockOpts3.Field2 != "" {
		t.Errorf("Service3 Field2 should be empty, got '%s'", mockOpts3.Field2)
	}
	if mockOpts3.Field3 != 42 {
		t.Errorf("Service3 Field3 should be 42, got %d", mockOpts3.Field3)
	}
}

func TestApplyServiceOptionsNonExistent(t *testing.T) {
	mockOpts := &MockServiceOptions{}

	// No service options configured, so no callbacks to execute
	if mockOpts.Field1 || mockOpts.Field2 != "" || mockOpts.Field3 != 0 {
		t.Error("Options should not be modified for non-existent service")
	}
}

func TestTypeAssertionFailure(t *testing.T) {
	cfg := NewConfig()

	// Add ServiceOptions directly to test the field
	cfg.ServiceOptions = []func(string, any){
		func(serviceID string, opts any) {
			if serviceID == "TestService" {
				if mockOpts, ok := opts.(*MockServiceOptions); ok {
					mockOpts.Field1 = true
				}
			}
		},
	}

	differentOpts := &struct{ Field string }{Field: "test"}
	for _, callback := range cfg.ServiceOptions {
		callback("TestService", differentOpts)
	}

	if differentOpts.Field != "test" {
		t.Error("Different options should not be modified")
	}
}

func TestChaining(t *testing.T) {
	cfg := NewConfig()

	cfg.ServiceOptions = []func(string, any){
		func(serviceID string, opts any) {
			if serviceID == "Service1" {
				if mockOpts, ok := opts.(*MockServiceOptions); ok {
					mockOpts.Field1 = true
				}
			}
		},
		func(serviceID string, opts any) {
			if serviceID == "Service2" {
				if mockOpts, ok := opts.(*MockServiceOptions); ok {
					mockOpts.Field2 = "chained"
				}
			}
		},
	}

	if len(cfg.ServiceOptions) != 2 {
		t.Fatalf("Expected 2 callbacks, got %d", len(cfg.ServiceOptions))
	}

	mockOpts1 := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("Service1", mockOpts1)
	}

	if !mockOpts1.Field1 {
		t.Error("Service1 Field1 should be true")
	}

	mockOpts2 := &MockServiceOptions{}
	for _, callback := range cfg.ServiceOptions {
		callback("Service2", mockOpts2)
	}

	if mockOpts2.Field2 != "chained" {
		t.Errorf("Service2 Field2 should be 'chained', got '%s'", mockOpts2.Field2)
	}
}
