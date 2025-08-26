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

package instanceprofile

import (
	"fmt"
	"strings"

	"github.com/aws/smithy-go"
	cache "github.com/patrickmn/go-cache"
	"github.com/samber/lo"
)

type RoleNotFoundError struct {
	error
}

func IsRoleNotFoundError(err error) bool {
	_, ok := lo.ErrorsAs[RoleNotFoundError](err)
	return ok
}

// ToRoleNotFoundError converts a smithy.APIError returned by AddRoleToInstanceProfile to a RoleNotFoundError.
func ToRoleNotFoundError(err error) (error, bool) {
	if err == nil {
		return nil, false
	}
	apiErr, ok := lo.ErrorsAs[smithy.APIError](err)
	if !ok {
		return nil, false
	}
	if apiErr.ErrorCode() != "NoSuchEntity" {
		return nil, false
	}
	// Differentiate between the instance profile not being found, and the role.
	if !strings.Contains(apiErr.ErrorMessage(), "role") {
		return nil, false
	}
	return RoleNotFoundError{
		error: err,
	}, true
}

// roleCache is a wrapper around a go-cache for handling role not found errors returned by AddRoleToInstanceProfile.
type roleCache struct {
	*cache.Cache
}

func (c roleCache) Key(name string) string {
	// NOTE: : is an illegal character in both instance profile and role names, ensuring there isn't risk of collision
	return fmt.Sprintf("role:%s", name)
}

// HasError returns the last RoleNotFoundError encountered when attempting to add the given role to an instance profile.
func (c roleCache) HasError(name string) (error, bool) {
	if err, ok := c.Get(c.Key(name)); ok {
		return err.(error), true
	}
	return nil, false
}

func (c roleCache) SetError(name string, err error) {
	if !IsRoleNotFoundError(err) {
		panic("role cache only accepts RoleNotFoundErrors")
	}
	c.SetDefault(c.Key(name), err)
}
