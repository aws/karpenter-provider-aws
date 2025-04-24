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

package options

import (
	"fmt"
	"net/url"

	"go.uber.org/multierr"
)

func (o Options) Validate() error {
	return multierr.Combine(
		o.validateEndpoint(),
		o.validateVMMemoryOverheadPercent(),
		o.validateReservedENIs(),
		o.validateUnavailableOfferingsTTL(),
		o.validateRequiredFields(),
	)
}

func (o Options) validateEndpoint() error {
	if o.ClusterEndpoint == "" {
		return nil
	}
	endpoint, err := url.Parse(o.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return fmt.Errorf("%q is not a valid cluster-endpoint URL", o.ClusterEndpoint)
	}
	return nil
}

func (o Options) validateVMMemoryOverheadPercent() error {
	if o.VMMemoryOverheadPercent < 0 {
		return fmt.Errorf("vm-memory-overhead-percent cannot be negative")
	}
	return nil
}

func (o Options) validateReservedENIs() error {
	if o.ReservedENIs < 0 {
		return fmt.Errorf("reserved-enis cannot be negative")
	}
	return nil
}

func (o Options) validateUnavailableOfferingsTTL() error {
	if o.UnavailableOfferingsTTL < 0 {
		return fmt.Errorf("unavailable-offerings-ttl cannot be negative")
	}
	return nil
}

func (o Options) validateRequiredFields() error {
	if o.ClusterName == "" {
		return fmt.Errorf("missing field, cluster-name")
	}
	return nil
}
