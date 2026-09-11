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

package nvidiadra

// GPUInfo is the instance-type-wide NVIDIA GPU metadata. Every GPU on an instance type is the same
// model, so the model attributes live here and only the PCI topology varies per device.
type GPUInfo struct {
	ProductName           string
	Type                  string
	Brand                 string
	Architecture          string
	CUDAComputeCapability string
	AddressingMode        string
	Memory                string
	Devices               []GPUDevice
}

// GPUDevice is the per-GPU topology, ordered by device index: the first entry describes "gpu-0".
// All three fields are fixed by the instance type's hardware layout, so they are safe to hard-code.
type GPUDevice struct {
	PCIBusID string
	PCIeRoot string
	NUMANode int64
}

// MIGCounterSet is the per-GPU budget that candidate MIG partitions draw down. The driver publishes
// one counter set per physical GPU; a partition consumes engines and memory plus the specific memory
// slices it occupies, which is what stops two overlapping placements being allocated at once.
type MIGCounterSet struct {
	Memory          string
	Multiprocessors int64
	CopyEngines     int64
	Decoders        int64
	Encoders        int64
	JPEGEngines     int64
	OFAEngines      int64
	// MemorySlices is the number of memory-slice counters, each with a value of 1.
	MemorySlices int64
}

// MIGProfile is one MIG profile and every position it can be placed in on a GPU. Each placement
// becomes its own candidate device, named "<NamePrefix>-<placement>", occupying SlicesPerPlacement
// memory slices starting at the placement index.
type MIGProfile struct {
	Profile            string
	NamePrefix         string
	Memory             string
	Multiprocessors    int64
	CopyEngines        int64
	Decoders           int64
	Encoders           int64
	JPEGEngines        int64
	OFAEngines         int64
	SlicesPerPlacement int64
	Placements         []int64
}

// MIGLayout is the set of candidate MIG partitions for one GPU model.
type MIGLayout struct {
	// ModeTogglable reports whether the driver advertises these partitions regardless of whether MIG
	// mode is currently on, which it does only when the GPU can switch MIG mode without a reset.
	// Hopper and later can; Ampere cannot, because switching needs a GPU reset that the driver's
	// kubelet plugin never initiates. So on Ampere the driver publishes either whole GPUs or
	// partitions depending on the node's boot-time MIG mode, and Karpenter cannot know which ahead of
	// launching the node. Only layouts with this set are safe to advertise unconditionally.
	ModeTogglable bool
	CounterSet    MIGCounterSet
	Profiles      []MIGProfile
}

