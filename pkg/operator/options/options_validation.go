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
	"slices"
	"time"

	"github.com/awslabs/operatorpkg/serrors"
	"go.uber.org/multierr"
)

func (o *Options) Validate() error {
	return multierr.Combine(
		o.validateEndpoint(),
		o.validatePricingEndpointRegion(),
		o.validateVMMemoryOverheadPercent(),
		o.validateReservedENIs(),
		o.validateRequiredFields(),
		o.validateAMIRefreshInterval(),
		o.validateSubnetRefreshInterval(),
		o.validateSecurityGroupRefreshInterval(),
	)
}

func (o *Options) validateAMIRefreshInterval() error {
	if o.AMIRefreshInterval < time.Minute {
		return fmt.Errorf("ami-refresh-interval must be at least 1m")
	}
	return nil
}

func (o *Options) validateSubnetRefreshInterval() error {
	if o.SubnetRefreshInterval < time.Minute {
		return fmt.Errorf("subnet-refresh-interval must be at least 1m")
	}
	return nil
}

func (o *Options) validateSecurityGroupRefreshInterval() error {
	if o.SecurityGroupRefreshInterval < time.Minute {
		return fmt.Errorf("security-group-refresh-interval must be at least 1m")
	}
	return nil
}

func (o *Options) validateEndpoint() error {
	if o.ClusterEndpoint == "" {
		return nil
	}
	endpoint, err := url.Parse(o.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return serrors.Wrap(fmt.Errorf("cluster endpoint URL is not valid"), "cluster-endpoint", o.ClusterEndpoint)
	}
	return nil
}

func (o *Options) validatePricingEndpointRegion() error {
	if o.PricingEndpointRegion == "" {
		return nil
	}
	regions := supportedPricingEndpointRegions()
	if !slices.Contains(regions, o.PricingEndpointRegion) {
		return serrors.Wrap(fmt.Errorf("pricing endpoint region must be one of %v", regions), "pricing-endpoint-region", o.PricingEndpointRegion)
	}
	return nil
}

func supportedPricingEndpointRegions() []string {
	return []string{
		"ap-south-1",
		"cn-northwest-1",
		"eu-central-1",
		"eu-isoe-west-1",
		"eusc-de-east-1",
		"us-east-1",
		"us-iso-east-1",
		"us-isob-east-1",
		"us-isof-south-1",
	}
}

func (o *Options) validateVMMemoryOverheadPercent() error {
	if o.VMMemoryOverheadPercent < 0 {
		return fmt.Errorf("vm-memory-overhead-percent cannot be negative")
	}
	return nil
}

func (o *Options) validateReservedENIs() error {
	if o.ReservedENIs < 0 {
		return fmt.Errorf("reserved-enis cannot be negative")
	}
	return nil
}

func (o *Options) validateRequiredFields() error {
	if o.ClusterName == "" {
		return fmt.Errorf("missing field, cluster-name")
	}
	return nil
}
