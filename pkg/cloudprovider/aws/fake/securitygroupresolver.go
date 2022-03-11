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

package fake

import (
	"context"
)

type SecurityGroupResolver struct {
	Filter func(filter map[string]string) ([]string, error)
}

func (c *SecurityGroupResolver) Get(ctx context.Context, filter map[string]string) ([]string, error) {
	return c.Filter(filter)
}

func DefaultSecurityGroupResolver() *SecurityGroupResolver {
	return &SecurityGroupResolver{
		Filter: func(filter map[string]string) ([]string, error) {
			return []string{"sg-99999999999999999"}, nil
		},
	}
}
