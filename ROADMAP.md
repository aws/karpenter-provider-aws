# Roadmap
*Disclaimer: All items and owners are subject to change.*
## Releases

* v0.2
    - Date: Q2, 2021
    - Karpenter meets a well known set of common use cases.
* v0.3
    - Date: Q3, 2021
    - Karpenter supports a majority of known use cases.
* v0.4
    - Date: Q4, 2021
    - Karpenter supports known use cases and has been rigorously tested for scale and performance.

| Feature                                                | Release | Owner           | Size   | Status |
| ------------------------------------------------------ | ------- | --------------- | ------ | ------ |
| Pack Multiple Pods per Node                            | v0.2    | prateekgogia    | Huge   | Done   |
| High Availability, zone selection                      | v0.2    | ellistarn       | Large  | Done   |
| Terminate nodes if unused for some TTL (5 minutes)     | v0.2    | njtran          | Medium | Done   |
| AWS: C, M, R Instance Family Support (General Purpose) | v0.2    | prateekgogia    | Medium | Done   |
| AWS: T Instance Family Support (Burstable)             | v0.3    | bwagner5        | Small  | Done   |
| Workload Isolation Support (taints, node selectors)    | v0.3    | ellistarn       | Large  | Done   |
| AWS: Spot Instance Types                               | v0.3    | bwagner5        | Small  | Done   |
| AWS: ARM Instance Types                                | v0.3    | jacobgabrielson | Small  | Done   |
| AWS: Accelerator Instance Types                        | v0.3    | etarn           | Small  | Done   |
| AWS: Launch Template Overrides                         | v0.3    | jacobgabrielson | Medium | Done   |
| AWS: Upgrade support for new nodes                     | v0.3    | etarn           | Small  |        |
| AWS: Subnet Discovery/Override                         | v0.3    |                 | Small  |        |
| AWS: Security Group Discovery/Override                 | v0.3    |                 | Small  |        |
| AWS: Upgrade support for existing nodes                | v0.3    |                 | Small  |        |
| Graceful node termination (cordon/drain)               | v0.3    | njtran          | Huge   |        |
| Scheduling: Topology Spread Constraints                | v0.4    |                 | Medium |        |
| Scheduling: Node Affinity                              | v0.4    |                 | Medium |        |
| Scheduling: Pod Affinity                               | v0.4    |                 | Medium |        |
| AWS: Separate AWS Cloud Provider repository            | v0.4    |                 | Small  |        |
| Testing: Integration                                   | v0.4    |                 | Medium |        |
| Testing: Scale                                         | v0.4    | njtran          | Large  |        |
| Testing: Performance                                   | v0.4    | jacobgabrielson | Large  |        |
| Release Automation                                     | v0.4    | bwagner5        | Medium |        |
| ARM Karpenter Binaries                                 | v0.4    | bwagner5        | Small  | Done   |
| Helm Charts                                            | v0.4    | ellistarn       | Medium | Done   |
| AWS: EBS Volumes launched in the correct zone          | Future  |                 | Small  |        |
| Sophisticated binpacking heuristics                    | Future  |                 | Huge   |        |
| AWS: Mac AMI                                           | Future  |                 | Medium |        |
| AWS: Windows AMI                                       | Future  |                 | Medium |        |
| AWS: HPC Instance Types                                | Future  |                 | Medium |        |
| AWS: EC2 Instance interruption                         | Future  |                 | Large  |        |
| Defragmentation                                        | Future  |                 | Huge   |        |
