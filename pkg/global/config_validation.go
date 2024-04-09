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

package global

import (
	"fmt"
	"net/url"
	"time"

	"go.uber.org/multierr"
)

func (c config) Validate() error {
	return multierr.Combine(
		c.validateEndpoint(),
		c.validateVMMemoryOverheadPercent(),
		c.validateAssumeRoleDuration(),
		c.validateReservedENIs(),
		c.validateRequiredFields(),
	)
}

func (c config) validateAssumeRoleDuration() error {
	if c.AssumeRoleDuration < time.Minute*15 {
		return fmt.Errorf("assume-role-duration cannot be less than 15 minutes")
	}
	return nil
}

func (c config) validateEndpoint() error {
	if c.ClusterEndpoint == "" {
		return nil
	}
	endpoint, err := url.Parse(c.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return fmt.Errorf("%q is not a valid cluster-endpoint URL", c.ClusterEndpoint)
	}
	return nil
}

func (c config) validateVMMemoryOverheadPercent() error {
	if c.VMMemoryOverheadPercent < 0 {
		return fmt.Errorf("vm-memory-overhead-percent cannot be negative")
	}
	return nil
}

func (c config) validateReservedENIs() error {
	if c.ReservedENIs < 0 {
		return fmt.Errorf("reserved-enis cannot be negative")
	}
	return nil
}

func (c config) validateRequiredFields() error {
	if c.ClusterName == "" {
		return fmt.Errorf("missing field, cluster-name")
	}
	return nil
}
