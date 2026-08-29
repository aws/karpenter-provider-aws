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

// THIS IS AN EXAMPLE FILE FOR TESTING DURING DEVELOPMENT AND MAY NOT BE CORRECT

package efadra

// PCI device descriptions, as reported by the driver.
const (
	PCIDeviceEFA = "Elastic Fabric Adapter (EFA)"
	PCIDeviceENA = "Elastic Network Adapter (ENA)"
)

// NetworkInfo is the instance-type-wide network device metadata.
type NetworkInfo struct {
	// PCIVendor is the same for every device on an instance type.
	PCIVendor string
	Devices   []NetworkDevice
}

// NetworkDevice is one PCI network device. Device names are derived from PCIAddress, matching the
// driver's convention, so they aren't stored.
type NetworkDevice struct {
	PCIAddress   string
	PCIDevice    string
	PCISubsystem string
	PCIeRoot     string
	NUMANode     int64
	RDMA         bool
	Type         string
}

// Network device metadata by instance type. Instance types absent from this map get no dra.net
// DynamicResources, and so are never picked for a DRA network request.
var Networks = map[string]*NetworkInfo{
	"p5.48xlarge": {
		PCIVendor: "Amazon.com, Inc.",
		Devices: []NetworkDevice{
			{PCIAddress: "0000:48:00.0", PCIDevice: PCIDeviceENA, PCISubsystem: "ec20", PCIeRoot: "pci0000:44", NUMANode: 0, RDMA: false, Type: "device"},
			{PCIAddress: "0000:4f:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:44", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:50:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:44", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:51:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:44", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:52:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:44", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:60:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:55", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:61:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:55", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:62:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:55", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:63:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:55", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:71:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:66", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:72:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:66", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:73:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:66", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:74:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:66", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:82:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:77", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:83:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:77", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:84:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:77", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:85:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:77", NUMANode: 0, RDMA: true},
			{PCIAddress: "0000:93:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:88", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:94:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:88", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:95:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:88", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:96:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:88", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:a4:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:99", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:a5:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:99", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:a6:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:99", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:a7:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:99", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:b5:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:aa", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:b6:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:aa", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:b7:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:aa", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:b8:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:aa", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:c6:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:bb", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:c7:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:bb", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:c8:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:bb", NUMANode: 1, RDMA: true},
			{PCIAddress: "0000:c9:00.0", PCIDevice: PCIDeviceEFA, PCISubsystem: "efa1", PCIeRoot: "pci0000:bb", NUMANode: 1, RDMA: true},
		},
	},
}
