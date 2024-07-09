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

package v1beta1

import (
	"context"

	"knative.dev/pkg/apis"
)

// Since v1 is the hub conversion version, We will only need to implement conversion webhooks for v1

func (in *EC2NodeClass) ConvertTo(ctx context.Context, to apis.Convertible) error { return nil }

func (in *EC2NodeClass) ConvertFrom(ctx context.Context, from apis.Convertible) error { return nil }
