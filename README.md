![](docs/images/karpenter-banner.png)

Karpenter is a node lifecycle management solution. It observes incoming pods and launches the right instances for the situation. Instance selection decisions are intent based and driven by the specification of incoming pods, including resource requests and scheduling constraints.

It's responsible for:
- **Launching** nodes for unschedulable pods
- **Replacing** existing nodes to improve resource utilization
- **Upgrading** nodes with outdated kubelet versions
- **Draining** nodes gracefully before preemption
- **Terminating** nodes if no longer needed

For most use cases, the entirety of a cluster’s capacity can be managed by a single Karpenter [Provisioner](./docs/README.md). Availability zone, instance type, capacity type, machine image, and scheduling constraints are automatically determined by the controller using a combination of defaults and overrides. Additionally, you can define multiple Provisioners, enabling use cases like isolation, entitlements, and sharding.

Karpenter optimizes for scheduling latency and utilization efficiency using two complementary control loops. First, is the allocator, a fast-acting latency-sensitive controller responsible for ensuring that incoming pods are scheduled as quickly as possible. Second, is the reallocator, a slow-acting cost-sensitive controller that replaces nodes as pods requests and capacity prices shift over time. Together, they maximize the availability and efficiency of your cluster.

Check out the [FAQs](docs/FAQs.md) to learn more.

![](docs/images/karpenter-overview.png)

This is an early stage, experimental project built with ❤️ and is available as a **developer preview**. We're excited you are here - jump in, let us know what you think. We welcome contributions.
## Installation
Follow the setup recommendations of your cloud provider.
- [AWS](docs/aws)

### Quick Install - Controller + Dependencies
```
sh -c "$(curl -fsSL https://raw.githubusercontent.com/awslabs/karpenter/v0.1.3/hack/quick-install.sh)"
```

### Kubectl - Standalone
```
kubectl apply -f https://raw.githubusercontent.com/awslabs/karpenter/v0.1.3/releases/aws/manifest.yaml
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

## Terms
Karpenter is an early stage, experimental project that is currently maintained by AWS and available as a preview. We request that you do not use Karpenter for production workloads at this time. See details in our [terms](docs/TERMS.md).

## License
This project is licensed under the Apache-2.0 License.
