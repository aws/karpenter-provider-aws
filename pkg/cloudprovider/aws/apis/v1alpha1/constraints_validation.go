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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"knative.dev/pkg/apis"
)

func (c *Constraints) Validate(ctx context.Context) (errs *apis.FieldError) {
	return c.AWS.validate(ctx).ViaField("provider")
}

func (a *AWS) validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		a.validateInstanceProfile(ctx),
		a.validateCapacityType(ctx),
		a.validateLaunchTemplate(ctx),
		a.validateSubnets(ctx),
		a.validateSecurityGroups(ctx),
		a.Cluster.Validate(ctx).ViaField("cluster"),
	)
}

func (a *AWS) validateCapacityType(ctx context.Context) (errs *apis.FieldError) {
	capacityTypes := []string{CapacityTypeSpot, CapacityTypeOnDemand}
	if !functional.ContainsString(capacityTypes, aws.StringValue(a.CapacityType)) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", aws.StringValue(a.CapacityType), capacityTypes), "capacityType"))
	}
	return errs
}

func (a *AWS) validateInstanceProfile(ctx context.Context)(errs *apis.FieldError) {
	if a.InstanceProfile == "" {
		errs = errs.Also(apis.ErrMissingField("instanceProfile"))
	}
	return errs
}

func (a *AWS) validateLaunchTemplate(ctx context.Context) (errs *apis.FieldError) {
	// nothing to validate at the moment
	return errs
}

func (a *AWS) validateSubnets(ctx context.Context) (errs *apis.FieldError) {
	if a.SubnetSelector == nil {
		errs = errs.Also(apis.ErrMissingField("subnetSelector"))
	}
	return errs
}

func (a *AWS) validateSecurityGroups(ctx context.Context) (errs *apis.FieldError) {
	if a.SecurityGroupsSelector == nil {
		errs = errs.Also(apis.ErrMissingField("securityGroupSelector"))
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
