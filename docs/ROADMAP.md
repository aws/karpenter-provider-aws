# Roadmap
*Disclaimer: All items and owners are subject to change.*
## Releases

* v0.2
    - Date: Q2, 2021
    - Karpenter is ready to be used by early adopters meets a well known set of common use cases.
* v0.3
    - Date: Q3, 2021
    - Karpenter is ready to be used in non-production scenarios and supports a majority of known use cases.
* v0.4
    - Date: Q4, 2021
    - Karpenter is ready to be used in production and been rigorously tested for scale and performance.

| Component    | Feature                                                | Scope  | Owner           |
| ------------ | ------------------------------------------------------ | ------ | --------------- |
| Allocator    | Pack Multiple Pods per Node                            | v0.2   | prateekgogia    |
| Allocator    | High Availability, zone selection                      | v0.2   | ellistarn       |
| Reallocator  | Terminate nodes if unused for some TTL (5 minutes)     | v0.2   | njtran          |
| Allocator    | AWS: C, M, R Instance Family Support (General Purpose) | v0.2   | prateekgogia    |
| Allocator    | AWS: T Instance Family Support (Burstable)             | v0.3   | bwagner5        |
| Allocator    | Workload Isolation Support (taints, node selectors)    | v0.3   | ellistarn       |
| Allocator    | AWS: Spot Instance Types                               | v0.3   | bwagner5        |
| Allocator    | AWS: ARM Instance Types                                | v0.3   | jacobgabrielson |
| Allocator    | Accelerator Instance Types                             | v0.3   |                 |
| Termination  | Graceful node termination (cordon/drain)               | v0.3   | njtran          |
| Interruption | Instance interruption events                           | v0.4   |                 |
| Allocator    | High Availabiity Support (topology spread, affinity)   | v0.4   | prateekgogia    |
| Project      | AWS: Separate AWS Cloud Provider repository            | v0.4   | prateekgogia    |
| Project      | Scale Testing                                          | v0.4   | njtran          |
| Project      | Performance Testing                                    | v0.4   | jacobgabrielson |
| Project      | ARM Karpenter Binaries                                 | v0.4   |                 |
| Project      | Helm Charts                                            | v0.4   |                 |
| Allocator    | AWS: EBS Volumes launched in the correct zone          | Future |                 |
| Allocator    | Sophisticated binpacking heuristics                    | Future |                 |
| Allocator    | Mac                                                    | Future |                 |
| Allocator    | Windows                                                | Future |                 |
| Allocator    | HPC                                                    | Future |                 |
| Reallocator  | Design - Defragmentation                               | Future |                 |
