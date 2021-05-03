![Build Status](https://img.shields.io/github/workflow/status/awslabs/karpenter/CI/main)
![Go Report](https://goreportcard.com/badge/github.com/awslabs/karpenter)
![GitHub stars](https://img.shields.io/github/stars/awslabs/karpenter)
![GitHub forks](https://img.shields.io/github/forks/awslabs/karpenter)
[![GitHub License](https://img.shields.io/badge/License-Apache%202.0-ff69b4.svg)](https://github.com/awslabs/karpenter/blob/main/LICENSE)
[![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/awslabs/karpenter/issues)

![](docs/images/karpenter-banner.png)

Karpenter is a node lifecycle management solution. It observes incoming pods and launches the right instances for the situation. Instance selection decisions are intent based and driven by the specification of incoming pods, including resource requests and scheduling constraints.

It's responsible for:
- **Launching** nodes for unschedulable pods
- **Replacing** existing nodes to improve resource utilization
- **Terminating** nodes if outdated or no longer needed
- **Draining** nodes gracefully before preemption

For most use cases, the entirety of a cluster’s capacity can be managed by a single Karpenter [Provisioner](./docs/README.md). Availability zone, instance type, capacity type, machine image, and scheduling constraints are automatically determined by the controller using a combination of defaults and overrides. Additionally, you can define multiple Provisioners, enabling use cases like isolation, entitlements, and sharding.

Karpenter optimizes for scheduling latency and utilization efficiency using two complementary control loops. First, is the allocator, a fast-acting latency-sensitive controller responsible for ensuring that incoming pods are scheduled as quickly as possible. Second, is the reallocator, a slow-acting cost-sensitive controller that replaces nodes as pods requests and capacity prices shift over time. Together, they maximize the availability and efficiency of your cluster.

Come discuss Karpenter in the [#provider-aws channel](https://kubernetes.slack.com/archives/C0LRMHZ1T) in the [Kubernetes slack](https://slack.k8s.io/)!

*Note: Reallocation is still in development. Check out the [FAQs](docs/FAQs.md) and [Roadmap](docs/ROADMAP.md) to learn more.*

<img src="docs/images/karpenter-overview.jpg" width="50%" height="50%">

## Installation
Follow the setup recommendations of your cloud provider.
- [AWS](docs/aws)

### Quick Install - Controller + Dependencies
```
sh -c "$(curl -fsSL https://raw.githubusercontent.com/awslabs/karpenter/v0.2.3/hack/quick-install.sh)"
```

### Kubectl - Standalone
```
kubectl apply -f https://raw.githubusercontent.com/awslabs/karpenter/v0.2.3/releases/aws/manifest.yaml
```

### Helm - Standalone
```
helm repo add karpenter https://awslabs.github.io/karpenter/charts
helm install karpenter karpenter/karpenter
```

## Docs
- [API](docs/README.md)
- [FAQs](docs/FAQs.md)
- [Roadmap](docs/ROADMAP.md)
- [Examples](docs/aws/examples)
- [Working Group](docs/working-group)
- [Developer Guide](docs/DEVELOPER_GUIDE.md)
- [Contributing](docs/CONTRIBUTING.md)

## License
This project is licensed under the Apache-2.0 License.
