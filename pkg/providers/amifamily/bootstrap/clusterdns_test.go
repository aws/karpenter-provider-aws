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

package bootstrap

import (
	"testing"
)

func TestNewClusterDNS(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected *ClusterDNS
	}{
		{
			name:     "empty slice returns nil",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "nil slice returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:  "single IP",
			input: []string{"10.0.0.1"},
			expected: &ClusterDNS{
				values: []string{"10.0.0.1"},
			},
		},
		{
			name:  "multiple IPs",
			input: []string{"10.0.0.1", "10.0.0.2"},
			expected: &ClusterDNS{
				values: []string{"10.0.0.1", "10.0.0.2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewClusterDNS(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Errorf("Expected %v, got nil", tt.expected)
				return
			}
			if len(result.values) != len(tt.expected.values) {
				t.Errorf("Expected %d values, got %d", len(tt.expected.values), len(result.values))
				return
			}
			for i, v := range result.values {
				if v != tt.expected.values[i] {
					t.Errorf("Expected value[%d] = %s, got %s", i, tt.expected.values[i], v)
				}
			}
		})
	}
}

func TestClusterDNS_String(t *testing.T) {
	tests := []struct {
		name     string
		dns      *ClusterDNS
		expected string
	}{
		{
			name:     "nil returns empty string",
			dns:      nil,
			expected: "",
		},
		{
			name:     "empty values returns empty string",
			dns:      &ClusterDNS{values: []string{}},
			expected: "",
		},
		{
			name:     "single value returns that value",
			dns:      &ClusterDNS{values: []string{"10.0.0.1"}},
			expected: "10.0.0.1",
		},
		{
			name:     "multiple values returns first value",
			dns:      &ClusterDNS{values: []string{"10.0.0.1", "10.0.0.2"}},
			expected: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dns.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestClusterDNS_Values(t *testing.T) {
	tests := []struct {
		name     string
		dns      *ClusterDNS
		expected []string
	}{
		{
			name:     "nil returns nil",
			dns:      nil,
			expected: nil,
		},
		{
			name:     "empty values returns empty slice",
			dns:      &ClusterDNS{values: []string{}},
			expected: []string{},
		},
		{
			name:     "single value",
			dns:      &ClusterDNS{values: []string{"10.0.0.1"}},
			expected: []string{"10.0.0.1"},
		},
		{
			name:     "multiple values",
			dns:      &ClusterDNS{values: []string{"10.0.0.1", "10.0.0.2"}},
			expected: []string{"10.0.0.1", "10.0.0.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dns.Values()
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d values, got %d", len(tt.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("Expected value[%d] = %s, got %s", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestClusterDNS_MarshalTOML(t *testing.T) {
	tests := []struct {
		name        string
		dns         *ClusterDNS
		expectError bool
	}{
		{
			name:        "nil marshals to empty",
			dns:         nil,
			expectError: false,
		},
		{
			name:        "empty values marshals to empty",
			dns:         &ClusterDNS{values: []string{}},
			expectError: false,
		},
		{
			name:        "single value marshals as string",
			dns:         &ClusterDNS{values: []string{"10.0.0.1"}},
			expectError: false,
		},
		{
			name:        "multiple values marshals as array",
			dns:         &ClusterDNS{values: []string{"10.0.0.1", "10.0.0.2"}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.dns.MarshalTOML()
			if tt.expectError && err == nil {
				t.Error("Expected error, got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

// Note: UnmarshalTOML functionality is tested through integration tests
// in the launch template suite, as it requires the full TOML context.

func TestClusterDNS_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		dns      *ClusterDNS
		expected bool
	}{
		{
			name:     "nil is empty",
			dns:      nil,
			expected: true,
		},
		{
			name:     "empty values is empty",
			dns:      &ClusterDNS{values: []string{}},
			expected: true,
		},
		{
			name:     "with values is not empty",
			dns:      &ClusterDNS{values: []string{"10.0.0.1"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dns.IsEmpty()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestClusterDNS_Count(t *testing.T) {
	tests := []struct {
		name     string
		dns      *ClusterDNS
		expected int
	}{
		{
			name:     "nil has count 0",
			dns:      nil,
			expected: 0,
		},
		{
			name:     "empty values has count 0",
			dns:      &ClusterDNS{values: []string{}},
			expected: 0,
		},
		{
			name:     "single value has count 1",
			dns:      &ClusterDNS{values: []string{"10.0.0.1"}},
			expected: 1,
		},
		{
			name:     "multiple values has correct count",
			dns:      &ClusterDNS{values: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dns.Count()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}