// NVIDIA GPU metadata by instance type. Instance types absent from this map get no NVIDIA
// DynamicResources, and so are never picked for a DRA GPU request.
var GPUs = map[string]*GPUInfo{
	"p4d.24xlarge": {
		ProductName:           "NVIDIA A100-SXM4-40GB",
		Type:                  "gpu",
		Brand:                 "Nvidia",
		Architecture:          "Ampere",
		CUDAComputeCapability: "8.0.0",
		AddressingMode:        "HMM",
		Memory:                "40Gi",
		Devices: []GPUDevice{
			{PCIBusID: "0000:10:1c.0", PCIeRoot: "pci0000:10", NUMANode: 0},
			{PCIBusID: "0000:10:1d.0", PCIeRoot: "pci0000:10", NUMANode: 0},
			{PCIBusID: "0000:20:1c.0", PCIeRoot: "pci0000:20", NUMANode: 0},
			{PCIBusID: "0000:20:1d.0", PCIeRoot: "pci0000:20", NUMANode: 0},
			{PCIBusID: "0000:90:1c.0", PCIeRoot: "pci0000:90", NUMANode: 1},
			{PCIBusID: "0000:90:1d.0", PCIeRoot: "pci0000:90", NUMANode: 1},
			{PCIBusID: "0000:a0:1c.0", PCIeRoot: "pci0000:a0", NUMANode: 1},
			{PCIBusID: "0000:a0:1d.0", PCIeRoot: "pci0000:a0", NUMANode: 1},
		},
	},
	"p6-b200.48xlarge": {
		ProductName:           "NVIDIA B200",
		Type:                  "gpu",
		Brand:                 "Nvidia",
		Architecture:          "Blackwell",
		CUDAComputeCapability: "10.0.0",
		AddressingMode:        "HMM",
		Memory:                "182784Mi",
		Devices: []GPUDevice{
			{PCIBusID: "0000:51:00.0", PCIeRoot: "pci0000:44", NUMANode: 0},
			{PCIBusID: "0000:52:00.0", PCIeRoot: "pci0000:44", NUMANode: 0},
			{PCIBusID: "0000:62:00.0", PCIeRoot: "pci0000:55", NUMANode: 0},
			{PCIBusID: "0000:63:00.0", PCIeRoot: "pci0000:55", NUMANode: 0},
			{PCIBusID: "0000:75:00.0", PCIeRoot: "pci0000:66", NUMANode: 1},
			{PCIBusID: "0000:76:00.0", PCIeRoot: "pci0000:66", NUMANode: 1},
			{PCIBusID: "0000:86:00.0", PCIeRoot: "pci0000:79", NUMANode: 1},
			{PCIBusID: "0000:87:00.0", PCIeRoot: "pci0000:79", NUMANode: 1},
		},
	},
	"p5.48xlarge": {
		ProductName:           "NVIDIA H100 80GB HBM3",
		Type:                  "gpu",
		Brand:                 "Nvidia",
		Architecture:          "Hopper",
		CUDAComputeCapability: "9.0.0",
		AddressingMode:        "HMM",
		Memory:                "81559Mi",
		Devices: []GPUDevice{
			{PCIBusID: "0000:53:00.0", PCIeRoot: "pci0000:44", NUMANode: 0},
			{PCIBusID: "0000:64:00.0", PCIeRoot: "pci0000:55", NUMANode: 0},
			{PCIBusID: "0000:75:00.0", PCIeRoot: "pci0000:66", NUMANode: 0},
			{PCIBusID: "0000:86:00.0", PCIeRoot: "pci0000:77", NUMANode: 0},
			{PCIBusID: "0000:97:00.0", PCIeRoot: "pci0000:88", NUMANode: 1},
			{PCIBusID: "0000:a8:00.0", PCIeRoot: "pci0000:99", NUMANode: 1},
			{PCIBusID: "0000:b9:00.0", PCIeRoot: "pci0000:aa", NUMANode: 1},
			{PCIBusID: "0000:ca:00.0", PCIeRoot: "pci0000:bb", NUMANode: 1},
		},
	},
}

