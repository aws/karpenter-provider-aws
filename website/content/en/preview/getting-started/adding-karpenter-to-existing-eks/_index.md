---
title: "Adding Karpenter to an Existing EKS Cluster (No Existing Autoscaler)"
linkTitle: "Add Karpenter to Existing EKS"
weight: 10
description: >
  Introduce self-managed Karpenter as the first autoscaling layer on an
  EKS cluster that today runs only fixed-size managed nodegroups (or Fargate)
  with no autoscaler. Covers IAM via CloudFormation, IRSA, discovery tagging,
  Helm install, and a phased workload migration plan.
---

This guide covers the scenario where:

- You have an EKS cluster running today, with workloads scheduled onto managed
  nodegroups or Fargate profiles.
- The cluster has no autoscaler. Nodegroups are sized manually (fixed
  `desiredCapacity`, or scaled by ad-hoc `eksctl scale` / CloudFormation runs).
- You want Karpenter to become the autoscaling layer for new pods and any
  workloads you migrate off the static nodegroups.

This is an additive install. Karpenter is layered onto the cluster without
disturbing any existing nodegroup or workload. There is no scale-down race to
manage and no drain step.

Migration is opt-in per workload. Existing nodegroups continue to run their
current workloads. New workloads, or existing workloads you re-target via
labels, taints, or affinity, land on Karpenter-launched nodes. Step 10 covers
how to shrink static nodegroups as load shifts off them.

The flow is:

1. Verify prerequisites on the existing cluster.
2. Set environment variables that the rest of the steps consume.
3. Provision the node IAM role, SQS interruption queue, EventBridge rules,
   and controller permissions in a single CloudFormation stack.
4. Create the controller IRSA role.
5. Wire the node IAM role into the cluster's auth surface (access entries or
   `aws-auth`).
6. Tag subnets and security groups for Karpenter discovery.
7. Install the controller via Helm, pinned to your existing static nodegroup.
8. Apply a `NodePool` and `EC2NodeClass`.
9. Validate with the upstream `inflate` test workload.
10. Pin critical add-ons (CoreDNS, metrics-server) to the static nodegroup.
11. Shift workloads onto Karpenter and shrink the static pools.
12. Operational follow-ups (metrics, billing alarms, AMI rotation).
13. Cleanup (only if rolling back).

The Karpenter controller pods run on one of your existing static nodegroup
nodes (or on Fargate). Do not run the Karpenter controller on a node Karpenter
manages: if Karpenter consolidates its own controller node, the controller
dies mid-disruption and the cluster gets stuck.

End-to-end cost is under \$1 in instance-hours. Step 13 returns the cluster to
its pre-install state.

## Other scenarios

If your starting state isn't "existing cluster with no autoscaler", use one
of these instead:

