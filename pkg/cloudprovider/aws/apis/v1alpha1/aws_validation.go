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

	"knative.dev/pkg/apis"
)

func (a *AWS) Validate(ctx context.Context) (errs *apis.FieldError) {
	return a.validate(ctx).ViaField("provider")
}

func (a *AWS) validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		a.validateInstanceProfile(),
		a.validateLaunchTemplate(),
		a.validateSubnets(),
		a.validateSecurityGroups(),
		a.validateTags(),
		a.Cluster.Validate(ctx).ViaField("cluster"),
	)
}

func (a *AWS) validateInstanceProfile() (errs *apis.FieldError) {
	if a.InstanceProfile == "" {
		errs = errs.Also(apis.ErrMissingField("instanceProfile"))
	}
	return errs
}

func (a *AWS) validateLaunchTemplate() (errs *apis.FieldError) {
	// nothing to validate at the moment
	return errs
}

func (a *AWS) validateSubnets() (errs *apis.FieldError) {
	if a.SubnetSelector == nil {
		errs = errs.Also(apis.ErrMissingField("subnetSelector"))
	}
	for key, value := range a.SubnetSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("subnetSelector['%s']", key)))
		}
	}
	return errs
}

func (a *AWS) validateSecurityGroups() (errs *apis.FieldError) {
	if a.SecurityGroupSelector == nil {
		errs = errs.Also(apis.ErrMissingField("securityGroupSelector"))
	}
	for key, value := range a.SecurityGroupSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("securityGroupSelector['%s']", key)))
		}
	}
	return errs
}

func (a *AWS) validateTags() (errs *apis.FieldError) {
	// Only checking for empty tag keys. Avoiding a check on number of tags (hard limit of 50) since that limit is shared by user
	// defined and Karpenter tags, and the latter could change over time.
	for key := range a.Tags {
		if key == "" {
			errs = errs.Also(apis.ErrInvalidValue("Empty tag keys are not supported", "tags"))
		}
	}
	return errs
}

func (c *Cluster) Validate(context.Context) (errs *apis.FieldError) {
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
