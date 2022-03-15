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

package cloudprovider

import (
	v1 "k8s.io/api/core/v1"
)

type ResourceFlags byte

const (
	// ResourceFlagNone is the default settings for resources where we normally scale up to larger nodes with no
	// restrictions.
	ResourceFlagNone ResourceFlags = iota

	// ResourceFlagMinimizeUsage indicates that instances which have this resource should not be considered for node
	// creation unless a pod specifically requests this resource type.  This is useful for preventing non-GPU workloads
	// from possibly scheduling to more expensive GPU instance type, or from causing a GPU instance type to scale up to
	// the next larger type due to a non-GPU workload
	ResourceFlagMinimizeUsage
)

var ResourceRegistration = map[v1.ResourceName]ResourceFlags{}
