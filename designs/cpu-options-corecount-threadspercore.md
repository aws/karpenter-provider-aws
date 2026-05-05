# EC2NodeClass `cpuOptions` for `coreCount` and `threadsPerCore`

## Context

Karpenter currently schedules using instance type capacity derived from EC2 defaults. AWS Launch Templates can apply CPU topology options (`coreCount`, `threadsPerCore`) at launch time. When these options reduce effective CPU (for example, disabling hyperthreading with `threadsPerCore: 1`), kubelet reports lower allocatable CPU than Karpenter assumed during provisioning.

This can produce scheduling mismatches where a NodeClaim is created for a pod, but the pod remains unschedulable on the launched node.

## Problem

When `EC2NodeClass` sets CPU options that change effective vCPU count:

- Karpenter may select instance types based on default vCPU
- kubelet reports reduced CPU after node boots
- pods can become unschedulable post-provisioning

This violates the expectation that Karpenter capacity calculations and kubelet-reported capacity are consistent.

## Goals

- Allow users to set `spec.cpuOptions.coreCount` and `spec.cpuOptions.threadsPerCore` on `EC2NodeClass`
- Apply CPU options to EC2 launch templates
- Adjust instance type CPU capacity calculations used by Karpenter to match configured CPU topology
- Preserve current behavior when `cpuOptions` is not set

## Non-Goals

- This proposal does not define additional cpuOptions fields (for example, nested virtualization)
- This proposal does not change NodePool requirement semantics

## Proposed API

Add the following optional fields to `EC2NodeClassSpec`:

- `cpuOptions.coreCount`
- `cpuOptions.threadsPerCore`

Validation:

- `coreCount` must be >= 1 when set
- `threadsPerCore` must be one of `{1,2}` when set

## Capacity Calculation

Introduce `adjustedCPU()` in instance type resolution.

Given EC2 defaults:

- `defaultVCpus`
- `defaultCores`
- `defaultThreadsPerCore`

Effective CPU is computed as:

- both set: `coreCount * threadsPerCore`
- only `threadsPerCore`: `defaultCores * threadsPerCore`
- only `coreCount`: `coreCount * defaultThreadsPerCore`
- neither set: `defaultVCpus` (existing behavior)

This value is used for:

- instance type capacity (`ResourceCPU`)
- kube-reserved CPU overhead calculation input

## Implementation Summary

- API: add `CPUOptions` to `EC2NodeClassSpec`
- Resolver: carry CPU options from `EC2NodeClass` to launch template + instance type resolution paths
- Launch template: map API fields to EC2 `CpuOptions`
- Instance types:
  - include CPU options in cache key to avoid stale capacity calculations
  - compute adjusted CPU capacity via `adjustedCPU()`
- Tests:
  - coverage for all `adjustedCPU()` combinations
  - launch template mapping tests
  - backward compatibility tests for nil options

## Backward Compatibility

No behavior changes when `spec.cpuOptions` is omitted.

## Risks and Tradeoffs

- Increasing API surface can overlap with future cpuOptions features
- CPU topology options may increase NodePool diversity and reduce consolidation opportunities

Mitigations:

- keep scope limited to `coreCount` and `threadsPerCore`
- preserve current defaults and behavior unless explicitly configured

## Why now

Users depend on this for:

- security hardening by disabling hyperthreading
- licensing models tied to physical cores
- deterministic CPU topology for compute-sensitive workloads

Correct capacity accounting is required to avoid post-provisioning scheduling failures.