| Starting state | Use this guide |
|---|---|
| Brand-new cluster, creating from scratch with `eksctl` | [Karpenter Getting Started]({{< ref "../getting-started-with-karpenter" >}}) |
| Existing cluster running [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) | [Migrating from Cluster Autoscaler]({{< ref "../migrating-from-cas" >}}) |
| Want AWS to manage the Karpenter controller (and the AMIs, CNI, kube-proxy, CoreDNS) | [EKS Auto Mode](https://docs.aws.amazon.com/eks/latest/userguide/automode.html). Karpenter is built in; this guide does not apply |
| Private EKS cluster (no outbound internet) | [Karpenter Private Clusters]({{< ref "../getting-started-with-karpenter#private-clusters" >}}), plus the [EKS Private Cluster requirements](https://docs.aws.amazon.com/eks/latest/userguide/private-clusters.html) |
| Karpenter via Terraform | [Amazon EKS Blueprints for Terraform: Getting Started](https://aws-ia.github.io/terraform-aws-eks-blueprints/getting-started/). Provisions cluster + Karpenter add-on declaratively, replacing the imperative Steps 3–8 in this guide |

## Add Karpenter to an existing cluster

### 1. Prerequisites

Configure your shell so the AWS CLI knows which account and region to use.
Step 2 sets the full env-var block; for the Step 1 checks below, set these
three now:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step01-shell-prelude.sh" language="bash" %}}

Then verify the following on the target cluster:

1. **EKS version 1.25 or newer.** Karpenter requires
   [Kubernetes 1.25+](https://kubernetes.io/releases/version-skew-policy/).
2. **[OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html)
   enabled.** Required for
   [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html).
   Check with:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step02-check-oidc.sh" language="bash" %}}
   If the cluster has no OIDC issuer, create one:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step03-associate-oidc.sh" language="bash" %}}
3. **At least one existing
   [managed nodegroup](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html)
   or [Fargate profile](https://docs.aws.amazon.com/eks/latest/userguide/fargate-profile.html)
   spanning two or more AZs** to host the controller. The Karpenter Helm
   chart installs two replicas with a zonal
   [`topologySpreadConstraint`](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/)
   set to `DoNotSchedule`. kube-scheduler treats that constraint as
   vacuously satisfied when only one matching topology domain exists, so a
   single-AZ system nodegroup silently lands both replicas in the same AZ.
   Verify with:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step04-az-count.sh" language="bash" %}}
   The count must be ≥ 2. (Substitute your system-label key/value; Step 7
   covers picking it.) Two replicas at 1 vCPU / 1 GiB each fit on a single
   `m6i.large` system node. This nodegroup hosts the controller for the life
   of the install; Step 6 pins the controller to it via `nodeSelector`.
4. **No autoscaler is installed.** Confirm with:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step05-detect-autoscaler.sh" language="bash" %}}
   Both should return empty. If Cluster Autoscaler is present, follow the
   [Migrating from Cluster Autoscaler]({{< ref "../migrating-from-cas" >}})
   guide instead. The [EKS best practices guide for
   Karpenter](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html)
   warns that running both at once causes scale thrashing.
5. **List your current managed nodegroups** to decide which one hosts the
   controller and to plan the workload migration:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step06-list-nodegroups.sh" language="bash" %}}
   Note any labels or taints. Karpenter NodePools you create later should
   either *match* them (so workloads can land on either) or *avoid* them (so
   workloads stay segregated by tier).
6. **Local CLI tools:**
   - [`aws` CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
   - [`kubectl`](https://kubernetes.io/docs/tasks/tools/) (matching your
     control-plane version, ±1)
   - [`helm` v3.8+](https://helm.sh/docs/intro/install/) (OCI registry support)
   - [`eksctl` v0.202.0+](https://eksctl.io/installation/) (only used for the
     IRSA service-account role; raw `aws iam` works if you prefer)

{{% alert title="Note" color="primary" %}}
This guide assumes the user running these commands has cluster-admin
Kubernetes RBAC and IAM permission to create roles, policies, SQS queues,
EventBridge rules, and access entries.
{{% /alert %}}

### 2. Set environment variables

Step 1 set `AWS_PROFILE`, `AWS_DEFAULT_REGION`, and `CLUSTER_NAME`. Add the
remaining variables the rest of the guide consumes:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step07-env-vars.sh" language="bash" %}}

{{% alert title="Warning" color="warning" %}}
If you re-open a shell, re-export every variable before continuing.
{{% /alert %}}

Then resolve the AL2023 AMI alias version to pin in the `EC2NodeClass`:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step08-alias-version.sh" language="bash" %}}

Pinning the AMI is a [best practice for production
clusters](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_lock_down_amis_in_production_clusters).
The alias `al2023@latest` drifts every node when a new AMI ships.

### 3. Provision Karpenter's AWS resources

Karpenter needs an IAM role for the nodes it launches plus a set of managed
policies for the controller. The remaining choice is whether to provision an
interruption queue, which determines how Karpenter handles spot and
maintenance events.

The interruption queue is an SQS queue plus four EventBridge rules that
forward EC2 lifecycle events:

- [Spot Instance Interruption Warning](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html):
  the 2-minute notice EC2 sends before reclaiming a spot instance. Karpenter
  taints the node, drains pods, and pre-emptively launches a replacement.
- [Rebalance recommendations](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html):
  early warnings for spot instances at elevated risk of interruption.
- Scheduled maintenance events from AWS Health.
- Instance state-change notifications.

Without the queue, Karpenter still launches and consolidates nodes correctly,
but pods on disrupted instances are killed abruptly when EC2 reclaims them.

Pick a path:

- **With interruption queue (recommended).** Full Karpenter functionality;
  required for clusters running spot. Provisions the SQS queue, 4 EventBridge
  rules, 6 controller IAM policies, and the node IAM role via the upstream
  CloudFormation template.
- **Without interruption queue.** Minimal install for on-demand-only
  workloads, dev/test clusters, or environments that cannot create SQS or
  EventBridge resources. Provisions the node IAM role and 4 controller IAM
  policies via raw `aws iam` calls.

{{< tabpane text=true right=false >}}
  {{% tab header="**Provision Karpenter's AWS resources**:" disabled=true /%}}
  {{% tab header="With interruption queue (recommended)" %}}
Deploy the upstream
[CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/Welcome.html)
template. It creates the node IAM role, the
[SQS queue](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html),
and the [EventBridge](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-what-is.html)
rules used for spot-interruption and scheduled-maintenance handling.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step09-deploy-cloudformation.sh" language="bash" %}}

Karpenter requires the
[EC2 spot service-linked role](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html#service-linked-roles-spot-instance-requests)
to launch spot instances. It is account-wide and may already exist:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step10-spot-slr.sh" language="bash" %}}

Verify the stack landed in the correct region:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step11-verify-stack.sh" language="bash" %}}

Both should return the cluster's region. If not, redeploy with the correct
`AWS_DEFAULT_REGION`.

{{% alert title="Note" color="primary" %}}
The template is parameterized only on `ClusterName`. Set region or partition
via the AWS CLI environment, not as template parameters. The full resource
list is in the
[CloudFormation template](https://github.com/aws/karpenter-provider-aws/blob/v{{< param "latest_release_version" >}}/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml).
{{% /alert %}}
  {{% /tab %}}
  {{% tab header="Without interruption queue" %}}
Create the node IAM role and four controller managed policies directly via
`aws iam`. The four policies match the CloudFormation template's
`NodeLifecyclePolicy`, `IAMIntegrationPolicy`, `EKSIntegrationPolicy`, and
`ResourceDiscoveryPolicy`; the JSON is committed under
[`policies/`](https://github.com/aws/karpenter-provider-aws/tree/v{{< param "latest_release_version" >}}/website/content/en/preview/getting-started/adding-karpenter-to-existing-eks/policies)
and downloaded by the script below.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step09b-static-node-role.sh" language="bash" %}}

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step09c-static-controller-policies.sh" language="bash" %}}

To launch spot instances (without graceful drain on interruption), create the
[EC2 spot service-linked role](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html#service-linked-roles-spot-instance-requests).
Skip for on-demand-only installs.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step10-spot-slr.sh" language="bash" %}}

{{% alert title="Note" color="primary" %}}
You can switch on interruption handling later without reinstalling. Provision
the SQS queue, EventBridge rules, and `KarpenterControllerInterruptionPolicy`
(re-run the CloudFormation template against the same `ClusterName`), attach
the new policy to your controller role, then run
`helm upgrade --reuse-values --set "settings.interruptionQueue=${CLUSTER_NAME}"`.
{{% /alert %}}
  {{% /tab %}}
{{< /tabpane >}}

### 4. Create the controller IRSA role

Create an IAM role that trusts the cluster's OIDC provider and attach every
controller managed policy created in Step 3:

{{% alert title="Warning" color="warning" %}}
Run this and every subsequent shell block in **bash**, not zsh. The array
patterns use bash syntax (`< <(...)`, `"${array[@]}"`). On macOS, prefix with
`bash -c '...'` or run `bash` first. Apple's `/bin/bash` 3.2 is sufficient;
`mapfile` (bash 4+) is intentionally avoided.
{{% /alert %}}

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step12-controller-irsa.sh" language="bash" %}}

`--role-only` provisions only the IAM role with the right trust policy; the
Helm chart creates the ServiceAccount itself.

Verify the trust policy and the attached policies:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step13-verify-irsa.sh" language="bash" %}}

The `trust` value should reference the cluster's OIDC provider, and the
attached policies should include every `KarpenterController*-${CLUSTER_NAME}`
policy created by the CloudFormation stack.

### 5. Wire the node role into the cluster's auth surface

EKS clusters use one of two
[authentication modes](https://docs.aws.amazon.com/eks/latest/userguide/grant-k8s-access.html):

- [Access entries](https://docs.aws.amazon.com/eks/latest/userguide/access-entries.html)
  (`accessConfig.authenticationMode = API` or `API_AND_CONFIG_MAP`).
- [`aws-auth` ConfigMap](https://docs.aws.amazon.com/eks/latest/userguide/auth-configmap.html)
  (`CONFIG_MAP`).

Check which one the cluster uses:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step14-detect-auth-mode.sh" language="bash" %}}

{{< tabpane text=true right=false >}}
  {{% tab header="**Wire the node role**:" disabled=true /%}}
  {{% tab header="API / API_AND_CONFIG_MAP" %}}
`create-access-entry` returns `ResourceInUseException` if the entry already
exists. Check first to keep the step idempotent:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step15-access-entry.sh" language="bash" %}}

The `EC2_LINUX` type maps the role to `system:nodes` with the
`system:node:{{EC2PrivateDNSName}}` username, matching managed nodegroup
behavior. For Windows nodes, use `--type EC2_WINDOWS`. For Bottlerocket,
use `--type EC2_LINUX`.
  {{% /tab %}}
  {{% tab header="CONFIG_MAP" %}}
`eksctl create iamidentitymapping` is not idempotent and appends a duplicate
row when re-run. Check first:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step16-aws-auth-configmap.sh" language="bash" %}}
  {{% /tab %}}
{{< /tabpane >}}

Verify the wiring:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step17-verify-auth.sh" language="bash" %}}

The first returns the node-role ARN; the second returns the `aws-auth`
mapping rolegroups (`system:bootstrappers`, `system:nodes`).

{{% alert title="Warning" color="warning" %}}
If a Karpenter-launched node never reaches `Ready` and the kubelet log shows
`Unauthorized`, this step is the most common cause. Confirm the role ARN in
the access entry or `aws-auth` ConfigMap matches `KarpenterNodeRole-${CLUSTER_NAME}`
exactly.
{{% /alert %}}

### 6. Tag subnets and security groups for discovery

Karpenter's [`EC2NodeClass`]({{< ref "../../concepts/nodeclasses" >}}) selects
subnets and security groups by tag. The
[recommended convention]({{< ref "../../concepts/nodeclasses#specsubnetselectorterms" >}})
is:

```
karpenter.sh/discovery = <cluster-name>
```

Tag only subnets with routes to the EKS endpoint and to AWS APIs (EC2, ECR,
STS) that nodes need at registration. The safe rule is to tag the same
subnets your existing system nodegroup runs in:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step18-tag-discovery.sh" language="bash" %}}

{{% alert title="Warning" color="warning" %}}
If you tag a private subnet without a default route (no NAT gateway),
Karpenter may pick it and nodes hang at `Registered=Unknown` with no
obvious error. Tag only routed subnets.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
If every cluster subnet is routable (NAT gateway present, or public-only
cluster), tag all cluster-listed subnets instead:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step19-tag-all-subnets.sh" language="bash" %}}
{{% /alert %}}

If you use a separate node-shared security group (eksctl-created clusters
have a `ClusterSharedNodeSecurityGroup`), tag that as well.

Verify:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step20-verify-discovery-tags.sh" language="bash" %}}

You should see at least 2 subnets across at least 2 AZs, and at least 1
security group. If the subnet listing shows only one AZ, add subnets in
additional AZs to the VPC, tag them, and re-verify before continuing.
Karpenter requires zonal diversity for spot-pool requirements and
zonal-spread topology constraints.

{{% alert title="Note" color="primary" %}}
Karpenter ORs across the array of selectors but ANDs within a single term.
Use a different discovery key only when your environment requires it (e.g.,
shared VPCs across multiple clusters), and apply it consistently to subnets
*and* security groups.
{{% /alert %}}

### 7. Install Karpenter via Helm

Karpenter is published as an
[OCI artifact](https://helm.sh/docs/topics/registries/) in
[`public.ecr.aws`](https://gallery.ecr.aws/karpenter/karpenter).

Identify a node label that pins controller pods to your static nodegroup:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step21-list-node-labels.sh" language="bash" %}}

{{% alert title="Warning" color="warning" %}}
Pinning the controller is required. Without a `nodeSelector`, Karpenter can
eventually consolidate the node hosting its own controller, and the cluster
gets stuck mid-disruption.
{{% /alert %}}

Use a label that uniquely identifies the static nodegroup. Prefer a
string-valued label with a dot-free key (e.g., `role=system`,
`workload=system`). If none exists, add one:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step22-add-system-label.sh" language="bash" %}}

{{% alert title="Note" color="primary" %}}
Helm's `--set` parser interprets dots as map separators and coerces
`true`/`false`/numbers. Two consequences for `nodeSelector` values:

- If the label key contains dots (e.g., `eks.amazonaws.com/nodegroup`),
  escape each dot: `--set "nodeSelector.eks\.amazonaws\.com/nodegroup=system"`.
- If the label value is `true`, `false`, or numeric, use `--set-string` so
  Helm preserves the string type.
{{% /alert %}}

Install, substituting `<your-system-label-key>` and `<your-system-label-value>`
for the label above. Pick the tab matching Step 3:

{{< tabpane text=true right=false >}}
  {{% tab header="**Helm install command**:" disabled=true /%}}
  {{% tab header="With interruption queue (recommended)" %}}
{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step23-helm-install.sh" language="bash" %}}
  {{% /tab %}}
  {{% tab header="Without interruption queue" %}}
{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step23b-helm-install-no-queue.sh" language="bash" %}}
  {{% /tab %}}
{{< /tabpane >}}

{{% alert title="Warning" color="warning" %}}
The chart installs two replicas with a zonal topology-spread constraint. If
only one AZ holds nodes matching the `nodeSelector`, kube-scheduler treats
the constraint as vacuously satisfied and silently schedules both replicas
into the same AZ. Verify spread before installing:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step24-verify-az-spread.sh" language="bash" %}}

The count must be ≥ 2.
{{% /alert %}}

#### Verification

Confirm the install is healthy with these four checks:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step25-verify-install.sh" language="bash" %}}

Expected output:

1. **Pods**: two `karpenter-*` pods, both `Running 1/1`, on nodes in different
   AZs.
2. **CRDs**: four entries: `ec2nodeclasses.karpenter.k8s.aws`,
   `nodeclaims.karpenter.sh`, `nodeoverlays.karpenter.sh`,
   `nodepools.karpenter.sh`.
3. **IRSA annotation**: the ARN of `KarpenterController-${CLUSTER_NAME}`.
4. **Controller log**: the four happy-path lines (health probe, leader lease,
   sub-controller start):
   ```
   "message":"starting server","name":"health probe","addr":"[::]:8081"
   "message":"Attempting to acquire leader lease..."
   "message":"Successfully acquired lease","lock":"kube-system/karpenter-leader-election"
   "message":"Starting Controller","controller":"provisioner"
   ```
   `Unauthorized` errors against EC2 or EKS indicate a wrong IRSA trust
   policy or a missing managed-policy attachment. Return to Step 4.

### 8. Apply a NodePool and EC2NodeClass

Karpenter does not launch anything until you provide a
[`NodePool`]({{< ref "../../concepts/nodepools" >}}) (constraints) and an
[`EC2NodeClass`]({{< ref "../../concepts/nodeclasses" >}}) (AWS-side details:
role, subnets, security group, AMI).

The defaults below follow the
[NodePool best-practice
recommendations]({{< ref "../../concepts/nodepools" >}}) and the
[EKS Karpenter best-practices
guide](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html):

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step26-add-nodepool.sh" language="bash" %}}

Notes on the defaults:

- [`capacity-type: ["spot", "on-demand"]`]({{< ref "../../concepts/scheduling#capacity-type" >}}):
  Karpenter prefers spot and falls back to on-demand. For workloads that
  cannot tolerate
  [spot interruptions](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html),
  split into separate NodePools by tier.
- `instance-category: ["c", "m", "r"]` with
  [`minValues: 2`]({{< ref "../../concepts/nodepools#min-values" >}}):
  forces at least two general-purpose families, improving
  [spot-pool diversity](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_avoid_overly_constraining_the_instance_types_that_karpenter_can_provision_especially_when_utilizing_spot).
- `instance-generation > 3`: excludes older, less efficient generations.
- `instance-size NotIn ["nano", "micro", "small", "metal"]`: avoids small
  pods-per-CPU instances and skips bare-metal.
- `limits.cpu = 100`: [caps total provisioned CPU]({{< ref "../../concepts/nodepools#speclimits" >}}).
  Tune for your workload.
- [`consolidationPolicy: WhenEmptyOrUnderutilized`]({{< ref "../../concepts/disruption#consolidation" >}})
  with `consolidateAfter: 1m`: packs aggressively but waits 60 s after the
  last pod event to avoid deploy-time thrash.
- [`budgets: [{nodes: "10%"}]`]({{< ref "../../concepts/disruption#nodepool-disruption-budgets" >}}):
  caps disruption at 10% of Karpenter-managed nodes at once.
- [`expireAfter: 720h`]({{< ref "../../concepts/disruption#expiration" >}}):
  replaces nodes every 30 days via drift, picking up new AMIs and patches.
- AMI alias pinned to `al2023@${ALIAS_VERSION}`. Never
  [`@latest` in production](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_lock_down_amis_in_production_clusters).

{{% alert title="Warning" color="warning" %}}
`limits` are per-NodePool, not per-cluster. Set `limits` on every NodePool
when splitting workloads.
{{% /alert %}}

Verify both objects. The `EC2NodeClass` reconciler is async; `Ready` and
`ValidationSucceeded` flip from `Unknown` to `True` within a few seconds, so
use `kubectl wait`:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step27-verify-nodepool.sh" language="bash" %}}

Expected:

- `EC2NodeClass.status.conditions`: `AMIsReady=True`, `SubnetsReady=True`,
  `SecurityGroupsReady=True`, `InstanceProfileReady=True`,
  `ValidationSucceeded=True`, `Ready=True`. A `False` value means the
  NodeClass cannot launch, usually a typo in `role:` or a missing discovery
  tag.
- `subnets`: at least 2 distinct zones. A single zone means Step 6 missed
  AZs.
- `securityGroups`: includes `eks-cluster-sg-*`. Without it, nodes register
  but pods cannot reach the control plane.
- `NodePool`: initially shows `NodeRegistrationHealthy=Unknown`. Expected
  until the first node registers in Step 9.

### 9. Validate with the upstream `inflate` workload

The upstream `inflate` deployment requests 1 vCPU per replica. Use it to
confirm scale-up and scale-down end-to-end.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step28-inflate.sh" language="bash" %}}

Watch the lifecycle:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step29-watch-nodeclaims.sh" language="bash" %}}

In a separate terminal, follow the controller logs:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step30-controller-logs.sh" language="bash" %}}

A successful scale-up emits these two lines (instance type, AZ, and node
name will differ):

```
"message":"launched nodeclaim","instance-type":"<picked-type>","zone":"<picked-az>","capacity-type":"spot"
"message":"registered nodeclaim","Node":{"name":"<assigned-node-name>"}
```

Wall-clock between the two should be ~15–30 s. Confirm:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step31-confirm-scaleup.sh" language="bash" %}}

Then test consolidation:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step32-scale-zero.sh" language="bash" %}}

After the `consolidateAfter` window expires (1 minute with the defaults), the
controller drains and terminates the node over ~60–90 s. Total wall-clock
from scale-to-zero to node deletion is typically 2–3 minutes:

```
"message":"disrupting node(s)","command":"Empty/...: delete: nodepools=[default]","decision":"delete"
"message":"tainted node","taint.Key":"karpenter.sh/disrupted","taint.Effect":"NoSchedule"
"message":"deleted node","Node":{"name":"..."}
```

Confirm the node is gone:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step33-confirm-consolidation.sh" language="bash" %}}

Both empty means the install is healthy end-to-end.

#### Final install checklist

Before moving on to Step 10, all of the following should be true:

- [ ] CloudFormation stack `Karpenter-${CLUSTER_NAME}` is `CREATE_COMPLETE`
- [ ] `KarpenterController-${CLUSTER_NAME}` IAM role exists and trusts the
      cluster's OIDC provider
- [ ] `KarpenterNodeRole-${CLUSTER_NAME}` is wired into the cluster's auth
      surface (access entry or `aws-auth`)
- [ ] At least 2 subnets and the cluster security group are tagged
      `karpenter.sh/discovery=${CLUSTER_NAME}`
- [ ] Both Karpenter controller pods are `Running` `1/1` on nodes in
      different AZs
- [ ] Controller log shows `Successfully acquired lease` with no
      `Unauthorized` errors
- [ ] A test `inflate` deployment scaled up, Karpenter launched a node, and
      consolidation removed it after scale-down

### 10. Pin critical add-ons to the static nodegroup

Pin cluster-critical add-ons
([CoreDNS](https://docs.aws.amazon.com/eks/latest/userguide/managing-coredns.html),
[metrics-server](https://docs.aws.amazon.com/eks/latest/userguide/metrics-server.html),
the [VPC CNI](https://docs.aws.amazon.com/eks/latest/userguide/managing-vpc-cni.html)
DaemonSet, and the Karpenter controller) to the static nodegroup. Otherwise
Karpenter may consolidate a node hosting CoreDNS, causing brief DNS outages
on scale-down. See the
[EKS scale-cluster-services best practices](https://docs.aws.amazon.com/eks/latest/best-practices/scale-cluster-services.html)
for CoreDNS lameduck and readiness tuning on churning clusters.

The Karpenter controller is already pinned via Step 7's `nodeSelector`. For
CoreDNS and metrics-server, add a `nodeAffinity` for the static nodegroup
label:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step34-pin-addons.sh" language="bash" %}}

The VPC CNI is a DaemonSet and lands on every node by design.

### 11. Shift workloads to Karpenter

Karpenter is running but every existing pod is still scheduled to a static
nodegroup. Migration is incremental and reversible: opt workloads in by
removing the nodegroup affinity that pinned them, or by adding a
`nodeSelector` matching a Karpenter NodePool label (e.g.,
`provisioner: karpenter` from Step 8). Apply with a rolling restart so new
pods land on Karpenter-launched nodes:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step35-rollout-restart.sh" language="bash" %}}

Once a workload has soaked on Karpenter, scale the originating static
nodegroup down a few instances at a time. Watch for pods landing back on
remaining static nodes (a sign of a missed selector) and wait for steady
state before scaling further:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step36-scale-static-nodegroup.sh" language="bash" %}}

Keep a minimum of 2 nodes in the controller's host nodegroup. It must not
autoscale to zero and should not be the same nodegroup Karpenter is
replacing.

{{% alert title="Note" color="primary" %}}
For workloads that cannot run on spot (long-running batch, stateful
services, anything without a checkpoint mechanism), add a second NodePool
with `capacity-type: ["on-demand"]` and a distinct label (e.g.,
`tier: critical`), then pin with `nodeSelector: {tier: critical}`.
{{% /alert %}}

{{% alert title="Warning" color="warning" %}}
Do not scale the controller's host nodegroup to zero. To move the controller
to a different nodegroup, run
`helm upgrade --reuse-values --set nodeSelector.<key>=<value>` first and
confirm the controller pods reschedule before scaling the original
nodegroup down.
{{% /alert %}}

### 12. Operational follow-ups

These are not required for Karpenter to work but should be in place for
production.

1. **Scrape Karpenter metrics.** Karpenter exposes
   [Prometheus metrics]({{< ref "../../reference/metrics" >}}) on port
   `8000`. With [Amazon Managed Prometheus
   (AMP)](https://docs.aws.amazon.com/prometheus/latest/userguide/what-is-Amazon-Managed-Service-Prometheus.html),
   add a `ServiceMonitor` or scrape config pointing at
   `karpenter.kube-system.svc:8000/metrics`. Karpenter publishes a
   [reference Grafana dashboard]({{< ref "../getting-started-with-karpenter#monitoring-with-grafana-optional" >}}).
2. **Set a billing alarm.** The
   [EKS Karpenter best-practices guide](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_create_billing_alarms_to_monitor_your_compute_spend)
   recommends a
   [CloudWatch billing alarm](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/monitor_estimated_charges_with_cloudwatch.html)
   on a dollar threshold, since `limits` apply per NodePool.
3. **Pin the controller pods off Karpenter-managed nodes** via
   `nodeSelector`/`affinity` (Step 7 covers this).
4. **Plan AMI rotation.** When a new
   [AL2023 AMI](https://docs.aws.amazon.com/eks/latest/userguide/al2023.html)
   ships, bump `ALIAS_VERSION` in the `EC2NodeClass`. Karpenter
   [drifts]({{< ref "../../concepts/disruption#drift" >}}) existing nodes
   on the next consolidation cycle, throttled by
   [`disruption.budgets`]({{< ref "../../concepts/disruption#nodepool-disruption-budgets" >}}).
5. **Add more
   [NodePools]({{< ref "../../concepts/nodepools" >}})** for workloads with
   different needs (GPU, ARM, dedicated tenancy). Make them
   [mutually exclusive](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_create_nodepools_that_are_mutually_exclusive_or_weighted)
   via taints or labels, or use `weight` to break ties.

### 13. Cleanup

To remove Karpenter and return the cluster to its pre-install state, tear
down in reverse order. Delete the `EC2NodeClass` and `NodePool` first so
Karpenter cleanly drains its nodes back onto the static nodegroups (or
evicts pods if no capacity is available; scale the static nodegroup up
first if needed).

If you returned in a fresh shell, re-derive env vars and discovery state
(`$SUBNETS` and `$CLUSTER_SG` from Step 6 do not persist):

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step37-cleanup-rederive-vars.sh" language="bash" %}}

Tear down. Pick the tab matching Step 3:

{{< tabpane text=true right=false >}}
  {{% tab header="**Cleanup**:" disabled=true /%}}
  {{% tab header="With interruption queue" %}}
{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step38-cleanup-teardown.sh" language="bash" %}}
  {{% /tab %}}
  {{% tab header="Without interruption queue" %}}
{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step38b-cleanup-teardown-no-queue.sh" language="bash" %}}
  {{% /tab %}}
{{< /tabpane >}}

The cluster is back to its pre-Karpenter state. A reinstall of Steps 1–9
will succeed.

{{% alert title="Note" color="primary" %}}
If `aws cloudformation wait stack-delete-complete` returns `DELETE_FAILED`
for `Karpenter-${CLUSTER_NAME}`, the stack's IAM managed policies are still
attached to a role, usually a leftover `KarpenterNodeRole-${CLUSTER_NAME}`
created by a prior `eksctl create nodegroup` or a Terraform module. Find
and detach:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step39-cleanup-detach-leftover-roles.sh" language="bash" %}}
{{% /alert %}}

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `kubectl get nodepool` shows `NodeRegistrationHealthy=Unknown` and no node ever registers | [Access entry](https://docs.aws.amazon.com/eks/latest/userguide/access-entries.html) / [`aws-auth`](https://docs.aws.amazon.com/eks/latest/userguide/auth-configmap.html) not patched (Step 5), or the node role is missing a managed policy |
| `kubectl describe nodeclaim` shows `Launched=False` with `RunInstancesAuthCheckFailed` | A customer [SCP](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_policies_scps.html) is denying `ec2:RunInstances`. Check [CloudTrail](https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-user-guide.html) for the failing dry-run event |
| Controller log: `Unauthorized` against EC2 or EKS API | [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) trust policy is wrong, or the controller policies didn't attach |
| Controller log: `failed to discover subnets` | Subnets are not tagged with `karpenter.sh/discovery=${CLUSTER_NAME}` (see [`subnetSelectorTerms`]({{< ref "../../concepts/nodeclasses#specsubnetselectorterms" >}})) |
| NodeClaim launches but `Registered=Unknown` | Most often the access entry / `aws-auth` row, or a [security group not allowing kubelet → control plane](https://docs.aws.amazon.com/eks/latest/userguide/sec-group-reqs.html) on TCP/443. If you tagged a node SG that doesn't have the right ingress, untag it. |
| `kubectl get pods` for the controller shows `CrashLoopBackoff` | Almost always the IRSA annotation on the [ServiceAccount](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) is missing or pointing at the wrong role |

For deeper troubleshooting, see the upstream [Karpenter
Troubleshooting]({{< ref "../../troubleshooting" >}}) guide.

## Next steps

### Reference

- [NodePool concepts]({{< ref "../../concepts/nodepools" >}}): full field
  reference.
- [EC2NodeClass concepts]({{< ref "../../concepts/nodeclasses" >}}): AMI
  aliases, custom user-data, etc.
- [Disruption]({{< ref "../../concepts/disruption" >}}): drift,
  consolidation, expiration, interruption.
- [EKS Karpenter Best Practices guide](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html):
  production-readiness checklist.

### Hands-on workshops

- [EKS Karpenter Workshop](https://catalog.workshops.aws/karpenter/en-US):
  the canonical AWS-hosted lab. Walks through consolidation, drift, spot,
  and disruption budgets.
- [EC2 Spot Workshop for Karpenter](https://ec2spotworkshops.com/karpenter.html):
  focuses on spot diversification and interruption handling.
- [Advanced EKS Immersion: Karpenter](https://www.eksworkshop.com/docs/autoscaling/compute/karpenter/):
  multi-NodePool design, GPU workloads, and Spot fallback patterns.

### Patterns and examples

- [Karpenter Blueprints](https://github.com/aws-samples/karpenter-blueprints):
  reusable NodePool / EC2NodeClass examples (GPU, ARM, batch, multi-tenant).
- [Tutorial: Run Kubernetes Clusters for Less with EC2 Spot and Karpenter](https://community.aws/tutorials/run-kubernetes-clusters-for-less-with-amazon-ec2-spot-and-karpenter):
  end-to-end cost-optimization walkthrough including spot interruption
  simulation.
