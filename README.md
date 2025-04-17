[![CI](https://github.com/aws/karpenter-provider-aws/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/aws/karpenter/actions/workflows/ci.yaml)
![GitHub stars](https://img.shields.io/github/stars/aws/karpenter-provider-aws)
![GitHub forks](https://img.shields.io/github/forks/aws/karpenter-provider-aws)
[![GitHub License](https://img.shields.io/badge/License-Apache%202.0-ff69b4.svg)](https://github.com/aws/karpenter-provider-aws/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/aws/karpenter-provider-aws)](https://goreportcard.com/report/github.com/aws/karpenter)
[![Coverage Status](https://coveralls.io/repos/github/aws/karpenter-provider-aws/badge.svg?branch=main)](https://coveralls.io/github/aws/karpenter?branch=main)
[![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/aws/karpenter-provider-aws/issues)

![](website/static/banner.png)

Karpenter is an open-source node provisioning project built for Kubernetes.
Karpenter improves the efficiency and cost of running workloads on Kubernetes clusters by:

* **Watching** for pods that the Kubernetes scheduler has marked as unschedulable
* **Evaluating** scheduling constraints (resource requests, nodeselectors, affinities, tolerations, and topology spread constraints) requested by the pods
* **Provisioning** nodes that meet the requirements of the pods
* **Removing** the nodes when the nodes are no longer needed

Come discuss Karpenter in the [#karpenter](https://kubernetes.slack.com/archives/C02SFFZSA2K) channel, in the [Kubernetes slack](https://slack.k8s.io/) or join the [Karpenter working group](https://karpenter.sh/docs/contributing/working-group/) bi-weekly calls. If you want to contribute to the Karpenter project, please refer to the Karpenter docs.

Check out the [Docs](https://karpenter.sh/docs/) to learn more.

## Talks
- 03/19/2024 [Harnessing Karpenter: Transforming Kubernetes Clusters with Argo Workflows](https://www.youtube.com/watch?v=rq57liGu0H4)
- 12/04/2023 [AWS re:Invent 2023 - Harness the power of Karpenter to scale, optimize & upgrade Kubernetes](https://www.youtube.com/watch?v=lkg_9ETHeks)
- 09/08/2022 [Workload Consolidation with Karpenter](https://youtu.be/BnksdJ3oOEs)
- 05/19/2022 [Scaling K8s Nodes Without Breaking the Bank or Your Sanity](https://www.youtube.com/watch?v=UBb8wbfSc34)
- 03/25/2022 [Karpenter @ AWS Community Day 2022](https://youtu.be/sxDtmzbNHwE?t=3931)
- 12/20/2021 [How To Auto-Scale Kubernetes Clusters With Karpenter](https://youtu.be/C-2v7HT-uSA)
- 11/30/2021 [Karpenter vs Kubernetes Cluster Autoscaler](https://youtu.be/3QsVRHVdOnM)
- 11/19/2021 [Karpenter @ Container Day](https://youtu.be/qxWJRUF6JJc)
- 05/14/2021 [Groupless Autoscaling with Karpenter @ Kubecon](https://www.youtube.com/watch?v=43g8uPohTgc)
- 05/04/2021 [Karpenter @ Container Day](https://youtu.be/MZ-4HzOC_ac?t=7137)


# Fix for Issue
To address the issue of certain metrics not being auto-generated in the documentation, particularly with `karpenter_pods_scheduling_decision_duration_seconds`, you can follow these steps to diagnose and fix the problem:

### Diagnosis

1. **Check Metric Definitions**: Verify that the metric `karpenter_pods_scheduling_decision_duration_seconds` is correctly defined in the codebase. This involves checking the relevant module or file where metrics are registered.

2. **Ensure Auto-Generation Tools are Configured**: Confirm that the tools and scripts responsible for auto-generating documentation (e.g., custom scripts, `make docgen`) are correctly configured to include all metrics. This may involve checking for configurations or annotations that guide the documentation generation process.

3. **Review Version Control**: Since the issue mentions that these metrics were introduced at v1, ensure that any changes or updates made in version control after their introduction did not inadvertently exclude these metrics from the documentation generation process.

4. **Inspect Build and Documentation Pipeline**: Examine the build and documentation generation pipeline to ensure that it is capturing all necessary files and data. This might involve checking the `make` commands or scripts to ensure they are comprehensive.

### Code Fix

Assuming the issue lies in the configuration of the auto-generation process, here is a general approach to fix it:

1. **Update Metric Registration**: Ensure that the metric is properly registered in the codebase. It should follow the same pattern as other metrics that are correctly documented.

   ```go
   // Example metric registration
   DescribeHistogram("karpenter_pods_scheduling_decision_duration_seconds",
       "The duration of scheduling decisions.",
       table.WithKind("Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob"),
       table.WithPredicates("aws:nodegroupName"),
   )
   ```

2. **Configure Documentation Generation**: Update the documentation generation configuration to include the new metrics. This might involve modifying a configuration file or script used by `make docgen`.

   - If using a tool like `godoc`, ensure it is set to scan the directories where these metrics are defined.
   - If using a custom script, ensure it includes logic to find and document all metric registrations.

3. **Test the Fix**: After making changes, run the documentation generation process to verify that the metrics are now included.

   ```bash
   make docgen
   ```

4. **Verify Documentation**: Once the documentation is regenerated, verify that `karpenter_pods_scheduling_decision_duration_seconds` appears in the Metrics reference.

### Additional Considerations

- **Version Control**: If the metrics were introduced in an older version, ensure that any tagging or branching strategy used in version control does not exclude these metrics from the documentation process.
  
- **Community Feedback**: Engage with the community or maintainers if the issue persists, as they might provide insights or have encountered similar issues.

By following these steps, you should be able to diagnose and fix the issue of missing metrics in the auto-generated documentation.
