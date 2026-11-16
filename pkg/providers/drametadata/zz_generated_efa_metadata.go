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

package drametadata

import (
	resourcev1 "k8s.io/api/resource/v1"
)

// EFAMetadataByInstanceType maps an EC2 instance type to its scraped EFA metadata.
var EFAMetadataByInstanceType = map[string]*DeviceMetadata{
	"c5n.18xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c5n.9xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c5n.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6a.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6gn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6i.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6i.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6id.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6id.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6in.32xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c6in.metal": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7a.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7g.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7g.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7gd.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7gd.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7gn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7gn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c7i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8a.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8g.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8g.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gb.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gb.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gb.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gb.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gb.metal-48xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gd.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gd.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gd.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gd.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gn.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gn.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gn.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8gn.metal-48xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8i.96xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8i.metal-96xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8ib.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8ib.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8ib.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8ib.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8id.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8id.96xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8id.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8id.metal-96xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8in.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8in.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8in.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c8in.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c9g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c9g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c9gd.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"c9gd.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"dl2q.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"f2.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"g4dn.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g4dn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g4dn.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g4dn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(false)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:c2")},
				},
			},
		},
	},
	"g5.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g5.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g5.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g5.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g5.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"g6.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"g6.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
				},
			},
		},
	},
	"g6.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
		},
	},
	"g6.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:84")},
				},
			},
		},
	},
	"g6.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"g6e.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"g6e.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
				},
			},
		},
	},
	"g6e.24xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
		},
	},
	"g6e.48xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:84")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:84")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a5")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a5")},
				},
			},
		},
	},
	"g6e.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"g7.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"g7.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
		},
	},
	"g7.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a1")},
				},
			},
		},
	},
	"g7.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"g7e.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"g7e.24xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
		},
	},
	"g7e.48xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:84")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:95")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a6")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:b7")},
				},
			},
		},
	},
	"g7e.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"gr6.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"hpc6a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc6id.32xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc7a.12xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc7a.24xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc7a.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc7a.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc7g.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc7g.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc7g.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"hpc8a.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i3en.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i3en.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i3en.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i4g.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i4i.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i4i.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i7i.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i7i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i7i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i7ie.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i7ie.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i8g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i8g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i8ge.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"i8ge.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"im4gn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"inf1.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m5dn.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m5dn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m5n.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m5n.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m5zn.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m5zn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6a.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6i.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6i.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6id.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6id.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6idn.32xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6idn.metal": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6in.32xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m6in.metal": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7a.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7g.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7g.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7gd.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7gd.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m7i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8a.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8azn.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8azn.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8g.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8g.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gb.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gb.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gb.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gb.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gb.metal-48xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gd.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gd.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gd.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gd.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gn.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gn.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gn.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8gn.metal-48xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8i.96xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8i.metal-96xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8ib.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8ib.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8ib.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8ib.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8id.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8id.96xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8id.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8id.metal-96xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idb.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idb.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idb.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idb.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idn.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idn.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idn.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8idn.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8in.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8in.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8in.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m8in.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m9g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m9g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m9gd.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"m9gd.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"p3dn.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
				},
			},
		},
	},
	"p4d.24xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:10")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:20")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:90")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a0")},
				},
			},
		},
	},
	"p4de.24xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:10")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:20")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:90")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa0")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a0")},
				},
			},
		},
	},
	"p5.48xlarge": {
		Count: 32,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
		},
	},
	"p5.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
				},
			},
		},
	},
	"p5e.48xlarge": {
		Count: 32,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(2)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(3)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa1")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
				},
			},
		},
	},
	"p5en.48xlarge": {
		Count: 16,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5d")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5d")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5d")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5d")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:76")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:76")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:76")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:76")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:8f")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:8f")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:8f")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa2")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:8f")},
				},
			},
		},
	},
	"p6-b200.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:79")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:79")},
				},
			},
		},
	},
	"p6-b300.48xlarge": {
		Count: 16,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:4b")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:4b")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5a")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5a")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:69")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:69")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:78")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(0)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:78")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:87")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:87")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:96")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:96")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a5")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a5")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:b4")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/numaNode":                {IntValue: intPtr(1)},
					"dra.net/pciDevice":               {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciSubsystem":            {StringValue: strPtr("efa3")},
					"dra.net/pciVendor":               {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":                    {BoolValue: boolPtr(true)},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:b4")},
				},
			},
		},
	},
	"r5dn.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r5dn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r5n.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r5n.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6a.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6i.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6i.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6id.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6id.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6idn.32xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6idn.metal": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6in.32xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r6in.metal": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7a.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7g.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7g.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7gd.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7gd.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7iz.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r7iz.metal-32xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8a.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8a.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8g.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8g.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gb.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gb.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gb.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gb.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gb.metal-48xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gd.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gd.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gd.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gd.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gn.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gn.48xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gn.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8gn.metal-48xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8i.96xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8i.metal-96xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8ib.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8ib.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8ib.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8ib.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8id.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8id.96xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8id.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8id.metal-96xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idb.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idb.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idb.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idb.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idn.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idn.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idn.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8idn.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8in.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8in.96xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8in.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r8in.metal-96xl": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r9g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r9g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r9gd.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"r9gd.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"trn1.32xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"trn1n.32xlarge": {
		Count: 16,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"trn2.3xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"trn2.48xlarge": {
		Count: 16,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"u7i-12tb.224xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"u7i-6tb.112xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"u7i-8tb.112xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"u7in-16tb.224xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"u7in-24tb.224xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"u7in-32tb.224xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"vt1.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x2idn.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x2idn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x2iedn.32xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x2iedn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x2iezn.12xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x2iezn.metal": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8aedz.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8aedz.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8g.24xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8g.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8g.metal-24xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8g.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8i.48xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8i.64xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8i.96xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8i.metal-48xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
	"x8i.metal-96xl": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"dra.net/pciDevice": {StringValue: strPtr("Elastic Fabric Adapter (EFA)")},
					"dra.net/pciVendor": {StringValue: strPtr("Amazon.com, Inc.")},
					"dra.net/rdma":      {BoolValue: boolPtr(true)},
				},
			},
		},
	},
}
