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
	"k8s.io/apimachinery/pkg/api/resource"
)

// GPUMetadataByInstanceType maps an EC2 instance type to its scraped NVIDIA GPU metadata.
var GPUMetadataByInstanceType = map[string]*DeviceMetadata{
	"g4dn.12xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g4dn.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g4dn.2xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g4dn.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g4dn.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g4dn.metal": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:17")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:17")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:e6")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:e6")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:f3")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:f3")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g4dn.xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("Tesla T4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g5.12xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5.24xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5.2xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5.xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.6.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A10G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23028Mi")},
				},
			},
		},
	},
	"g5g.16xlarge": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g5g.2xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g5g.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g5g.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g5g.metal": {
		Count: 2,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0002:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("16Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0002:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("16Gi")},
				},
			},
		},
	},
	"g5g.xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Turing")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.5.0")},
					"productName":                     {StringValue: strPtr("NVIDIA T4G")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("15Gi")},
				},
			},
		},
	},
	"g6.12xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:37")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:39")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:3b")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:3d")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:4c")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6.24xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5e")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:60")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:62")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:64")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6.2xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:30")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:9e")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a0")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a2")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a4")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:ad")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:af")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:b1")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:b3")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:35")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6.xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:30")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"g6e.12xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:37")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:39")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:3b")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:3d")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6e.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:4c")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6e.24xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:62")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:64")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:68")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6e.2xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:2f")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6e.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:9d")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:9f")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a1")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a3")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:c5")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:c7")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:c9")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:cb")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6e.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:33")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6e.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:35")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6e.xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L40S")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:2f")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("46068Mi")},
				},
			},
		},
	},
	"g6f.2xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("NvidiaVWS")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4-6Q")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:30")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("6Gi")},
				},
			},
		},
	},
	"g6f.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("NvidiaVWS")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4-12Q")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("12Gi")},
				},
			},
		},
	},
	"g6f.large": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("NvidiaVWS")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4-3Q")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:30")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("3Gi")},
				},
			},
		},
	},
	"g6f.xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("NvidiaVWS")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4-3Q")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:30")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("3Gi")},
				},
			},
		},
	},
	"gr6.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"gr6.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:35")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("23034Mi")},
				},
			},
		},
	},
	"gr6f.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ada Lovelace")},
					"brand":                           {StringValue: strPtr("NvidiaVWS")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.9.0")},
					"productName":                     {StringValue: strPtr("NVIDIA L4-12Q")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:34")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("12Gi")},
				},
			},
		},
	},
	"p3dn.24xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Volta")},
					"brand":                           {StringValue: strPtr("Tesla")},
					"cudaComputeCapability":           {VersionValue: strPtr("7.0.0")},
					"productName":                     {StringValue: strPtr("Tesla V100-SXM2-32GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:00")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("32Gi")},
				},
			},
		},
	},
	"p4d.24xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:10")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:10")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:20")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:20")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:90")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:90")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a0")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-40GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a0")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("40320Mi")},
				},
			},
		},
	},
	"p4de.24xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:10")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:10")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:20")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:20")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:90")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:90")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a0")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Ampere")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("8.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA A100-SXM4-80GB")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a0")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("80Gi")},
				},
			},
		},
	},
	"p5.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
		},
	},
	"p5.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H100 80GB HBM3")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:24")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("81152Mi")},
				},
			},
		},
	},
	"p5e.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:77")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:88")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:99")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:aa")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:bb")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
		},
	},
	"p5en.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5d")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5d")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:76")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:76")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:8f")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Hopper")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("9.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA H200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:8f")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("140Gi")},
				},
			},
		},
	},
	"p6-b200.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:44")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:55")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:66")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:79")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.0.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B200")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:79")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("182784Mi")},
				},
			},
		},
	},
	"p6-b300.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:4b")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:5a")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:69")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:78")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:87")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:96")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:a5")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
			{
				Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
					"architecture":                    {StringValue: strPtr("Blackwell")},
					"brand":                           {StringValue: strPtr("Nvidia")},
					"cudaComputeCapability":           {VersionValue: strPtr("10.3.0")},
					"productName":                     {StringValue: strPtr("NVIDIA B300 SXM6 AC")},
					"resource.kubernetes.io/pcieRoot": {StringValue: strPtr("pci0000:b4")},
					"type":                            {StringValue: strPtr("gpu")},
				},
				Capacity: map[resourcev1.QualifiedName]resourcev1.DeviceCapacity{
					"memory": {Value: resource.MustParse("268Gi")},
				},
			},
		},
	},
}
