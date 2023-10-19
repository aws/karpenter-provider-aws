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

package settings

import (
	"fmt"
	"net/url"
	"time"

	"knative.dev/pkg/apis"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

func (s Settings) Validate() (errs *apis.FieldError) {
	return errs.Also(
		s.validateEndpoint(),
		s.validateTags(),
		s.validateVMMemoryOverheadPercent(),
		s.validateReservedENIs(),
		s.validateAssumeRoleDuration(),
	).ViaField("aws")
}

func (s Settings) validateAssumeRoleDuration() (errs *apis.FieldError) {
	if s.AssumeRoleDuration < time.Minute*15 {
		return errs.Also(apis.ErrInvalidValue("assumeRoleDuration cannot be less than 15 Minutes", "assumeRoleDuration"))
	}
	return nil
}

func (s Settings) validateEndpoint() (errs *apis.FieldError) {
	if s.ClusterEndpoint == "" {
		return nil
	}
	endpoint, err := url.Parse(s.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%q not a valid clusterEndpoint URL", s.ClusterEndpoint), "clusterEndpoint"))
	}
	return nil
}

func (s Settings) validateTags() (errs *apis.FieldError) {
	for k := range s.Tags {
		for _, pattern := range v1alpha1.RestrictedTagPatterns {
			if pattern.MatchString(k) {
				errs = errs.Also(errs, apis.ErrInvalidKeyName(k, "tags", fmt.Sprintf("tag contains a restricted tag matching the pattern %q", pattern.String())))
			}
		}
	}
	return errs
}

func (s Settings) validateVMMemoryOverheadPercent() (errs *apis.FieldError) {
	if s.VMMemoryOverheadPercent < 0 {
		return errs.Also(apis.ErrInvalidValue("cannot be negative", "vmMemoryOverheadPercent"))
	}
	return nil
}

func (s Settings) validateReservedENIs() (errs *apis.FieldError) {
	if s.ReservedENIs < 0 {
		return errs.Also(apis.ErrInvalidValue("cannot be negative", "reservedENIs"))
	}
	return nil
}
