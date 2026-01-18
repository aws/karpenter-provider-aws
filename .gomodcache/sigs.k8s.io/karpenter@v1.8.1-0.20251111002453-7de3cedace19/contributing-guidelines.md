# Karpenter - Contributor Ladder

This document’s goal is to define clear, scalable, and transparent criteria to support community members to grow responsibility in Karpenter. This document also intends to capture a leadership path for contributors that intend to provide a sustained contribution to Karpenter by taking on reviewer and approver responsibilities at various levels. 

Ultimately, the criteria in this doc is aspirational. No set of written requirements can encapsulate the full criteria when determining if someone meets the bar to be a reviewer or approver, as some of the criteria are subjective and relies on the trust that each nominee has established with the community. To help guide readers, this document outlines ways to demonstrate expertise of the code base, sound judgement on decision tradeoffs, end user advocacy, care for community, and ability to work as a distributed team.

Much of this document uses the [SIG-Node Contributor Ladder](https://github.com/kubernetes/community/blob/master/sig-node/sig-node-contributor-ladder.md) as prior art. The goal is to mold these requirements to fit the Karpenter’s community. These requirements also lean on the established Kubernetes [membership documentation](https://github.com/kubernetes/community/blob/master/community-membership.md) for terminology.

As a final precursor, to become a reviewer or approver, users must nominate themselves. They are responsible for cutting a PR to the upstream repository, providing evidence in-line with the suggested requirements. Users should feel free to reach out to an existing approver to understand what how they land in respect to the criteria. The following sections are guiding criteria and guidelines, where the final decision lies with the maintainers. 

## Reviewers and Approvers

As an autoscaler, Karpenter is responsible for minimizing cost, maximizing application runtime, and automatically updating nodes. Its role as a critical cluster component managed by users sets a high bar for contributions and prioritizes efficiency, operationalization, and simplicity for its users. 

At a high level, reviewers and approvers should be inclined to scrutinize the cost-benefit tradeoffs for level of maintenance versus user-benefit, which may materialize as a bias to say “no” in reviews and design discussions. Reviewers should have an initial bias towards coding over reviews to demonstrate a baseline for knowledge of code base. In design discussions, reviewers and approvers should aim to maintain Karpenter’s efficiency, operationalization, and efficiency. 

Karpenter is a customer driven project, building solutions for real customer problems, and delaying solving theoretical ones. Reviewers and approvers should represent these tenets by minimizing changes to Karpenter’s API surface. No API is the best API, as an un-used or dead API becomes a burden for users to reason about, and a further burden for Karpenter users to maintain.

Lastly, as a library vended for and consumed by cloud providers, Karpenter aims to foster participation from all active cloud providers. Karpenter’s set of reviewers and approvers should have representatives from well known cloud provider implementations, as changes to upstream Karpenter can affect all dependent cloud provider implementations.

To become a reviewer or approver, a user should begin by cutting an issue to track the approval process.

### Reviewers

Reviewer status is an indication that the person is committed to Karpenter activities, demonstrates technical depth, has accumulated enough context, and is overall trustworthy to help inform approvers and aid contributors by applying a lgtm. Anyone is welcome to review a PR with a comment or feedback even if they do not have rights to apply a lgtm. The requirements listed in the [membership document](https://github.com/kubernetes/community/blob/master/community-membership.md#reviewer) highlight this as well.

The following is a guiding set of criteria for considering a user eligible to be a reviewer:

* Committed - proof of sustained contributions
* Be a [Kubernetes org member](https://github.com/kubernetes/community/blob/master/community-membership.md#member) (which has its [own set of requirements](https://github.com/kubernetes/community/blob/master/community-membership.md#requirements))
* Active Karpenter member for at least 6 months
* Demonstrates technical depth
* Primary reviewer for at least 5 PRs to the codebase
* Reviewed or merged at least 10 non-trivial substantial PRs to the codebase
* Knowledgeable about the codebase
* Reliable and builds consensus - established trust with the community
* Sponsored by an approver
* With no objections from other approvers

#### Committed

A user’s commitment should be established by looking at PR review history. Committed users should be participating in Karpenter meetings or other ad-hoc meetings that arise when tackling specific problems (exceptions are allowed for cases when timezone or other personal limitations are not allowing for the meeting participation). 

#### Technically sound

Proof of primary reviewership and significant contributions must be provided. Nominees must provide the list of PRs (at least 5 for primary reviewer and 10 substantial PRs authored or reviewed) as suggested in the membership document. Here are additional comments for this list of PRs:

* Reviewed PRs must be merged.
* Since the purpose is to demonstrate the nominee's technical depth, PRs like analyzer warnings fixes, mechanical “find/replace”-type PRs, minor improvements of logging and insignificant bug fixes are valued, but not counted towards the reviewer status nomination. Lack of reviews of those PRs may be a red flag for nomination approval.
* A primary reviewer should drive the review of the PR without significant input / guidance from the approver or other reviewers.

It is hard to assess codebase knowledge and it always will be a judgement call. Karpenter will rely on the listed PRs to ensure the person reviewed PRs from different areas of the codebase and on the comments made during Karpenter meetings.

Additional ways to establish the knowledge of context are:

* Contributions to Karpenter documentation
* Blog posts - k8s-hosted and external
* Contributions to other adjacent sub-projects within SIG Autoscaling

#### Trustworthy

Reviewer nominations are accepted by Karpenter approvers. Karpenter approvers take nominations seriously and are invested in building a healthy community. Nominees should help approvers understand their future goals in the community so we can help continue to build trust and mutual relationships and nurture new opportunities if and when a contributor wants to become an approver!

### Approvers 

Karpenter approvers have a lot of responsibilities. It is expected that a Karpenter approver keeps the codebase quality high by giving feedback, thoroughly reviewing code, and giving recommendations to Karpenter members and reviewers. Karpenter approvers are essentially gatekeepers to keep the code base at high quality. Karpenter maintains a rigidly high bar for becoming a Karpenter approver by developing trust in a community and demonstrating expertise with a bias towards initial code contributions over reviewing PRs.

We expect at this stage of Karpenter maturity for approvers to have a strong bias to say “no” to unneeded changes or improvements that don't clearly articulate and demonstrate broad benefits. As an autoscaler, approvers have a responsibility to evaluate changes or improvements at scale. [While scale dimensions and thresholds are complex](https://github.com/kubernetes/community/blob/master/sig-scalability/configs-and-limits/thresholds.md#kubernetes-thresholds), approvers should consider how changes may impact Karpenter's scalability and have a bias for “no” when any of these dimension's scalability is compromised. It also means that the velocity of new features may be affected by this bias. Our continuous work to improve the reliability of the codebase will help to maintain feature velocity going forward.

While evaluating a nomination for approval, nominees may be asked to provide examples of strict scrutiny. Strict scrutiny refers to instances where a performance regression, vulnerability, or complex unintended interaction could have occurred. We do not expect existing approvers or nominees to be perfect (no one is!) but as a maintainer community we have had instances of pull requests that we want to learn from and spot to mitigate potential risks given our trust to users and existing project maturity level. Where specific examples are not present for a nominee (which is fine), we may privately share examples from our past experience for warning signs.

In addition to the formal requirements for the [approver role](https://github.com/kubernetes/community/blob/master/community-membership.md#approver), Karpenter makes these recommendations for nominees for the Karpenter approver status on how to demonstrate expertise and develop trust. Ideally approver rights in more than one of these is **desired but not required**. This is a means of earning trust to existing approvers. 

#### Deep expertise across multiple core controllers 

* Demonstrated influence across multiple core controllers (e.g. provisioning, disruption, cluster state, etc.)
* Troubleshooting complex issues that touch require a holistic understanding of the code base, with an understanding of common 3rd party use-cases and tooling.
* Create and merge major code simplification and/or optimization PRs indicating deep understanding of tradeoffs taken and validation of potential side effects.

#### Proficient in features development

* Drive a few major features at all three stages:
    * “alpha” - design proposal and discussions
    * “beta” - initial customer feedback collection
    * “GA/deprecation” - stabilizing feature, following PRs, or managing deprecation.
* Demonstrate ability to stage changes and pass PRs keeping the end user experience and Kubernetes reliability as top priorities.
* Be a reviewer for a few major features and demonstrate meaningful participation in the review process.
* Give actionable feedback for the features and initial proposals during the Karpenter meetings.

#### Active community support

* Have approval rights in a well-known cloud provider implementation of Karpenter or in an adjacent SIG Autoscaling sub-project. 
* Be a primary PR reviewer for numerous PRs in multiple areas listed as a requirement for a reviewer.
* Actively triage issues and PRs, provide support to contributors to drive their PRs to completion.
* Be present, and participate in Karpenter meetings by speaking about features or improvements driven, or find some other way to prove the identity behind GitHub handle.

### Cleanup and Emeritus

The `kubernetes-sigs/karpenter` sub-project abides by the same cleanup process prescribed in https://github.com/kubernetes/community/blob/master/contributors/guide/owners.md#cleanup. It is generally recommended that reviewers or approvers who know that they are no longer going to be actively invovled _remove themselves_ from the OWNERS_ALIASES file; however, approvers may also initate a PR and reachout to the relevant reviewer/approver if they recognize that the user is no longer actively involved in the project (as defined in the community contributors doc linked above).