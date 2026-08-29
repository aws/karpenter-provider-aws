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

// GPUMetadataByInstanceType maps an EC2 instance type to its scraped NVIDIA GPU metadata.
var GPUMetadataByInstanceType = map[string]*DeviceMetadata{
	"g6.12xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:37")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:39")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:3b")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:3d")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
	"g6.16xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:4c")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
	"g6.24xlarge": {
		Count: 4,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:5e")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:60")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:62")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:64")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
	"g6.2xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:30")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
	"g6.48xlarge": {
		Count: 8,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:9e")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:a0")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:a2")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:a4")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:ad")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:af")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:b1")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:b3")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
	"g6.4xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:34")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
	"g6.8xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:35")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
	"g6.xlarge": {
		Count: 1,
		Devices: []DRADevice{
			{
				Attributes: map[string]DRADeviceAttribute{
					"architecture":                    {String: strPtr("Ada Lovelace")},
					"brand":                           {String: strPtr("Nvidia")},
					"cudaComputeCapability":           {Version: strPtr("8.9.0")},
					"productName":                     {String: strPtr("NVIDIA L4")},
					"resource.kubernetes.io/pcieRoot": {String: strPtr("pci0000:30")},
					"type":                            {String: strPtr("gpu")},
				},
				Capacity: map[string]string{
					"memory": "23034Mi",
				},
			},
		},
	},
}
