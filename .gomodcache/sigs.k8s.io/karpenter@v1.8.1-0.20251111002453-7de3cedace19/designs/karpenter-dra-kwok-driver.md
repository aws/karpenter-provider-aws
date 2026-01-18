# KWOK DRA Driver Design

## Summary
The upstream kubernetes/perf-tests repository includes a [DRA KWOK Driver](https://github.com/kubernetes/perf-tests/pull/3491/files), but it's designed for **ClusterLoader2 scale testing** with pre-created static nodes that cannot be used for Karpenter testing.

This design introduces a **Karpenter DRA KWOK Driver** - a mock DRA driver that acts on behalf of KWOK nodes created by Karpenter. When KWOK nodes register with the cluster, the driver creates ResourceSlices advertising fake GPU/device resources. This simulates what a real DRA driver (like NVIDIA GPU Operator) would do, but with fake devices for testing purposes. The driver watches for KWOK nodes and creates corresponding ResourceSlices based on either Node Overlay or ConfigMap configuration. The driver acts independently as a standard Kubernetes controller, ensuring ResourceSlices exist on the API server for both the scheduler and Karpenter's cluster state to discover.

### Workflow
1. **Test creates ResourceClaim** with device attribute selectors
2. **Test creates DRA pod** referencing the ResourceClaim
3. **Karpenter provisions KWOK node** in response to unschedulable pod
4. **Node registration triggers ResourceSlice creation** based on:
   - **Case 1:** Check for matching NodeOverlay with embedded ResourceSlice objects (future enhancement)
   - **Case 2:** Use ConfigMap mappings if no NodeOverlay matches
   - **Case 3:** Eventually cloudproviders will be able to provide potential ResourceSlice shapes through the InstanceType interface (Future TODO: implement a way for cloudproviders to inform our DRAKWOKDriver of those shapes).
5. **Kubernetes scheduler discovers ResourceSlices** and binds pod to node
6. **Pod successfully schedules** to the node with available DRA resources
7. **Test validates** node creation, ResourceSlice creation, pod scheduling, and Karpenter behavior
8. **Cleanup automatically removes** ResourceSlices when nodes are deleted

## Implementation

### Case 1: Node Overlay Integration
Tests **Karpenter's integrated DRA scheduling** where DRA device counts are known during the scheduling simulation via extended resources. NodeOverlay informs Karpenter about expected DRA device capacity during scheduling through extended resources, Provisioner includes these extended resources in NodeClaim templates, and Karpenter provisions nodes knowing they will have specific device counts. The driver then creates ResourceSlices with detailed device information matching the NodeOverlay's extended resource count.

**Example Node Overlay with DRA** (future API extension):
```yaml
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: gpu-dra-config
spec:
  weight: 10  # Higher weight for conflict resolution
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["g5.48xlarge"]
  capacity:
    karpenter.sh.dra-kwok-driver/device: "8"  # Custom extended resource for DRA devices
  # TODO: Extend NodeOverlay API to embed ResourceSlice templates
  resourceSlices:  # FUTURE: Embedded ResourceSlice objects (not yet implemented)
  - apiVersion: resource.k8s.io/v1
    kind: ResourceSlice
    spec:
      # nodeName will be filled in by driver when node is created
      driver: "karpenter.sh.dra-kwok-driver"
      devices:
      - name: "nvidia-h100-0"
        driver: "karpenter.sh.dra-kwok-driver"
        attributes:
          memory: "80Gi"
          compute-capability: "9.0"
          vendor: "nvidia"
      - name: "nvidia-h100-1"
        driver: "karpenter.sh.dra-kwok-driver"
        attributes:
          memory: "80Gi"
          compute-capability: "9.0"
          vendor: "nvidia"
      # ... (6 more devices for total of 8)
```

**How it works**:
1. **Test author defines NodeOverlay configuration**: "g5.48xlarge KWOK nodes should have 8x fake H100 GPUs" via ResourceSlices
2. **Driver watches for KWOK nodes**: When Karpenter creates a KWOK node with `instance-type: g5.48xlarge`
3. **NodeOverlay match found**: Driver checks for NodeOverlay with embedded ResourceSlice objects, finds matching configuration
4. **Driver creates ResourceSlice**: Acts as fake DRA driver using embedded ResourceSlice objects from NodeOverlay
5. **Scheduler sees configured devices**: ResourceSlices with fake devices become available for DRA pod scheduling
6. **Test validation**: Validates that the driver correctly provides DRA resources and enables successful pod scheduling

### Case 2: ConfigMap Fallback Configuration
Tests **DRA resource provisioning when no NodeOverlay configuration is found** - simulating scenarios where ResourceSlices exist on nodes but weren't defined through NodeOverlay configuration. This addresses when other out of band components manage nodes, partial NodeOverlay coverage (only some instance types configured), and 3rd party DRA driver integration (GPU operators working independently). The driver falls back to ConfigMap-based device configuration when no matching NodeOverlay is found, creating ResourceSlices that Karpenter must then discover and incorporate into future scheduling decisions. This ensures we correctly test that Karpenter successfully discovers ResourceSlices and schedules against them, even if they weren't defined on any NodeOverlays.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dra-kwok-configmap
  namespace: karpenter
data:
  config.yaml: |
    driver: "karpenter.sh.dra-kwok-driver"
    mappings:
    - name: "h100-nodes"
      nodeSelector:
        matchLabels:
          node.kubernetes.io/instance-type: "g5.48xlarge"
          kwok.x-k8s.io/node: "fake"
      resourceSlice:
        devices:
        - name: "nvidia-h100"
          count: 8
          attributes:
            memory: "80Gi"
            compute-capability: "9.0" 
            device_class: "gpu"
            vendor: "nvidia"
    - name: "fpga-nodes"
      nodeSelector:
        matchLabels:
          node.kubernetes.io/instance-type: "f1.2xlarge"
          kwok.x-k8s.io/node: "fake"
      resourceSlice:
        devices:
        - name: "xilinx-u250"
          count: 1
          attributes:
            memory: "16Gi"
            device_class: "fpga"
            vendor: "xilinx"
```

**How it works**:
1. **Test author defines ConfigMap configuration**: "g5.48xlarge KWOK nodes should have 8x fake H100 GPUs when no NodeOverlay is found"
2. **Driver watches for KWOK nodes**: When Karpenter creates a KWOK node with `instance-type: g5.48xlarge`
3. **No NodeOverlay match found**: Driver checks for NodeOverlay with embedded ResourceSlice objects, finds none, falls back to ConfigMap
4. **Driver creates ResourceSlice**: Acts as fake DRA driver using ConfigMap configuration
5. **Scheduler sees configured devices**: ResourceSlices with fake devices become available for DRA pod scheduling
6. **Test validation**: Validates that the driver correctly provides DRA resources and enables successful pod scheduling

## Directory Structure
```
karpenter/
├── dra-kwok-driver/                   
│   ├── main.go                        # Driver entry point                     
│   └── pkg/
│       ├── controller/
│       │   ├── controller.go          # Main controller logic
│       │   ├── nodeoverlay.go         # NodeOverlay parsing (Case 1)
│       │   ├── configmap.go           # ConfigMap parsing (Case 2)
│       │   └── resourceslice.go       # ResourceSlice operations
│       └── config/
│           └── types.go               # Configuration types
└── test/suites/integration/
    └── dra_kwok_test.go               # Our DRA KWOK integration tests
```
1. main.go starts the controller
2. controller.go receives KWOK node events
3. nodeoverlay.go tries to find matching NodeOverlay (Case 1)
4. If no match: configmap.go provides fallback config (Case 2)  
5. resourceslice.go creates/updates/deletes the ResourceSlices
6. types.go provides the data structures throughout