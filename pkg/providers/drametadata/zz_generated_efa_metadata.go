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
}
