---
title: "Design Guide"
linkTitle: "Design Guide"
weight: 20
description: >
  Read this before making large changes to Karpenter
---

Technical designs are essential to building robust, intuitive, and performant products that delight users. Writing a design can accelerate decision making and avoid wasting time on an implementation that never lands. But what makes a good design? These guidelines were authored with the Karpenter community in mind, but apply broadly to the development of Kubernetes Operators.

Designs don’t have to be long or formal, and should match the scope of the problem they’re trying to solve.

* Are there multiple potential solutions?
* Will users need to be aware of the changes?
* Would it be painful to discard a rejected implementation?
* When in doubt, write a 1 pager.

## Tell a Story

A design is a story that connects a user need with a technical direction that solves the need. Designs come in all shapes and sizes, and this document intentionally avoids prescribing a one-size-fits-all template. There’s no substitute for an author thinking deeply about a problem space, and mapping that to a clear story that walks readers through the ideas and helps them reason about a solution space. Keep readers engaged with concise language and make every word count.

Your story should include,

* [Context] Include some technical background that helps readers think about your idea in context
* [Problem] Clearly identify the problem to be solved and some guiding principles to help think about the solutions
* [Solutions] Talk through different potential solutions and their tradeoffs. Include diagrams to clarify concepts
* [Recommendation] Make a recommendation, but don’t be overly invested in it

The best way to improve your story telling skills is to write and review designs. Seek inspiration from recent designs in the project as well as from other domains. Focus on your audience and continuously reread and refine your design with their perspective in mind.

## Gather Broad Feedback

The bigger the change, the more likely your design will have broader implications than intended. Be vocal about design ideas as they’re explored and run them by engineering leaders in relevant systems. Surface your design ideas at the Karpenter working group, or asynchronously on the [Kubernetes Slack channel for Karpenter](https://kubernetes.slack.com/archives/C02SFFZSA2K).

The Kubernetes community is also a valuable source of feedback from both users and Kubernetes developers. Does your design touch scoped owned by any Kubernetes SIGs? Consider discussing the design ideas at the SIG or in their slack channel. Socializing high level ideas before the review gives your audience more time to think about possible interactions with existing and future systems.

It can be tempting to rush to solutions that unblock user adoption or ease user pain, but the wrong solution can have a greater negative impact on users than it solves. It’s impossible to know all future use cases and how your design choices may impact them, but the more thorough your investigation, the more likely your solution is to deliver long term value.

## Simple Solutions to Complex Problems

The best solutions are invisible to users and “Just Work™”. It’s easy to forget that users have business problems to focus on and each parameter and behavior your design introduces increases user cognitive load. Pragmatically, it’s not always possible to meet the broad requirements of Kubernetes without providing options, but solution spaces typically include a spectrum of configuration complexity.  Recognize that a solution for one user segment may be directly at odds with another or create long term technical debt for the project. Often, requirements only exist to workaround bugs or missing features in related systems. Deep dive requirements until you’re certain they’re necessary and ensure each bit of complexity justifies its existence.

## Common Gotchas

### Does your change introduce new APIs?

APIs are notoriously hard to get right and even harder to change. Kubernetes defines an [api deprecation policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/) that helps systems make backwards incompatible changes to APIs before graduating to a stable API with compatibility guarantees. Once an API is stable, features are typically via [feature gates](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/), which allows for experimentation and deprecation.

Think about how your API changes impact existing parameters and their deprecation policies. Consider how the user interacts with the product as a whole and if the feature supersedes or overlaps with existing concepts. Weigh the costs of deprecating existing features to the benefit of simplifying the product for all future users. The answer will change depending on the maturity of the product and breadth of adoption.

Build minimal and maintainable APIs by:

* Push back on requirements that introduces concepts for all users to solve problems for a few.
* Identify an opinionated default that solves the majority of use cases.
* Delay introducing a parameter into your API surface until users demand it; you can always add it later.
* Rely on existing concepts and idioms from the Kubernetes ecosystem. Look to [Kubernetes APIs](https://pkg.go.dev/k8s.io/api/core/v1) and projects like [Tekton](https://github.com/tektoncd/cli), [Knative](https://github.com/knative/serving), and [ACK](https://github.com/aws-controllers-k8s) and find concepts that will be familiar to users.
* Take advantage of opportunities to refine APIs while the impact of backwards incompatibility is small

### Does your change behave differently with different cloud providers?

Kubernetes is an open standard that users rely on to work across vendors. Users care deeply about this, as it minimizes the technical complexity to operate in different environments. Identify whether or not your feature varies across cloud providers or are bespoke to a specific provider. For some features, it’s possible to rely on existing vendor neutral abstractions. For others, it’s possible to define a neutral abstraction that cloud providers can implement.

Achieving consensus for new neutral concepts is hard. Often, the best path is to demonstrate value on a single vendor, and work to achieve neutrality as a followup effort. Be cautious about introducing or changing vendor neutral interfaces, as it will require changes from all providers. Similarly, invest heavily in getting these interfaces right in the early stages. As projects mature, these interfaces are rarely changed.

### Does your change expose details users may rely on?

Kubernetes based systems often use a layered architectural pattern that exposes underlying layers of abstraction. This approach enables broad extensibility and allows other systems to integrate at multiple layers of the stack. For example, Karpenter creates EC2 instances in your AWS account. This enables you to view logs or react to their creation with other automation without requiring any features from Karpenter. However, Karpenter also applies specific EC2 tags to the EC2 instances. Are the tags an implementation detail or an interface? What can you change without breaking compatibility?

Be intentional and explicit about the interface and implementation of your design and ensure that this is communicated to users. If implementation details are exposed through other APIs, expect users to rely on them as an interface unless told otherwise. In general, aim to minimize the project’s interface to maximize future flexibility.

### Does your change have a risk of breaking an undocumented invariant?

Systems often contain mechanisms that are implicitly assumed as invariant, but may not be obvious, especially over time. Existing mechanisms may not be extensible enough to support your design, and may require them to be rewritten as part of the design scope. Be aware that regression tests never have complete coverage and well intentioned engineers thought carefully about how things were done before your requirements.

* Identify the fundamental reason the existing mechanism is insufficient and be able to explain it in plain terms.
* Separate the new mechanism from the new feature that relies on it.
* Clean up after yourself and avoid getting stuck halfway between old and new mechanisms.

### Does your change impact performance?

Users have high expectations for performance on Kubernetes. Karpenter is especially sensitive, as it has the potential to impact application availability during traffic spikes. Think about how your solution scales, and look for opportunities to improve performance at the design level. Often, good designs don’t require trading-off a great UX for performance. Make it work, make it fast, make it pretty.

* Beware code that scales linearly with pods or nodes. Milliseconds in testing turn into seconds at scale.
* Cloud provider read APIs can have surprisingly high latency and low limits, use caching to minimize calls.
* Increases to memory and CPU usage increase capex cost for operators. Profile and optimize your implementations.
