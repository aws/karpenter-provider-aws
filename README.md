![Build Status](https://img.shields.io/github/workflow/status/awslabs/karpenter/CI/main)
![GitHub stars](https://img.shields.io/github/stars/awslabs/karpenter)
![GitHub forks](https://img.shields.io/github/forks/awslabs/karpenter)
[![GitHub License](https://img.shields.io/badge/License-Apache%202.0-ff69b4.svg)](https://github.com/awslabs/karpenter/blob/main/LICENSE)
[![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/awslabs/karpenter/issues)

![](website/static/banner.png)

> **❗Note:** Karpenter is in active development and should be considered **pre-production** software. Backwards incompatible API changes are possible in future releases and support is best-effort by the Karpenter community.

Karpenter is a node lifecycle management solution. It observes incoming pods and launches the right instances for the situation. Instance selection decisions are intent based and driven by the specification of incoming pods, including resource requests and scheduling constraints.

It's responsible for:
- **Launching** nodes for unschedulable pods
- **Replacing** existing nodes to improve resource utilization
- **Terminating** nodes if outdated or no longer needed
- **Draining** nodes gracefully before preemption

For most use cases, the entirety of a cluster’s capacity can be managed by a single Karpenter [Provisioner](README.md). Availability zone, instance type, capacity type, machine image, and scheduling constraints are automatically determined by the controller using a combination of defaults and overrides. Additionally, you can define multiple Provisioners, enabling use cases like isolation, entitlements, and sharding.

Karpenter optimizes for scheduling latency and utilization efficiency using two complementary control loops. First, is the allocator, a fast-acting latency-sensitive controller responsible for ensuring that incoming pods are scheduled as quickly as possible. Second, is the reallocator, a slow-acting cost-sensitive controller that replaces nodes as pods requests and capacity prices shift over time. Together, they maximize the availability and efficiency of your cluster.

Come discuss Karpenter in the [#provider-aws channel](https://kubernetes.slack.com/archives/C0LRMHZ1T) in the [Kubernetes slack](https://slack.k8s.io/)!

Check out the [FAQs](https://karpenter.sh/docs/faqs/) to learn more.

## Installation

Follow the setup recommendations of your cloud provider.
- [AWS](https://karpenter.sh/docs/cloud-providers/aws/)

> ❗ Note: There may be backwards incompatible changes between versions when upgrading before v0.3.0. Karpenter follows [Kubernetes versioning guidelines](https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-changes). Before upgrading, we recommend:
> - Check the [release notes](https://github.com/awslabs/karpenter/releases)
> - Uninstall Karpenter
> - Remove all nodes launched by karpenter
> - Reinstall Karpenter

## References
- [Docs](https://karpenter.sh/docs/)
- [API](README.md)
- [FAQs](https://karpenter.sh/docs/faqs/)
- [Working Group](WORKING_GROUP.md)
- [Developer Guide](https://karpenter.sh/docs/development-guide/)
- [Contributing](CONTRIBUTING.md)

## Talks
- [Karpenter @ Container Day, October 2021](https://youtu.be/3f0Tv7IiQQw?t=19028)
- [Karpenter @ Container Day, May 2021](https://youtu.be/MZ-4HzOC_ac?t=7137)
- [Groupless Autoscaling with Karpenter @ Kubecon, May 2021](https://www.youtube.com/watch?v=43g8uPohTgc)

## License
This project is licensed under the Apache-2.0 License.
