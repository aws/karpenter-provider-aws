# Integration Testing
author: @njtran

Currently, users can only test Karpenter by adding to a list of integration tests run in a mock environment or installing and testing Karpenter on a real cluster. Users who want to run more comprehensive tests are limited by the lack of test automation and configurability.

To increase testing of Karpenter, this document introduces plans to create infrastructure and tools to extend the current mechanisms and enable developers to contribute with more confidence.

## Mediums of Testing

__Code Contributions__ are currently tested with integration tests automated by GitHub Actions. Testing a contribution comprehensively is hard for a few reasons:

* Testing all supported Kubernetes and Karpenter versions would be very tedious for one developer
* Testing with real instances would incur unwanted costs to the contributor

__On-demand testing__ is where a developer may need to test Karpenter independent of contribution. This testing may only be useful for a smaller subset of Karpenter users. Some use-cases include:

* Scale tests to verify test workloads or dependencies
* Benchmark performance of custom builds and changes

__Periodic tests__ are meant to run on a pre-determined basis to monitor the health of the Karpenter project. These can be used to:

* Track performance regressions/progressions over time
* Validate that Karpenter functionally works for existing and future releases.

## How Testing Will Impact the User Workflow

__(To be implemented)__ When contributing, users will need to go through some steps to automatically test their changes.

* When a user cuts a PR and is ready to test, a maintainer will need to approve testing by posting `/ok-to-test`
* A robot watching the PR will kick off tests as configured in the testing folder. The tests will run in infrastructure owned by the respective cloud provider.
* A link to the logs and metrics will be posted in the PR, whether it’s prow as used in the [kubernetes/test-infra](https://github.com/kubernetes/test-infra) or an alternative solution, such as Tekton.
* After the tests are successful, the robot will report the results and a maintainer will merge it once approving the code.

__(To be implemented)__ Users can follow instructions to replicate the infrastructure used to run the tests by the cloud provider. Contributors will be able to run tests on their own if they want to shorten the reviewer loop. The associated README will instruct users how to run tests as automated for a PR.

__(To be implemented)__ Contributing to the list of test suites in the testing folder will include those tests in automated testing. As some contributions will only affect real instances, contributors will need to include new tests as well. A user will be able to follow a README in the testing folder to understand how to test their new tests as well.

## Operational Excellence

__(To be implemented)__ Periodic testing will be an important part of Karpenter’s testing history. Results and history will be visualized as a testgrid (https://testgrid.k8s.io/) where users can look at metrics and logs for each set of test runs.

__(To be implemented)__ Upgrade instructions between releases as detailed in the Upgrade Guide (https://karpenter.sh/preview/upgrading/upgrade-guide/#how-do-we-break-incompatibility) will be tested as well. Additional tests will be included in the PR to create the release. As a result, releases will go through the same process as normal commits, and will ensure that upgrade instructions that introduce breaking changes are tested.
