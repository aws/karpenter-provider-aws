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

package v1alpha1

import (
	"context"
	"fmt"
	"net/url"

	"github.com/awslabs/karpenter/pkg/utils/functional"
	"knative.dev/pkg/apis"
)

func (c *Constraints) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		c.validateCapacityType(ctx),
		c.validateLaunchTemplate(ctx),
		c.validateSubnets(ctx),
		c.Cluster.Validate(ctx),
	)
}

func (c *Constraints) validateCapacityType(ctx context.Context) (errs *apis.FieldError) {
	capacityType, ok := c.Labels[CapacityTypeLabel]
	if !ok {
		return nil
	}
	capacityTypes := []string{CapacityTypeSpot, CapacityTypeOnDemand}
	if !functional.ContainsString(capacityTypes, capacityType) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", capacityType, capacityTypes), fmt.Sprintf("labels[%s]", CapacityTypeLabel)))
	}
	return errs
}

func (c *Constraints) validateLaunchTemplate(ctx context.Context) (errs *apis.FieldError) {
	// nothing to validate at the moment
	return errs
}

func (c *Constraints) validateSubnets(ctx context.Context) (errs *apis.FieldError) {
	if c.GetSubnetName() != nil && c.GetSubnetTagKey() != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(fmt.Sprintf("labels[%s]", SubnetNameLabel), fmt.Sprintf("labels[%s]", SubnetTagKeyLabel)))
	}
	return errs
}

func (c *Cluster) Validate(ctx context.Context) (errs *apis.FieldError) {
	if len(c.Name) == 0 {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if len(c.Endpoint) == 0 {
		errs = errs.Also(apis.ErrMissingField("endpoint"))
	} else {
		endpoint, err := url.Parse(c.Endpoint)
		// url.Parse() will accept a lot of input without error; make
		// sure it's a real URL
		if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not a valid URL", c.Endpoint), "endpoint"))
		}
	}
	return errs
}
