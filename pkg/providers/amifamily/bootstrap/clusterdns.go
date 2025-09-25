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
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// ClusterDNS represents the cluster DNS configuration that can be either a single IP string
// or a slice of IP strings, matching Bottlerocket's cluster-dns-ip field flexibility.
//
// This type handles the TOML marshaling/unmarshaling to support both:
// - cluster-dns-ip = "10.0.0.1" (single string)
// - cluster-dns-ip = ["10.0.0.1", "10.0.0.2"] (array of strings)
type ClusterDNS struct {
	values []string
}

// NewClusterDNS creates a ClusterDNS from a slice of strings.
// Returns nil if the input slice is empty.
func NewClusterDNS(ips []string) *ClusterDNS {
	if len(ips) == 0 {
		return nil
	}
	return &ClusterDNS{values: ips}
}

// String returns the first DNS IP for backward compatibility.
// This is useful when legacy code expects a single DNS IP.
func (c *ClusterDNS) String() string {
	if c == nil || len(c.values) == 0 {
		return ""
	}
	return c.values[0]
}

// Values returns all DNS IPs as a slice.
// Returns nil if ClusterDNS is nil.
func (c *ClusterDNS) Values() []string {
	if c == nil {
		return nil
	}
	return c.values
}

// IsEmpty returns true if ClusterDNS is nil or has no values.
func (c *ClusterDNS) IsEmpty() bool {
	return c == nil || len(c.values) == 0
}

// Count returns the number of DNS IPs.
func (c *ClusterDNS) Count() int {
	if c == nil {
		return 0
	}
	return len(c.values)
}

// MarshalTOML implements custom TOML marshaling.
// - If single value, marshals as string: cluster-dns-ip = "10.0.0.1"
// - If multiple values, marshals as array: cluster-dns-ip = ["10.0.0.1", "10.0.0.2"]
// - If empty, returns empty bytes
func (c *ClusterDNS) MarshalTOML() ([]byte, error) {
	if c.IsEmpty() {
		return []byte(""), nil
	}
	
	if len(c.values) == 1 {
		return toml.Marshal(c.values[0])
	}
	return toml.Marshal(c.values)
}

// UnmarshalTOML implements custom TOML unmarshaling.
// Handles both string and array formats from TOML input.
func (c *ClusterDNS) UnmarshalTOML(data []byte) error {
	// Try to unmarshal as string first (most common case)
	var singleIP string
	if err := toml.Unmarshal(data, &singleIP); err == nil {
		c.values = []string{singleIP}
		return nil
	}

	// Try to unmarshal as array
	var multipleIPs []string
	if err := toml.Unmarshal(data, &multipleIPs); err == nil {
		c.values = multipleIPs
		return nil
	}

	return fmt.Errorf("cluster-dns-ip must be either a string or array of strings")
}
