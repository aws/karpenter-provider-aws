[![Build Status](https://img.shields.io/github/actions/workflow/status/aws/karpenter-core/presubmit.yaml?branch=main)](https://github.com/aws/karpenter-core/actions/workflows/presubmit.yaml)
![GitHub stars](https://img.shields.io/github/stars/aws/karpenter-core)
![GitHub forks](https://img.shields.io/github/forks/aws/karpenter-core)
[![GitHub License](https://img.shields.io/badge/License-Apache%202.0-ff69b4.svg)](https://github.com/aws/karpenter-core/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/aws/karpenter-core)](https://goreportcard.com/report/github.com/aws/karpenter-core)
[![Coverage Status](https://coveralls.io/repos/github/aws/karpenter-core/badge.svg?branch=main)](https://coveralls.io/github/aws/karpenter-core?branch=main)
[![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/aws/karpenter-core/issues)

# Karpenter

Karpenter improves the efficiency and cost of running workloads on Kubernetes clusters by:

* **Watching** for pods that the Kubernetes scheduler has marked as unschedulable
* **Evaluating** scheduling constraints (resource requests, nodeselectors, affinities, tolerations, and topology spread constraints) requested by the pods
* **Provisioning** nodes that meet the requirements of the pods
* **Removing** the nodes when the nodes are no longer needed

## Karpenter Implementations
Karpenter is a multi-cloud project with implementations by the following cloud providers:
- [AWS](https://github.com/aws/karpenter-provider-aws)
- [Azure](https://github.com/Azure/karpenter-provider-azure)
- [AlibabaCloud](https://github.com/cloudpilot-ai/karpenter-provider-alibabacloud)
- [Bizfly Cloud](https://github.com/bizflycloud/karpenter-provider-bizflycloud)
- [Cluster API](https://github.com/kubernetes-sigs/karpenter-provider-cluster-api)
- [GCP](https://github.com/cloudpilot-ai/karpenter-provider-gcp)
- [IBM Cloud](https://github.com/kubernetes-sigs/karpenter-provider-ibm-cloud)
- [Proxmox](https://github.com/sergelogvinov/karpenter-provider-proxmox)
- [Oracle Cloud Infrastructure (OCI)](https://github.com/zoom/karpenter-oci)

## Community, discussion, contribution, and support

If you have any questions or want to get the latest project news, you can connect with us in the following ways:
- __Using and Deploying Karpenter?__ Reach out in the [#karpenter](https://kubernetes.slack.com/archives/C02SFFZSA2K) channel in the [Kubernetes slack](https://slack.k8s.io/) to ask questions about configuring or troubleshooting Karpenter.
- __Contributing to or Developing with Karpenter?__ Join the [#karpenter-dev](https://kubernetes.slack.com/archives/C04JW2J5J5P) channel in the [Kubernetes slack](https://slack.k8s.io/) to ask in-depth questions about contribution or to get involved in design discussions.

### Working Group Meetings
Bi-weekly meetings alternating between Thursdays @ 9:00 PT ([convert to your timezone](http://www.thetimezoneconverter.com/?t=9:00&tz=Seattle)) and Thursdays @ 15:00 PT ([convert to your timezone](http://www.thetimezoneconverter.com/?t=15:00&tz=Seattle))

### Issue Triage Meetings
Weekly meetings alternating between repositories and time slots. Please check the calendar invite for specific dates:

**kubernetes-sigs/karpenter**:
- Alternating Mondays @ 9:00 PT ([convert to your timezone](http://www.thetimezoneconverter.com/?t=9:00&tz=Seattle)) and @ 15:00 PT [convert to your timezone](http://www.thetimezoneconverter.com/?t=15:00&tz=Seattle) monthly

**aws/karpenter-provider-aws**:
- Alternating Mondays @ 9:00 PT ([convert to your timezone](http://www.thetimezoneconverter.com/?t=9:00&tz=Seattle)) and @ 15:00 PT [convert to your timezone](http://www.thetimezoneconverter.com/?t=15:00&tz=Seattle) monthly

#### Meeting Resources
- **Zoom Link**: [Join Meeting](https://zoom.us/j/95618088729) (password: 77777)
- **Calendar**: Subscribe to our [Google Calendar](https://calendar.google.com/calendar/u/0?cid=N3FmZGVvZjVoZWJkZjZpMnJrMmplZzVqYmtAZ3JvdXAuY2FsZW5kYXIuZ29vZ2xlLmNvbQ)
- **Meeting Notes**: View our [Working Group Log](https://docs.google.com/document/d/18BT0AIMugpNpiSPJNlcAL2rv69yAE6Z06gUVj7v_clg/edit?usp=sharing)

Pull Requests and feedback on issues are very welcome!
See the [issue tracker](https://github.com/aws/karpenter-core/issues) if you're unsure where to start, especially the [Good first issue](https://github.com/aws/karpenter-core/issues?q=is%3Aopen+is%3Aissue+label%3Agood-first-issue) and [Help wanted](https://github.com/aws/karpenter-core/issues?utf8=%E2%9C%93&q=is%3Aopen+is%3Aissue+label%3Ahelp-wanted) tags, and
also feel free to reach out to discuss.

See also our [contributor guide](CONTRIBUTING.md) and the Kubernetes [community page](https://kubernetes.io/community) for more details on how to get involved.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

## Talks
- 09/08/2022 [Workload Consolidation with Karpenter](https://youtu.be/BnksdJ3oOEs)
- 05/19/2022 [Scaling K8s Nodes Without Breaking the Bank or Your Sanity](https://www.youtube.com/watch?v=UBb8wbfSc34)
- 03/25/2022 [Karpenter @ AWS Community Day 2022](https://youtu.be/sxDtmzbNHwE?t=3931)
- 12/20/2021 [How To Auto-Scale Kubernetes Clusters With Karpenter](https://youtu.be/C-2v7HT-uSA)
- 11/30/2021 [Karpenter vs Kubernetes Cluster Autoscaler](https://youtu.be/3QsVRHVdOnM)
- 11/19/2021 [Karpenter @ Container Day](https://youtu.be/qxWJRUF6JJc)
- 05/14/2021 [Groupless Autoscaling with Karpenter @ Kubecon](https://www.youtube.com/watch?v=43g8uPohTgc)
- 05/04/2021 [Karpenter @ Container Day](https://youtu.be/MZ-4HzOC_ac?t=7137)
