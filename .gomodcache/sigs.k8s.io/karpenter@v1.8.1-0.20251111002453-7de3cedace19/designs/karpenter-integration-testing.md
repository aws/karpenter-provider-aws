# Karpenter - Integration Testing RFC

## Summary

Historically, the [`aws/karpenter-provider-aws`](https://github.com/aws/karpenter-provider-aws) repository has served as the primary testing ground for Karpenter's core functionality, with comprehensive integration tests covering provisioning, scheduling, disruption handling, networking configurations, and AWS-specific feature implementations. The [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter) repository has contained limited integration testing, primarily utilizing the KWOK (Kubernetes Without Kubelet) Provider with `KWOKNodeClass` for node simulation to validate basic provisioning and deprovisioning functionality. To best support specific Karpenter provider implementations and to reduce the toil of test duplication, this document identifies core common tests that should be migrated from [`aws/karpenter-provider-aws`](https://github.com/aws/karpenter-provider-aws) provider implementation into [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter). 

The test migration will help provider implementers by reducing the pain points of having to be aware of all functionality changes in [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter), having to continually update individual tests within their repositories, and by specifying a common set of features and capabilities that are expected from all Karpenter provider implementations.  As the project evolves, both the design requirements and this documentation will be updated to reflect current best practices and project needs.

The testing framework for [`aws/karpenter-provider-aws`](https://github.com/aws/karpenter-provider-aws) has several types of tests:

* Unit Tests
    * Run in a mock Kubernetes environment
    * Part of the regular developer workflow
    * Runs as a pre-merge CI check on GitHub

* Integration Tests
    * Create EKS clusters and interact with a live API server and AWS APIs 
    * Runs against every commit merged to `main` and release branches

* Scale Tests
    * Will validate Karpenter's performance at large scales
    * Focus on horizontal scaling workloads and resource provisioned by Karpenter
    * Runs once a day to measures benchmarks for Karpenter provisioning and deprovisioning nodes

* Soak Tests 
    * Designed to identify resource usage regressions
    * Will target memory leaks and long-running stability issues
    * Runs head commit of the  [`aws/karpenter-provider-aws`](https://github.com/aws/karpenter-provider-aws) for 7 days on an hour interval.

The [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter) repository mainly utilizes unit tests to validate functionality. 

## Problem 

The testing architecture in the Karpenter ecosystem faces two critical challenges in its testing framework: having a set of tests for all core functionality and having a standard set of tests for all providers. The [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter) repository lacks comprehensive integration tests to validate core functionality across code changes, making it difficult to ensure that fundamental Karpenter behaviors remain intact as the codebase evolves.

Additionally, cloud provider implementations of Karpenter lack a standardized conformance testing framework, meaning there's no systematic way to verify that provider-specific implementations maintain compatibility with [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter)'s core requirements and expectations. It's worth noting that historically the [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter) repository relied on tests running within [`aws/karpenter-provider-aws`](https://github.com/aws/karpenter-provider-aws). 

This document defines the appropriate placement of different test types across repositories, establishes clear testing responsibilities for both [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter) and cloud provider implementations, and provides a conformance testing framework that cloud providers can use to validate their implementations against [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter)'s requirements.

## Solution

The [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter) repository will serve as the primary location for a common test suite. The repository will contain tests for the majority of core functionality validation, including CEL validation for NodePool and NodeClaim. This repository will also provide base integration tests that cloud providers can consume and adapt by injecting their own NodeClass implementations to verify compatibility with core functionality. Any additional validation rules introduced by cloud providers must be tested within their respective provider repositories. The responsibility boundary will be drawn as such: 

#### kubernetes-sigs/karpenter

* Environment: Kind clusters with KWOK Karpenter Controller for simulated node provisioning
* Test Coverage:
    * NodePool Configuration
    * Standalone NodeClaim Configuration
    * Provisioning
    * Scheduling
    * Disruption Management
        * Consolidation
        * Drift (including NodePool changes)
        * Expiration
        * Emptiness
        * Node Repair Termination
    * Termination
    * Storage
    * Scale

#### karpenter Cloud Provider

*  Environment: Non-Kind Cluster with in-cluster Cloud Provider Karpenter Controller
* Test Coverage:
    * OS Management
    * NodeClass Configuration
    * Disruption Handling
        * Drift (NodeClass)
        * Node Repair Conditions
        * Forceful Disruption Operations


As part of the initial functional integration testing, the cloud-agnostic tests from [`aws/karpenter-provider-aws`](https://github.com/aws/karpenter-provider-aws) will be migrated to [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter), where they will be made available for use by any cloud provider. The migration will focus on provider-neutral test cases including core provisioning behaviors, scaling operations, configuration validation, and general controller functionality. This consolidation will establish a standardized testing framework while reducing duplicate code across different provider implementations. By centralizing these common test scenarios, we can ensure consistent validation of critical functionality across all cloud providers while maintaining clear documentation and portable test utilities such as:

##### Chaos 

* High Level Description: Validate that karpenter does not have any runaway scaling during provisioning or consolidation
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/chaos
* Test Cases:
    * Should not produce a runaway scale-up when consolidation is enabled
    * Should not produce a runaway scale-up when emptiness is enabled 

##### Consolidation

* High Level Description: Validate the basic consolidation action works as expected
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/consolidation
* Test Cases: 
    * LastPodEventTime
        * should update lastPodEventTime when pods are scheduled and removed
        * should update lastPodEventTime when pods go terminal
    * Budgets
        * should respect budgets for empty delete consolidation
        * should respect budgets for non-empty delete consolidation
        * should respect budgets for non-empty replace consolidation
        * should not allow consolidation if the budget is fully blocking
        * should not allow consolidation if the budget is fully blocking during a scheduled time
        * should consolidate nodes (delete)
            * if the nodes are on-demand nodes
            * if the nodes are spot nodes
        * should consolidate nodes (replace)
            * if the nodes are on-demand nodes
            * if the nodes are spot nodes
        * should consolidate on-demand nodes to spot (replace)
    * Capacity Reservation 
        * should consolidate into a reserved offering
        * should consolidate between reserved offerings


##### Drift

* High Level Description: Validate the expected drift functionality of each field and expected case to limit drift
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/drift
* Test Cases: 
    * Budgets 
        * should respect budgets for empty drift
        * should respect budgets for non-empty delete drift 
        * should respect budgets for non-empty replace drift 
        * should not allow drift if the budgets is fully blocking 
        * should not allow drift if te budget is fully blocking during a scheduled time 
    * NodePool
        * Static Drift 
            * Annotations 
            * Labels 
            * Taints 
            * Start-up Taints 
            * Node Requirement
    * Drift Failure 
        * should not disrupt a drifted node if the replacement node never registers 
        * should not disrupt a drifted node if the replacement node registers but never initialized 
        * should not drift any nodes if their pod disruption budgets are unhealthy 

##### Expiration

* High Level Description: Validate expiration operation for karpenter. This should validate that nodepool disruption budget are not respected, however PDBs are respected   
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/expiration 
* Test Cases: 
    * should expire the node after the expiration is reached
    * should replace expired node with a single node and schedule all pods 

##### Integration

* High Level Description: Multiple functionality are tested. This is the catch all suite for testing karpenter
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/integration
* Test Cases: 
    * DaemonSet
        * should account for LimitRange Default on daemonSet pods for resources
        * should account for LimitRange DefaultRequest on daemonSet pods for resources
    * Hash
        * should have NodePool hash
    * Repair Policy
        * should ignore disruption budgets
        * should ignore do-not-disrupt annotation on node
        * should ignore terminationGracePeriod on the nodepool
    * Utilization
        * should provision one pod per node
    * Validation 
        * NodePool
            * should error when a restricted label is used in labels (karpenter.sh/nodepool)
            * should error when a restricted label is used in labels (kubernetes.io/custom-label)
            * should allow a restricted label exception to be used in labels (node-restriction.kubernetes.io/custom-label)
            * should allow a restricted label exception to be used in labels ([*].node-restriction.kubernetes.io/custom-label)
            * should error when a requirement references a restricted label (karpenter.sh/nodepool)
            * should error when a requirement uses In but has no values
            * should error when a requirement uses an unknown operator
            * should error when Gt is used with multiple integer values
            * should error when Lt is used with multiple integer values
            * should error when consolidateAfter is negative
            * should succeed when ConsolidationPolicy=WhenEmptyOrUnderutilized is used with consolidateAfter
            * should error when minValues for a requirement key is negative
            * should error when minValues for a requirement key is zero
            * should error when minValues for a requirement key is more than 50
            * should error when minValues for a requirement key is greater than the values specified within In operator

##### Nodeclaim

* High Level Description: Validate that action taken by the nodeclaim controllers. This validates standalone claims and the garbage collection mechanisms  
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/nodeclaim
* Test Cases: 
    * Standalone NodeClaim  
        * should create a standard NodeClaim based on resource requests
        * should create a NodeClaim propagating all NodeClaim spec details
        * should remove the cloudProvider NodeClaim when the cluster NodeClaim is deleted
        * should delete a NodeClaim from the node termination finalizer 
        * should delete a NodeClaim after the registration timeout when the node doesn’t register
        * should delete a NodeClaim if it references a NodeClass that doesn’t exist
        * should delete a NodeClaim if it references a NodeClass that isn’t Ready 
    * Garbage Collection 
        * should succeed to garbage collect an Instance that was launched by a NodeClaim but has no Instance mapping 
        * should succeed to garbage collect an Instance that was deleted without cluster’s knowledge 

##### Scheduling

* High Level Description: Validating the scheduling ability of Karpenter. This will look at the cloud provider injected labels as well as the cloud agonistic labels.
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/scheduling
* Test Cases: 
    * should apply annotations to the node
    * Labels
        * should support restricted label domain exceptions
            * node-restriction.kuberentes.io
            * node.kubernetes.io
            * kops.k8s.io
    * Provisioning
        * should provision a node for naked pods
        * should provision a node for a deployment
        * should provision a node for a self-affinity deployment
        * should provision three nodes for a zonal topology spread
        * should provision a node using a NodePool with higher priority
        * should provision a right-sized node when a pod has InitContainers (cpu)
            * sidecar requirements + later init requirements do exceed container requirements
            * sidecar requirements + later init requirements do not exceed container requirements
            * init container requirements exceed all later requests
        * should provision a right-sized node when a pod has InitContainers (mixed resources)
        * should provision a node for a pod with overlapping zone and zone-id requirements
        * should provision nodes for pods with zone-id requirements in the correct zo

##### Storage 

* High Level Description: Validate the behaviors around using PVC and statefulsets. Karpenter does special handling in scheduling these pods, consolidating, and termination of these pods
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/storage
* Test Cases: 
    * Static
        * should run a pod with a pre-bound persistent volume (empty storage class)
        * should run a pod with a pre-bound persistent volume (non-existent storage class)
        * should run a pod with a pre-bound persistent volume while respecting topology constraints
        * should run a pod with a generic ephemeral volume
    * Dynamic
        * should run a pod with a dynamic persistent volume
        * should run a pod with a dynamic persistent volume while respecting allowed topologies
        * should run a pod with a dynamic persistent volume while respecting volume limits
        * should run a pod with a generic ephemeral volume
    * Stateful Workloads
        * should run on a new node without 6+ minute delays when disrupted
        * should not block node deletion if stateful workload cannot be drained

##### Termination 

* High Level Description: Looks at forceful termination and termination grace period
* Source: https://github.com/aws/karpenter-provider-aws/tree/main/test/suites/termination
* Test Cases: 
    * Emptiness 
        * should not allow emptiness if the budget is fully blocking
        * should not allow emptiness if the budget is fully blocking during a scheduled time
        * should terminate an empty node
    * Termination Grace Period 
        * should delete pod with do-not-disrupt when it reaches its terminationGracePeriodSeconds
    * Termination 
        * should drain pods on a node in order

