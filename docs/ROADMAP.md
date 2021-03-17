# Roadmap

## Releases

* v0.2
    - Date: Q2, 2021
    - Karpenter is ready to be used by early adopters meets a well known set of common use cases.
* v0.3
    - Date: Q3, 2021
    - Karpenter is ready to be used in non-production scenarios and supports a majority of known use cases.
* v0.4
    - Date: Q4, 2021
    - Karpenter is ready to be used in production and been rigorously tested for scale and performance

| Component    | Feature                                                | Scope  | Owner           |
| ------------ | ------------------------------------------------------ | ------ | --------------- |
| Allocator    | Pack Multiple Pods per Node                            | v0.2   | prateekgogia    |
| Allocator    | High Availability, zone selection                      | v0.2   | ellistarn       |
| Reallocator  | Terminate nodes if unused for some TTL (5 minutes)     | v0.2   | njtran          |
| Allocator    | AWS: C, M, R Instance Family Support (General Purpose) | v0.2   | prateekgogia    |
| Allocator    | AWS: T Instance Family Support (Burstable)             | v0.3   | bwagner5        |
| Allocator    | Workload isolation Support (taints)                    | v0.3   | ellistarn       |
| Allocator    | Workload isolation Support (node selectors)            | v0.3   | ellistarn       |
| Allocator    | High Availabiity Topology Spread                       | v0.3   | ellistarn       |
| Allocator    | Spot Instance Types                                    | v0.3   | bwagner5        |
| Allocator    | ARM Nodes                                              | v0.3   | jacobgabrielson |
| Termination  | Design - Graceful Termination                          | v0.3   | njtran          |
| Allocator    | EBS Volumes launched in the correct zone               | v0.4   | prateekgogia    |
| Allocator    | P/G Instance Family Support (Accelerators)             | v0.4   | bwagner5        |
| Upgrade      | Design - Node Upgrade                                  | v0.4   | njtran          |
| Interruption | Design - Spot Rebalance, Maintenance Events            | v0.4   | prateekgogia    |
| Project      | Separate AWS Cloud Provider repository                 | v0.4   | ellistarn       |
| Project      | Scale Testing                                          | v0.4   | njtran          |
| Project      | Performance Testing                                    | v0.4   | jacobgabrielson |
| Project      | ARM Karpenter Binaries                                 | v0.4   |                 |
| Project      | Helm Charts                                            | v0.4   |                 |
| Allocator    | Sophisticated binpacking heuristics                    | Future |                 |
| Allocator    | Mac                                                    | Future |                 |
| Allocator    | Windows                                                | Future |                 |
| Allocator    | HPC                                                    | Future |                 |
| Reallocator  | Design - Defragmentation                               | Future |                 |
