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
}