// MIG layouts by GPU product name. The partition layout is a property of the GPU model rather than
// the instance type, so every instance type carrying the same GPU shares one entry. Only populated
// for MIG-capable models; the driver advertises these only when the DynamicMIG feature gate is on
// and MIG mode is enabled on the hardware.
var MIGLayouts = map[string]*MIGLayout{
	// Ampere. Kept for reference, but never advertised: MIG mode cannot be switched without a GPU
	// reset, so the driver publishes whole GPUs or partitions depending on how the node booted.
	"NVIDIA A100-SXM4-40GB": {
		ModeTogglable: false,
		CounterSet: MIGCounterSet{
			Memory: "40320Mi", Multiprocessors: 98, CopyEngines: 7,
			Decoders: 5, Encoders: 0, JPEGEngines: 1, OFAEngines: 1, MemorySlices: 8,
		},
		Profiles: []MIGProfile{
			{
				Profile: "1g.5gb", NamePrefix: "mig-1g5gb-19",
				Memory: "4864Mi", Multiprocessors: 14, CopyEngines: 1,
				Decoders: 0, Encoders: 0, JPEGEngines: 0, OFAEngines: 0,
				SlicesPerPlacement: 1, Placements: []int64{0, 1, 2, 3, 4, 5, 6},
			},
			{
				Profile: "1g.5gb+me", NamePrefix: "mig-1g5gb-me-20",
				Memory: "4864Mi", Multiprocessors: 14, CopyEngines: 1,
				Decoders: 1, Encoders: 0, JPEGEngines: 1, OFAEngines: 1,
				SlicesPerPlacement: 1, Placements: []int64{0, 1, 2, 3, 4, 5, 6},
			},
			{
				Profile: "1g.10gb", NamePrefix: "mig-1g10gb-15",
				Memory: "9984Mi", Multiprocessors: 14, CopyEngines: 1,
				Decoders: 1, Encoders: 0, JPEGEngines: 0, OFAEngines: 0,
				SlicesPerPlacement: 2, Placements: []int64{0, 2, 4, 6},
			},
			{
				Profile: "2g.10gb", NamePrefix: "mig-2g10gb-14",
				Memory: "9984Mi", Multiprocessors: 28, CopyEngines: 2,
				Decoders: 1, Encoders: 0, JPEGEngines: 0, OFAEngines: 0,
				SlicesPerPlacement: 2, Placements: []int64{0, 2, 4},
			},
			{
				Profile: "3g.20gb", NamePrefix: "mig-3g20gb-9",
				Memory: "20096Mi", Multiprocessors: 42, CopyEngines: 3,
				Decoders: 2, Encoders: 0, JPEGEngines: 0, OFAEngines: 0,
				SlicesPerPlacement: 4, Placements: []int64{0, 4},
			},
			{
				Profile: "4g.20gb", NamePrefix: "mig-4g20gb-5",
				Memory: "20096Mi", Multiprocessors: 56, CopyEngines: 4,
				Decoders: 2, Encoders: 0, JPEGEngines: 0, OFAEngines: 0,
				SlicesPerPlacement: 4, Placements: []int64{0},
			},
			{
				Profile: "7g.40gb", NamePrefix: "mig-7g40gb-0",
				Memory: "40320Mi", Multiprocessors: 98, CopyEngines: 7,
				Decoders: 5, Encoders: 0, JPEGEngines: 1, OFAEngines: 1,
				SlicesPerPlacement: 8, Placements: []int64{0},
			},
		},
	},
	// Hopper. Captured from a p5.48xlarge whose GPUs all reported MIG mode Disabled, where the driver
	// published these 25 placements per GPU alongside the 8 whole GPUs.
	"NVIDIA H100 80GB HBM3": {
		ModeTogglable: true,
		CounterSet: MIGCounterSet{
			Memory: "81152Mi", Multiprocessors: 132, CopyEngines: 8,
			Decoders: 7, Encoders: 0, JPEGEngines: 7, OFAEngines: 1, MemorySlices: 8,
		},
		Profiles: []MIGProfile{
			{
				Profile: "1g.10gb", NamePrefix: "mig-1g10gb-19",
				Memory: "9984Mi", Multiprocessors: 16, CopyEngines: 1,
				Decoders: 1, Encoders: 0, JPEGEngines: 1, OFAEngines: 0,
				SlicesPerPlacement: 1, Placements: []int64{0, 1, 2, 3, 4, 5, 6},
			},
			{
				Profile: "1g.10gb+me", NamePrefix: "mig-1g10gb-me-20",
				Memory: "9984Mi", Multiprocessors: 16, CopyEngines: 1,
				Decoders: 1, Encoders: 0, JPEGEngines: 1, OFAEngines: 1,
				SlicesPerPlacement: 1, Placements: []int64{0, 1, 2, 3, 4, 5, 6},
			},
			{
				Profile: "1g.20gb", NamePrefix: "mig-1g20gb-15",
				Memory: "20096Mi", Multiprocessors: 26, CopyEngines: 1,
				Decoders: 1, Encoders: 0, JPEGEngines: 1, OFAEngines: 0,
				SlicesPerPlacement: 2, Placements: []int64{0, 2, 4, 6},
			},
			{
				Profile: "2g.20gb", NamePrefix: "mig-2g20gb-14",
				Memory: "20096Mi", Multiprocessors: 32, CopyEngines: 2,
				Decoders: 2, Encoders: 0, JPEGEngines: 2, OFAEngines: 0,
				SlicesPerPlacement: 2, Placements: []int64{0, 2, 4},
			},
			{
				Profile: "3g.40gb", NamePrefix: "mig-3g40gb-9",
				Memory: "40448Mi", Multiprocessors: 60, CopyEngines: 3,
				Decoders: 3, Encoders: 0, JPEGEngines: 3, OFAEngines: 0,
				SlicesPerPlacement: 4, Placements: []int64{0, 4},
			},
			{
				Profile: "4g.40gb", NamePrefix: "mig-4g40gb-5",
				Memory: "40448Mi", Multiprocessors: 64, CopyEngines: 4,
				Decoders: 4, Encoders: 0, JPEGEngines: 4, OFAEngines: 0,
				SlicesPerPlacement: 4, Placements: []int64{0},
			},
			{
				Profile: "7g.80gb", NamePrefix: "mig-7g80gb-0",
				Memory: "81152Mi", Multiprocessors: 132, CopyEngines: 8,
				Decoders: 7, Encoders: 0, JPEGEngines: 7, OFAEngines: 1,
				SlicesPerPlacement: 8, Placements: []int64{0},
			},
		},
	},
	// Blackwell. Captured from a p6-b200.48xlarge whose GPUs all reported MIG mode Disabled, where the
	// driver published these 25 placements per GPU alongside the 8 whole GPUs.
	"NVIDIA B200": {
		ModeTogglable: true,
		CounterSet: MIGCounterSet{
			Memory: "182784Mi", Multiprocessors: 148, CopyEngines: 16,
			Decoders: 7, Encoders: 0, JPEGEngines: 7, OFAEngines: 1, MemorySlices: 8,
		},
		Profiles: []MIGProfile{
			{
				Profile: "1g.23gb", NamePrefix: "mig-1g23gb-19",
				Memory: "20992Mi", Multiprocessors: 18, CopyEngines: 2,
				Decoders: 1, Encoders: 0, JPEGEngines: 1, OFAEngines: 0,
				SlicesPerPlacement: 1, Placements: []int64{0, 1, 2, 3, 4, 5, 6},
			},
			{
				Profile: "1g.23gb+me", NamePrefix: "mig-1g23gb-me-20",
				Memory: "20992Mi", Multiprocessors: 18, CopyEngines: 2,
				Decoders: 1, Encoders: 0, JPEGEngines: 1, OFAEngines: 1,
				SlicesPerPlacement: 1, Placements: []int64{0, 1, 2, 3, 4, 5, 6},
			},
			{
				Profile: "1g.45gb", NamePrefix: "mig-1g45gb-15",
				Memory: "45312Mi", Multiprocessors: 30, CopyEngines: 2,
				Decoders: 1, Encoders: 0, JPEGEngines: 1, OFAEngines: 0,
				SlicesPerPlacement: 2, Placements: []int64{0, 2, 4, 6},
			},
			{
				Profile: "2g.45gb", NamePrefix: "mig-2g45gb-14",
				Memory: "45312Mi", Multiprocessors: 36, CopyEngines: 4,
				Decoders: 2, Encoders: 0, JPEGEngines: 2, OFAEngines: 0,
				SlicesPerPlacement: 2, Placements: []int64{0, 2, 4},
			},
			{
				Profile: "3g.90gb", NamePrefix: "mig-3g90gb-9",
				Memory: "89Gi", Multiprocessors: 70, CopyEngines: 6,
				Decoders: 3, Encoders: 0, JPEGEngines: 3, OFAEngines: 0,
				SlicesPerPlacement: 4, Placements: []int64{0, 4},
			},
			{
				Profile: "4g.90gb", NamePrefix: "mig-4g90gb-5",
				Memory: "89Gi", Multiprocessors: 72, CopyEngines: 8,
				Decoders: 4, Encoders: 0, JPEGEngines: 4, OFAEngines: 0,
				SlicesPerPlacement: 4, Placements: []int64{0},
			},
			{
				Profile: "7g.180gb", NamePrefix: "mig-7g180gb-0",
				Memory: "182784Mi", Multiprocessors: 148, CopyEngines: 16,
				Decoders: 7, Encoders: 0, JPEGEngines: 7, OFAEngines: 1,
				SlicesPerPlacement: 8, Placements: []int64{0},
			},
		},
	},
}
