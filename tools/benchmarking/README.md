# Karpenter Benchmarking

This tool benchmarks Kubernetes deployments with a focus on Karpenter's node provisioning and deprovisioning capabilities. It creates deployments from YAML files, scales them to a specified number of pods, measures the time until all pods are ready, and then runs selected benchmark tests. It also tracks cost metrics for nodes throughout the process.

## Prerequisites

- Go 1.21 or later
- Access to a Kubernetes cluster with Karpenter installed
- `kubectl` configured with access to your cluster
- Instance types configuration file (for cost tracking)

## Installation

```bash
# Build the program
cd benchmarking
go build -o benchmark
```

## Usage

```bash
./benchmark --file=<path-to-deployment-yaml> --bench=<benchmark-name> --instance-types=<path-to-instance-types> [--namespace=<namespace>] [--replicas=<number-of-replicas>]
```

### Parameters

- `--file`: Path to the deployment YAML file (required)
- `--bench`: Name of the benchmark to run (required, options: "emptiness", "consolidation")
- `--instance-types`: Path to the instance types JSON file (required for cost tracking)
- `--namespace`: Kubernetes namespace for the deployment (default: "default")
- `--kubeconfig`: Path to the kubeconfig file (default: `~/.kube/config`)
- `--replicas`: Number of replicas to scale to (default: 100)

### Benchmark Types

1. **emptiness**: Scales the deployment down to 0 pods and waits for all pods to terminate and nodes to be cleared.
2. **consolidation**: Scales the deployment down to half the original number of replicas and monitors cost reduction over time.

### Deployment Types

The repository includes several deployment configurations:

1. **simple**: Basic deployment without special constraints
2. **anti-affinity**: Deployment with pod anti-affinity rules to force pods onto separate nodes
3. **zonal-tsc**: Deployment with topology spread constraints across zones

### Cloud Providers

The repository supports multiple cloud providers:

1. **AWS**: EC2 node classes and node pools with various CPU configurations
2. **Kwok**: Simulated node provider for testing

## Workflow

1. **Karpenter Validation**: The program first validates that Karpenter is running and restarts the deployment if needed
2. **Deployment Creation**: Creates or uses an existing deployment from the provided YAML file
3. **Scale Up**: Scales the deployment to the specified number of replicas
4. **Readiness Check**: Waits for all pods to be ready and records the time taken
5. **Node Tracking**: Identifies and tracks which nodes are running the deployment pods
6. **Benchmark Execution**: Runs the selected benchmark:
   - For **emptiness**: Scales to 0 pods and waits for all pods and nodes to be cleared
   - For **consolidation**: Scales to half the original replicas and monitors cost over time
7. **Cost Monitoring**: Collects cost data points every 5 seconds for up to 10 minutes or until cost reaches zero
8. **Metrics Collection**: Retrieves Prometheus metrics from Karpenter (if available on localhost:8080)
9. **Cleanup**: Removes the deployment, all associated pods, and any NodeClaims/Nodes created

## Cost Tracking

The program provides detailed cost tracking functionality:

- **Initial Cost**: Total cost when all pods are running
- **Cost Monitoring**: Collects cost data points every 5 seconds during the benchmark
- **Cost Analysis**: Calculates minimum, maximum, and average costs
- **Cost Reduction**: Percentage reduction from initial to final cost

### Instance Types File Format

The instance types file should be a JSON file mapping instance type names to their costs:

```json
{
  "t3.medium": 0.0416,
  "t3.large": 0.0832,
  "m5.xlarge": 0.192,
  ...
}
```

## Metrics Collection

The program collects Prometheus metrics from Karpenter on `localhost:8080/metrics`, including:
- **Scheduling Duration**: Time taken for scheduling simulations
- **Disruption Decisions**: Metrics about consolidation and disruption decisions
- **Resource Utilization**: CPU and memory usage metrics

To ensure metrics are available, you may need to port-forward the Karpenter service:
```bash
kubectl port-forward -n kube-system deployment/karpenter 8080:8080
```

## Cleanup Process

The program includes a comprehensive cleanup process that:

1. **Deployment Deletion**: Deletes the deployment with foreground propagation
2. **Pod Verification**: Ensures all pods associated with the deployment are removed
3. **NodeClaim Cleanup**: Removes any NodeClaims created by Karpenter
4. **Node Cleanup**: Removes any nodes if necessary
5. **Concurrent Deletion**: Uses concurrent operations (up to 10 at a time) for faster cleanup

The cleanup process runs automatically at the end of the benchmark and also in case of early termination or panic.

## Sample Output

```
Loading instance types from ../cloudproviders/Kwok/instance-types.json
Loaded 144 instance types from file
Creating deployment pause-deployment...
Scaling deployment to 5000 replicas...
Waiting for all pods to be ready...
All pods are ready! Time taken: 12m17.324179541s
Deployment is running on 1252 nodes
Total cost for nodes: 67.977299
Running test suite: consolidation
Scaling deployment down to 2500 replicas...
Monitoring cost for 10 minutes or until completely scaled down...
Current cost at 0s: 67.977299
Current cost at 5s: 67.977299
Current cost at 10s: 65.234567
Current cost at 15s: 62.123456
...
Current cost at 9m56s: 33.988650

Cost Data Summary:
==================
Total data points collected: 117

Summary:
Scale up time (0 to 5000 pods): 12m17.324179541s
Initial cost: 67.977299
Final cost: 33.988650
Minimum cost: 33.988650
Maximum cost: 67.977299
Average cost: 43.935302
Cost reduction: 50.00%

Metrics:
======================
karpenter_scheduler_scheduling_duration_seconds (Duration of scheduling simulations used for deprovisioning and provisioning in seconds.)
  controller=disruption [histogram (p50: 0.042, p90: 0.179, p99: 35.333)]= 695.3448898209987
  controller=provisioner [histogram (p50: 0.208, p90: 31.154, p99: 39.615)]= 1086.5978401940017
karpenter_disruption_evaluation_total (Number of disruption decision evaluations performed by Karpenter)
  controller=disruption method=consolidate_nodes [counter]= 15
  controller=disruption method=empty_nodes [counter]= 8
karpenter_disruption_actions_total (Number of disruption actions performed by Karpenter)
  controller=disruption action=delete method=consolidate [counter]= 625
  controller=disruption action=delete method=empty [counter]= 627

Cleaning up resources...
Deleting deployment pause-deployment in namespace default
Waiting for deployment to be deleted...
Checking for any remaining pods...
All deployment resources have been successfully cleaned up.
```

## Contributing

When adding new benchmarks:
1. Create a new file in the `benchmarking/bench/` directory
2. Implement the `Suite` interface
3. Add the new benchmark to the switch statement in `main.go`
4. Document the benchmark behavior in this README
