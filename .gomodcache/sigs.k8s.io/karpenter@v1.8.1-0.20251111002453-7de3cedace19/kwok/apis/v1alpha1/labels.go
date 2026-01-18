/*
Copyright The Kubernetes Authors.

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
	"sigs.k8s.io/karpenter/kwok/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

const (
	// Labels that can be selected on and are propagated to the node
	InstanceSizeLabelKey   = apis.Group + "/instance-size"
	InstanceFamilyLabelKey = apis.Group + "/instance-family"
	InstanceMemoryLabelKey = apis.Group + "/instance-memory"
	InstanceCPULabelKey    = apis.Group + "/instance-cpu"

	// Internal labels that are propagated to the node
	KwokLabelKey          = "kwok.x-k8s.io/node"
	KwokLabelValue        = "fake"
	NodeViewerLabelKey    = "eks-node-viewer/instance-price"
	KwokPartitionLabelKey = "kwok-partition"
)

func init() {
	v1.RestrictedLabelDomains = v1.RestrictedLabelDomains.Insert(apis.Group)
	v1.WellKnownLabels = v1.WellKnownLabels.Insert(
		InstanceSizeLabelKey,
		InstanceFamilyLabelKey,
		InstanceCPULabelKey,
		InstanceMemoryLabelKey,
	)
}
