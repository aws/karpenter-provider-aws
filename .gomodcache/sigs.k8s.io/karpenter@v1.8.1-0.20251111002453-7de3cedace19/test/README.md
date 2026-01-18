# Karpenter Integration Testing

Karpenter leverages Github Actions to run our E2E test suites.

## Directories
- `./.github/workflows`: Workflow files run within this repository. Relevant files for E2E testing are prefixed with `e2e-`
- `./.github/actions/e2e`: Composite actions utilized by the E2E workflows
- `./test/suites`: Directories defining test suites
- `./test/pkg`: Common utilities and expectations
- `./test/hack`: Testing scripts

# Integrating Karpenter Tests for Cloud Providers

This guide outlines how cloud providers can integrate Karpenter tests into their provider-specific implementations.

## Overview

Karpenter's testing framework is designed to be extensible across different cloud providers. By integrating these tests into your provider implementation, you can ensure compatibility with Karpenter's core functionality while validating your provider-specific features.

## Prerequisites

- A working Karpenter provider implementation
- Go development environment
- Access to your cloud provider's resources for testing
- Kubernetes cluster for integration testing

## Test Integration Steps

### 1. Set Up Test Infrastructure

Create a dedicated test directory in your provider repository:

```bash
mkdir -p test/suites
mkdir -p test/pkg/environment/yourprovider
```

### 2. Define Default NodePool and NodeClass

You need to define default NodePool and NodeClass configurations that will be passed to the Karpenter test framework. These configurations will be used during test runs to create nodes in your cloud provider.

Create the following files in your test directory:

**Default NodePool (`test/pkg/environment/yourprovider/default_nodepool.yaml`)**:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
    consolidateAfter: Never
    budgets:
      - nodes: 100%
  limits:
    cpu: 1000
    memory: 1000Gi
  template:
    spec:
      expireAfter: Never
      requirements:
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
      nodeClassRef:
        group: karpenter.yourprovider.com
        kind: YourProviderNodeClass
        name: default
```

**Default NodeClass (`test/pkg/environment/yourprovider/default_providernodeclass.yaml`)**:

```yaml
apiVersion: karpenter.yourprovider.com/v1alpha1
kind: YourProviderNodeClass
metadata:
  name: default
spec:
  # Add your provider-specific configuration here
  # For example:
  # instanceType: t3.large
  # region: us-west-2
```

These files follow the same pattern that kubernetes-sigs/karpenter uses for its tests, as seen in the `test/pkg/environment/common` directory.

### 3. Import Karpenter Core Test Framework

Add the Karpenter core repository as a dependency in your Go module:

```bash
go get github.com/aws/karpenter-core
```

### 4. Configure CI/CD Integration

Set up CI/CD pipelines to run your tests automatically. You can use the following Makefile target as a reference:

```makefile
upstream-e2etests: 
	CLUSTER_NAME=${CLUSTER_NAME} envsubst < $(shell pwd)/test/pkg/environment/yourprovider/default_providernodeclass.yaml > ${TMPFILE}
	go test \
		-count 1 \
		-timeout 3.25h \
		-v \
		$(KARPENTER_CORE_DIR)/test/suites/... \
		--ginkgo.focus="${FOCUS}" \
		--ginkgo.timeout=3h \
		--ginkgo.grace-period=5m \
		--ginkgo.vv \
		--default-nodeclass="$(TMPFILE)"\
		--default-nodepool="$(shell pwd)/test/pkg/environment/yourprovider/default_nodepool.yaml"
```

This command:
1. Substitutes environment variables in your NodeClass template
2. Runs the Karpenter core test suites with appropriate timeouts and parameters
3. Specifies your provider's default NodeClass and NodePool configurations

You can also set up GitHub Actions workflows:

```yaml
# .github/workflows/tests.yaml
name: Integration Tests
on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.20'
      - name: Set up cloud provider credentials
        run: |
          # Set up your cloud provider credentials here
      - name: Run tests
        run: make upstream-e2etests
```

## Test Suites

The Karpenter core test framework includes the following test suites that will be executed against your provider implementation:

### Integration Tests (`test/suites/integration`)

1. **Basic Integration Tests**
   - DaemonSet compatibility
   - Pod scheduling with various constraints
   - Resource allocation and limits
   - Node startup and registration

2. **NodeClaim Tests**
   - Standalone NodeClaim creation and management
   - NodeClaim status conditions
   - NodeClaim resource allocation
   - NodeClaim template validation

3. **Expiration Tests**
   - Node expiration based on TTL settings
   - Graceful node termination
   - Pod rescheduling during node expiration
   - Expiration with active workloads

4. **Chaos Tests**
   - Runaway scale-up prevention
   - System stability under load
   - Recovery from disruptions
   - Concurrent operations handling

5. **Performance Tests**
   - Provisioning at scale (100+ pods)
   - Resource utilization efficiency
   - Scheduling latency measurements
   - Consolidation effectiveness

These test suites validate that your provider implementation correctly integrates with Karpenter's core functionality and can handle various operational scenarios.

## Example Providers

Reference these existing provider implementations for guidance:

- [AWS Provider](https://github.com/aws/karpenter-provider-aws)

## Getting Help

If you encounter issues while integrating Karpenter tests:

- Join the [#karpenter-dev](https://kubernetes.slack.com/archives/C04JW2J5J5P) channel in Kubernetes Slack
- Attend the bi-weekly working group meetings
- Open issues in the [Karpenter Core repository](https://github.com/aws/karpenter-core/issues)
