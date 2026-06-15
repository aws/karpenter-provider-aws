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

This guide is for the specific scenario where:

- **You have an EKS cluster running today.** Workloads are scheduled onto one
  or more managed nodegroups (or Fargate profiles).
- **The cluster has no autoscaler.** Nodegroups are sized manually — either
  fixed `desiredCapacity`, or scaled by ad-hoc `eksctl scale` /
  CloudFormation runs. Cluster Autoscaler is not installed.
- **You want Karpenter to become the autoscaling layer.** Going forward,
  capacity for new pods (and eventually pods migrated off your static
  nodegroups) is provisioned by Karpenter.

Because there is no existing autoscaler, this is a purely **additive** install.
Karpenter is layered onto the cluster without disturbing any existing
nodegroup or workload. There is no scale-down race to manage (the way a CAS →
Karpenter migration would have), and no need to drain anything during the
install.

The migration model is **opt-in per workload**. Existing nodegroups continue
to run the workloads scheduled to them. New workloads — or existing workloads
you re-target via labels, taints, or affinity — land on Karpenter-launched
nodes. You shrink the static nodegroups manually, on your own timeline, as
load shifts off them. Step 10 covers that phase explicitly.

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

The Karpenter controller pods themselves run on one of your existing static
nodegroup nodes (or on Fargate). **Do not run the Karpenter controller on a
node Karpenter manages** — if Karpenter consolidates its own controller node,
the controller dies mid-disruption and the cluster gets stuck.

The estimated cost of running this guide end-to-end is under **\$1** in
instance-hours. Step 13 returns the cluster to its pre-install,
static-nodegroup-only state.

### Other scenarios

If your starting state isn't "existing cluster with no autoscaler", use one
of these instead:

| Starting state | Use this guide |
|---|---|
| Brand-new cluster, creating from scratch with `eksctl` | [Karpenter — Getting Started]({{< ref "../getting-started-with-karpenter" >}}) |
| Existing cluster running [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) | [Karpenter — Migrating from Cluster Autoscaler]({{< ref "../migrating-from-cas" >}}) |
| Want AWS to manage the Karpenter controller (and the AMIs, CNI, kube-proxy, CoreDNS) | [EKS Auto Mode](https://docs.aws.amazon.com/eks/latest/userguide/automode.html) — Karpenter is built in; this guide does not apply |
| Private EKS cluster (no outbound internet) | [Karpenter — Private Clusters]({{< ref "../getting-started-with-karpenter#private-clusters" >}}), plus the [EKS Private Cluster requirements](https://docs.aws.amazon.com/eks/latest/userguide/private-clusters.html) |
| Karpenter via Terraform | [Amazon EKS Blueprints for Terraform — Getting Started](https://aws-ia.github.io/terraform-aws-eks-blueprints/getting-started/) — provisions cluster + Karpenter add-on declaratively, replacing the imperative Steps 3–8 in this guide |

## 1. Prerequisites

Before running any commands, configure your shell so the AWS CLI knows
which account and region to talk to. Step 2 sets the full env-var block,
but the Step 1 prereq checks below already use `${CLUSTER_NAME}` and
need `AWS_PROFILE` resolved — set those three now:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step01-shell-prelude.sh" language="bash" %}}

Then verify the following on the cluster you intend to install Karpenter on:

1. **EKS version 1.25 or newer.** The current Karpenter release requires
   [Kubernetes 1.25+](https://kubernetes.io/releases/version-skew-policy/).
2. **[OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html)
   enabled.** Required for
   [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html). Check with:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step02-check-oidc.sh" language="bash" %}}
   If the cluster does not have an OIDC issuer, create one:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step03-associate-oidc.sh" language="bash" %}}
3. **At least one existing
   [managed nodegroup](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html)
   or [Fargate profile](https://docs.aws.amazon.com/eks/latest/userguide/fargate-profile.html)**
   that the Karpenter controller pods can run on. **The host nodegroup
   must span at least 2 AZs** to preserve HA for the controller. The
   Karpenter Helm chart installs two replicas with a zonal
   [`topologySpreadConstraint`](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/)
   set to `DoNotSchedule`, but kube-scheduler treats that constraint as
   vacuously satisfied when only one matching topology domain exists —
   so on a single-AZ system nodegroup, both replicas land in the same
   AZ silently. The install appears healthy, but the controller has no
   AZ redundancy and a single-AZ event will take both replicas down.
   Verify with the doc's check:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step04-az-count.sh" language="bash" %}}
   The count must be ≥ 2. (Substitute your actual system-label key/value;
   Step 7 covers picking it.) Two replicas at 1 vCPU / 1 GiB each fit on a
   single `m6i.large` system node. This nodegroup must keep running for the
   life of the install — it hosts the controller. Step 6 pins the controller
   to it via `nodeSelector`.
4. **No autoscaler is installed.** Confirm with:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step05-detect-autoscaler.sh" language="bash" %}}
   Both should return empty. If Cluster Autoscaler is present, this is not the
   right guide — you need a CAS → Karpenter migration plan that drains and
   removes CAS first. The [EKS best practices guide for
   Karpenter](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html)
   warns that running both at once causes scale thrashing.
5. **List your current managed nodegroups.** You'll need this to decide which
   one hosts the controller, and later to plan the workload migration:
   {{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step06-list-nodegroups.sh" language="bash" %}}
   Note any labels or taints — Karpenter NodePools you create later should
   either *match* them (so workloads can land on either) or *avoid* them (so
   workloads stay segregated by tier).
6. **Local CLI tools:**
   - [`aws` CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
   - [`kubectl`](https://kubernetes.io/docs/tasks/tools/) (matching your
     control-plane version, ±1)
   - [`helm` v3.8+](https://helm.sh/docs/intro/install/) (OCI registry support)
   - [`eksctl` v0.202.0+](https://eksctl.io/installation/) (only used for the
     IRSA service-account role; you can do this with raw `aws iam` if you
     prefer)

{{% alert title="Note" color="primary" %}}
The doc assumes the user running these commands has cluster-admin Kubernetes
RBAC and IAM permission to create roles, policies, SQS queues, EventBridge
rules, and access entries. If you don't, hand the IAM/SQS/EventBridge step to
the team that does.
{{% /alert %}}

## 2. Set environment variables

Step 1's prelude already set `AWS_PROFILE`, `AWS_DEFAULT_REGION`, and
`CLUSTER_NAME`. This step adds the remaining variables the rest of the
guide consumes:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step07-env-vars.sh" language="bash" %}}

{{% alert title="Warning" color="warning" %}}
If you re-open a shell, re-export everything before continuing — the rest of
this guide depends on these variables.
{{% /alert %}}

Then resolve the AL2023 AMI alias version to pin in the `EC2NodeClass`:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step08-alias-version.sh" language="bash" %}}

Pinning the AMI is a [best practice for production
clusters](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_lock_down_amis_in_production_clusters);
the alias `al2023@latest` will drift every node when a new AMI ships.

## 3. Provision Karpenter's AWS resources

Karpenter needs an IAM role for the nodes it launches plus a set of
managed policies for the controller itself. The choice you make here is
**whether to provision an interruption queue** alongside those IAM
resources, because that single decision determines how much
infrastructure the install creates and how Karpenter handles spot and
maintenance events.

The interruption queue is an SQS queue plus four EventBridge rules that
forward EC2 lifecycle events into the queue:

- [Spot Instance Interruption Warning](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html) —
  the 2-minute notice EC2 sends before reclaiming a spot instance.
  Karpenter reads this from the queue, taints the node, drains pods, and
  pre-emptively launches a replacement so workloads continue running.
- [Rebalance recommendations](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html) —
  early warnings for spot instances at elevated risk of interruption.
- Scheduled maintenance events from AWS Health.
- Instance state-change notifications.

Without the queue, Karpenter still launches and consolidates nodes
correctly, but pods on disrupted instances are killed abruptly with no
graceful drain when EC2 reclaims them.

Pick a path:

- **With interruption queue (recommended)** — full Karpenter
  functionality. Best for any cluster running spot. Provisions an SQS
  queue + 4 EventBridge rules + 6 controller IAM policies + the node
  IAM role via the upstream CloudFormation template.
- **Without interruption queue** — minimal install. Suitable only for
  on-demand-only workloads, dev/test clusters, or cases where you have
  no IAM permission to create SQS queues or EventBridge rules.
  Provisions only the node IAM role + 4 controller IAM policies via
  raw `aws iam` calls. No SQS, no EventBridge, no CloudFormation.

{{< tabpane text=true right=false >}}
  {{% tab header="**Provision Karpenter's AWS resources**:" disabled=true /%}}
  {{% tab header="With interruption queue (recommended)" %}}
Deploy the upstream
[CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/Welcome.html)
template as a single stack. It creates the IAM role for
Karpenter-launched nodes plus the
[SQS queue](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html)
and [EventBridge](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-what-is.html)
rules Karpenter uses for spot-interruption and scheduled-maintenance
handling.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step09-deploy-cloudformation.sh" language="bash" %}}

Karpenter also needs the
[EC2 spot service-linked role](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html#service-linked-roles-spot-instance-requests)
to launch spot instances. It's account-wide, so it may already exist:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step10-spot-slr.sh" language="bash" %}}

Verify the stack landed in the correct region:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step11-verify-stack.sh" language="bash" %}}

Both should return the cluster's region in the URL/ARN. If not, you ran
the deploy with a stale `AWS_DEFAULT_REGION` — delete the misplaced
stack and redeploy.

{{% alert title="Note" color="primary" %}}
The template is parameterized only on `ClusterName`. To change region
or partition, set those via the AWS CLI environment, not as template
parameters. The full list of resources the stack creates (node role,
SQS queue, EventBridge rules, controller permissions) is in the
[CloudFormation template](https://github.com/aws/karpenter-provider-aws/blob/v{{< param "latest_release_version" >}}/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml)
itself.
{{% /alert %}}
  {{% /tab %}}
  {{% tab header="Without interruption queue" %}}
Create the node IAM role and four controller managed policies directly
via `aws iam` calls. The four policies are identical to the
CloudFormation template's `NodeLifecyclePolicy`, `IAMIntegrationPolicy`,
`EKSIntegrationPolicy`, and `ResourceDiscoveryPolicy`; their JSON
bodies are committed under
[`policies/`](https://github.com/aws/karpenter-provider-aws/tree/v{{< param "latest_release_version" >}}/website/content/en/preview/getting-started/adding-karpenter-to-existing-eks/policies)
in this guide's directory and downloaded by the script below.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step09b-static-node-role.sh" language="bash" %}}

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step09c-static-controller-policies.sh" language="bash" %}}

If you also intend to launch spot instances (without graceful drain on
interruption), create the
[EC2 spot service-linked role](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html#service-linked-roles-spot-instance-requests).
Skip this step for on-demand-only installs.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step10-spot-slr.sh" language="bash" %}}

{{% alert title="Note" color="primary" %}}
You can switch on interruption handling later without reinstalling.
Provision the SQS queue + EventBridge rules +
`KarpenterControllerInterruptionPolicy` (run the CloudFormation
template against the same `ClusterName` to do it in one shot), attach
the new policy to your controller role, then run
`helm upgrade --reuse-values --set "settings.interruptionQueue=${CLUSTER_NAME}"`.
{{% /alert %}}
  {{% /tab %}}
{{< /tabpane >}}

## 4. Create the controller IRSA role

The Karpenter controller authenticates to AWS via IAM Roles for Service
Accounts (IRSA). Create an IAM role that trusts the cluster's OIDC provider
and attach every controller managed policy that the CloudFormation stack
created in Step 3:

{{% alert title="Warning" color="warning" %}}
Run this and every subsequent shell block in **bash**, not zsh. The
array patterns use bash syntax (`< <(...)`, `"${array[@]}"`). On macOS
the default shell is zsh — prefix with `bash -c '...'` or run `bash`
first. Apple's `/bin/bash` is 3.2 (last GPLv2 release), which is
sufficient for the patterns below; `mapfile` is intentionally avoided
because it requires bash 4+.
{{% /alert %}}

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step12-controller-irsa.sh" language="bash" %}}

`--role-only` is important — the Helm chart creates the ServiceAccount itself,
and we just want `eksctl` to provision the IAM role with the right trust
policy.

Verify the role was created with the expected trust policy and the right
number of attached policies:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step13-verify-irsa.sh" language="bash" %}}

The `trust` value should reference your cluster's OIDC provider, and the
attached-policies list should include every `KarpenterController*-${CLUSTER_NAME}`
policy created by the CloudFormation stack in Step 3.

## 5. Wire the node role into the cluster's auth surface

EKS clusters use one of two
[authentication modes](https://docs.aws.amazon.com/eks/latest/userguide/grant-k8s-access.html):

- **[Access entries](https://docs.aws.amazon.com/eks/latest/userguide/access-entries.html)**
  (`accessConfig.authenticationMode = API` or `API_AND_CONFIG_MAP`) — the
  modern path.
- **[`aws-auth` ConfigMap](https://docs.aws.amazon.com/eks/latest/userguide/auth-configmap.html)**
  (`CONFIG_MAP`) — the legacy path.

Check which one this cluster uses:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step14-detect-auth-mode.sh" language="bash" %}}

{{< tabpane text=true right=false >}}
  {{% tab header="**Wire the node role**:" disabled=true /%}}
  {{% tab header="API / API_AND_CONFIG_MAP" %}}
`create-access-entry` returns `ResourceInUseException` if the entry
already exists, so check first to keep the step idempotent:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step15-access-entry.sh" language="bash" %}}

The `EC2_LINUX` type automatically maps the role to `system:nodes` with the
`system:node:{{EC2PrivateDNSName}}` username — the same mapping used by
managed nodegroups.

For Windows nodes, use `--type EC2_WINDOWS`. For Bottlerocket,
`--type EC2_LINUX` works.
  {{% /tab %}}
  {{% tab header="CONFIG_MAP" %}}
`eksctl create iamidentitymapping` is **not idempotent** — running it
again appends a duplicate row. Check first:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step16-aws-auth-configmap.sh" language="bash" %}}
  {{% /tab %}}
{{< /tabpane >}}

Verify the wiring landed:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step17-verify-auth.sh" language="bash" %}}

The first returns the node-role ARN; the second returns the `aws-auth`
mapping rolegroups (`system:bootstrappers`, `system:nodes`).

{{% alert title="Warning" color="warning" %}}
If a Karpenter-launched node never reaches `Ready` and the kubelet log shows
`Unauthorized`, this step is the most common cause. Confirm the role ARN in
the access entry / `aws-auth` ConfigMap matches `KarpenterNodeRole-${CLUSTER_NAME}`
exactly.
{{% /alert %}}

## 6. Tag subnets and security groups for discovery

Karpenter's [`EC2NodeClass`]({{< ref "../../concepts/nodeclasses" >}})
selects subnets and security groups by tag. The
[recommended convention]({{< ref "../../concepts/nodeclasses#specsubnetselectorterms" >}})
is:

```
karpenter.sh/discovery = <cluster-name>
```

**Important:** every subnet you tag must have a route to the EKS cluster
endpoint and to AWS APIs (EC2, ECR, STS) that Karpenter-launched nodes
need at registration time. For VPCs without a NAT gateway, the EKS-listed
subnets typically include both public subnets (with a default route via
the IGW — these work) and private subnets (with no default route — nodes
launched here will hang `Registered=Unknown`). **Tag only the routed
subnets**; if you tag all four indiscriminately, Karpenter will randomly
pick a stranded subnet and the install appears stuck with no obvious
error message.

The simplest safe rule: **tag the same subnets your existing system
nodegroup runs in.** Those are guaranteed to be reachable. Pull subnet
IDs from the EC2 instances backing your existing nodes:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step18-tag-discovery.sh" language="bash" %}}

{{% alert title="Note" color="primary" %}}
If you know all the cluster's subnets are routable (e.g., NAT gateway is
configured for private subnets, or the cluster only has public subnets),
you can tag every cluster-listed subnet instead:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step19-tag-all-subnets.sh" language="bash" %}}
{{% /alert %}}

If you also use a separate node-shared security group (eksctl-created
clusters have one called `ClusterSharedNodeSecurityGroup`), tag that too.

Verify:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step20-verify-discovery-tags.sh" language="bash" %}}

You should see at least 2 subnets across at least 2 AZs, and at least 1
security group. **If the subnet listing shows only one AZ, fix it now** —
Karpenter needs zonal diversity to satisfy spot-pool requirements and
zonal-spread topology constraints. Add subnets in additional AZs to your
VPC, tag them, and re-verify before continuing.

{{% alert title="Note" color="primary" %}}
Karpenter ORs across the array of selectors but ANDs within a single term.
The single tag `karpenter.sh/discovery=<cluster>` is the conventional
discovery key — pick a different convention only if your environment requires
it (e.g., shared VPCs across multiple clusters), and apply it consistently
to subnets *and* security groups.
{{% /alert %}}

## 7. Install Karpenter via Helm

Karpenter is published as an
[OCI artifact](https://helm.sh/docs/topics/registries/) in
[`public.ecr.aws`](https://gallery.ecr.aws/karpenter/karpenter).

First, identify the node label that pins controller pods to your existing
static nodegroup. **This is required**, not optional — without it, Karpenter
can eventually consolidate the node hosting its own controller and the
cluster gets stuck mid-disruption. Confirm the label exists before
installing:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step21-list-node-labels.sh" language="bash" %}}

Look for a label that uniquely identifies the static nodegroup. The
simplest pattern is a custom label whose key contains no dots and whose
value is a plain string, e.g. `role=system` or `workload=system`. If
none exists, add one:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step22-add-system-label.sh" language="bash" %}}

{{% alert title="Note" color="primary" %}}
Helm's `--set` parser interprets dots as map separators, and infers
boolean/numeric values automatically. Two consequences for label keys
and values:

- If your label key contains dots (e.g. `eks.amazonaws.com/nodegroup`),
  escape each dot as `\.` in the `--set` value:
  `--set "nodeSelector.eks\.amazonaws\.com/nodegroup=system"`.
  Without escaping, Helm builds a nested map (`{eks: {amazonaws: ...}}`)
  and server-side apply rejects it.
- If your label value is `true`, `false`, or a number, use `--set-string`
  instead of `--set` so Helm preserves the value as a string. Kubernetes
  `nodeSelector` values must be strings, but `--set` will coerce
  `true` to a boolean and the install will fail.

To avoid both pitfalls, prefer simple string-valued labels with
dot-free keys for the controller's `nodeSelector`.
{{% /alert %}}

Then install, substituting `<your-system-label-key>` and
`<your-system-label-value>` for the label you just confirmed. Pick the
tab matching the path you took in Step 3:

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
The chart installs **two replicas with a zonal topology-spread constraint**.
The host nodegroup must span ≥ 2 AZs (verified in Step 1). If only one AZ
holds nodes matching your `nodeSelector`, kube-scheduler treats the
topology-spread constraint as vacuously satisfied (only one eligible
topology domain exists), so both replicas schedule into the same AZ
silently — no `FailedScheduling` event, no `Pending` pod. The install
appears healthy, but you've lost AZ redundancy for the controller and an
AZ-correlated event will take both replicas down. Verify spread is
real *before* installing:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step24-verify-az-spread.sh" language="bash" %}}

The count must be ≥ 2.
{{% /alert %}}

### Verification

Confirm the install is fully healthy with these four checks:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step25-verify-install.sh" language="bash" %}}

What you should see:

1. **Pods**: two `karpenter-*` pods, both `Running`, both `1/1` Ready, on
   nodes in different AZs.
2. **CRDs**: four entries — `ec2nodeclasses.karpenter.k8s.aws`,
   `nodeclaims.karpenter.sh`, `nodeoverlays.karpenter.sh`,
   `nodepools.karpenter.sh`.
3. **IRSA annotation**: the ARN of `KarpenterController-${CLUSTER_NAME}`.
4. **Controller log**: the four canonical happy-path lines — health probe
   server starting, leader-lease acquired, sub-controllers starting:
   ```
   "message":"starting server","name":"health probe","addr":"[::]:8081"
   "message":"Attempting to acquire leader lease..."
   "message":"Successfully acquired lease","lock":"kube-system/karpenter-leader-election"
   "message":"Starting Controller","controller":"provisioner"
   ```
   If you see `Unauthorized` errors against EC2 or EKS APIs instead, the IRSA
   trust policy or one of the controller managed-policy attachments is
   wrong — go back to Step 4.

## 8. Apply a NodePool and EC2NodeClass

Karpenter doesn't launch anything until you give it a
[`NodePool`]({{< ref "../../concepts/nodepools" >}}) (what constraints
it can pick from) and an
[`EC2NodeClass`]({{< ref "../../concepts/nodeclasses" >}}) (the
AWS-side details — role, subnets, SG, AMI).

The defaults below follow the
[NodePool best-practice
recommendations]({{< ref "../../concepts/nodepools" >}}) and the
[EKS Karpenter best-practices
guide](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html):

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step26-add-nodepool.sh" language="bash" %}}

Things to call out about these defaults:

- **[`capacity-type: ["spot", "on-demand"]`]({{< ref "../../concepts/scheduling#capacity-type" >}})** —
  Karpenter prefers spot when available and falls back to on-demand. For
  workloads that cannot tolerate
  [spot interruptions](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html),
  split into separate NodePools by tier.
- **`instance-category: ["c", "m", "r"]` with
  [`minValues: 2`]({{< ref "../../concepts/nodepools#min-values" >}})** —
  forces Karpenter to consider at least two of the general-purpose families,
  improving [spot-pool diversity](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_avoid_overly_constraining_the_instance_types_that_karpenter_can_provision_especially_when_utilizing_spot).
- **`instance-generation > 3`** — excludes older, less efficient generations.
- **`instance-size NotIn ["nano", "micro", "small", "metal"]`** — avoids
  bin-packing onto pods-too-many-for-the-CPU instances and skips bare-metal.
- **`limits.cpu = 100`** — [caps total provisioned CPU]({{< ref "../../concepts/nodepools#speclimits" >}})
  so a runaway scheduler bug doesn't bankrupt the cluster. Tune for your real
  workload.
- **[`consolidationPolicy: WhenEmptyOrUnderutilized`]({{< ref "../../concepts/disruption#consolidation" >}})**
  with `consolidateAfter: 1m` — packs aggressively but waits 60 s after the
  last pod-add/remove event, avoiding thrash during deploys.
- **[`budgets: [{nodes: "10%"}]`]({{< ref "../../concepts/disruption#nodepool-disruption-budgets" >}})** —
  never disrupt more than 10% of Karpenter-managed nodes at once.
- **[`expireAfter: 720h`]({{< ref "../../concepts/disruption#expiration" >}})** —
  replace nodes every 30 days, picking up new AMIs and patches via natural
  drift.
- **AMI alias pinned to `al2023@${ALIAS_VERSION}`** — never
  [`@latest` in production](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_lock_down_amis_in_production_clusters).

{{% alert title="Warning" color="warning" %}}
`limits` are **per-NodePool**, not per-cluster. If you split the workload
across multiple NodePools, set `limits` on each.
{{% /alert %}}

Verify both objects landed correctly. The `EC2NodeClass` reconciler is
async — `Ready` and `ValidationSucceeded` flip from `Unknown` to `True`
within a few seconds, so use `kubectl wait` rather than checking
immediately:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step27-verify-nodepool.sh" language="bash" %}}

What to expect:

- **`EC2NodeClass.status.conditions`**: `AMIsReady=True`, `SubnetsReady=True`,
  `SecurityGroupsReady=True`, `InstanceProfileReady=True`,
  `ValidationSucceeded=True`, `Ready=True`. Any `False` here means the
  NodeClass cannot launch — most often a typo in `role:` or a missing
  discovery tag.
- **`subnets`** should list ≥ 2 distinct zones. If only one zone is
  returned, your subnet tagging in Step 6 missed AZs.
- **`securityGroups`** should include the cluster's `eks-cluster-sg-*`
  group. If it's missing, nodes will register but pods won't reach the
  control plane.
- **`NodePool`** initially shows `NodeRegistrationHealthy=Unknown` — that's
  expected until the first node successfully registers in Step 9.

## 9. Validate with the upstream `inflate` workload

The upstream getting-started guide ships a synthetic `inflate` deployment that
requests 1 vCPU per replica. Use it to confirm Karpenter scales up and down
end-to-end.

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step28-inflate.sh" language="bash" %}}

Watch the lifecycle:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step29-watch-nodeclaims.sh" language="bash" %}}

In a separate terminal, follow the controller logs:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step30-controller-logs.sh" language="bash" %}}

A successful scale-up emits these two lines (exact instance type, AZ, and
node name will differ in your run):

```
"message":"launched nodeclaim","instance-type":"<picked-type>","zone":"<picked-az>","capacity-type":"spot"
"message":"registered nodeclaim","Node":{"name":"<assigned-node-name>"}
```

Wall-clock between the two should be ~15–30 s. Confirm:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step31-confirm-scaleup.sh" language="bash" %}}

Then test consolidation:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step32-scale-zero.sh" language="bash" %}}

After your `consolidateAfter` window expires (1 minute with the defaults
above), the controller emits the disruption sequence over the next ~60–90 s
as the node drains and terminates. Total wall-clock from scale-to-zero to
node deletion is typically 2–3 minutes:

```
"message":"disrupting node(s)","command":"Empty/...: delete: nodepools=[default]","decision":"delete"
"message":"tainted node","taint.Key":"karpenter.sh/disrupted","taint.Effect":"NoSchedule"
"message":"deleted node","Node":{"name":"..."}
```

Confirm the node is gone:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step33-confirm-consolidation.sh" language="bash" %}}

If both come back empty, the install is healthy end-to-end.

### Final install checklist

Before moving on to Step 10, the following should all be true:

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

## 10. Pin critical add-ons to the static nodegroup

Before shifting any real workloads onto Karpenter, pin cluster-critical
control-plane add-ons
([CoreDNS](https://docs.aws.amazon.com/eks/latest/userguide/managing-coredns.html),
[metrics-server](https://docs.aws.amazon.com/eks/latest/userguide/metrics-server.html),
the [VPC CNI](https://docs.aws.amazon.com/eks/latest/userguide/managing-vpc-cni.html)
DaemonSet, and the Karpenter controller itself) to your existing static
nodegroup. Without this, Karpenter can pick a node hosting CoreDNS for
consolidation and the cluster takes a brief DNS outage every time it scales
down. The
[EKS scale-cluster-services best practices](https://docs.aws.amazon.com/eks/latest/best-practices/scale-cluster-services.html)
also recommend tuning CoreDNS lameduck and readiness for clusters with
churning nodes.

The Karpenter controller is already pinned (Step 7's `nodeSelector`). For
CoreDNS and metrics-server, add a `nodeAffinity` that prefers your static
nodegroup label:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step34-pin-addons.sh" language="bash" %}}

The VPC CNI is a DaemonSet, so it lands on every node by design — no
pinning needed.

## 11. Shift workloads to Karpenter

Karpenter is now running but isn't yet doing anything for your real
workloads — every existing pod is still scheduled to a static nodegroup.
Migration is incremental and reversible: you opt workloads in by either
removing the nodegroup affinity that pinned them, or by adding a
`nodeSelector` that matches the Karpenter NodePool label
(`provisioner: karpenter` in the example from Step 8). Apply with a rolling
restart so new pods land on Karpenter-launched nodes:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step35-rollout-restart.sh" language="bash" %}}

Once a workload has soaked successfully on Karpenter, scale the static
nodegroup it used to live on **down a few instances at a time**, watching
for any pod that lands back on the remaining static nodes (a sign of a
missed selector). Wait for steady state, then scale down further:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step36-scale-static-nodegroup.sh" language="bash" %}}

Keep a minimum of 2 nodes in the nodegroup that hosts the Karpenter
controller and the pinned add-ons — it must not autoscale to zero, and it
should not be the same nodegroup Karpenter is replacing.

{{% alert title="Note" color="primary" %}}
If you have workloads that must not run on spot (long-running batch,
stateful services, anything without a checkpoint mechanism), add a second
NodePool with `capacity-type: ["on-demand"]` and a distinct label
(e.g. `tier: critical`), and pin those workloads with
`nodeSelector: {tier: critical}`.
{{% /alert %}}

{{% alert title="Warning" color="warning" %}}
**Do not scale the controller's host nodegroup to zero.** If you need to
move the controller onto a different nodegroup later, run `helm upgrade
--reuse-values --set nodeSelector.<new-label-key>=<new-label-value>` first
and confirm the controller pods reschedule before scaling the original
nodegroup down.
{{% /alert %}}

## 12. Operational follow-ups

You don't need any of these to make Karpenter work, but every production
install should have them.

1. **Scrape Karpenter metrics.** Karpenter exposes
   [Prometheus metrics]({{< ref "../../reference/metrics" >}}) on port
   `8000`. If you use [Amazon Managed Prometheus
   (AMP)](https://docs.aws.amazon.com/prometheus/latest/userguide/what-is-Amazon-Managed-Service-Prometheus.html),
   add a `ServiceMonitor` or scrape config pointing at
   `karpenter.kube-system.svc:8000/metrics`. Karpenter also publishes a
   [reference Grafana dashboard]({{< ref "../getting-started-with-karpenter#monitoring-with-grafana-optional" >}})
   you can import.
2. **Set a billing alarm.** The
   [EKS Karpenter best-practices guide](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_create_billing_alarms_to_monitor_your_compute_spend)
   recommends a
   [CloudWatch billing alarm](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/monitor_estimated_charges_with_cloudwatch.html)
   tied to a dollar threshold, since `limits` apply per NodePool, not per
   cluster.
3. **Pin the controller pods off Karpenter-managed nodes** with a
   `nodeSelector`/`affinity` (Step 7 already covers this if you have a system
   nodegroup).
4. **Plan AMI rotation.** When a new
   [AL2023 AMI](https://docs.aws.amazon.com/eks/latest/userguide/al2023.html)
   ships, bump `ALIAS_VERSION` in the `EC2NodeClass`. Karpenter will
   [drift]({{< ref "../../concepts/disruption#drift" >}}) existing
   nodes during their next consolidation cycle, throttled by
   [`disruption.budgets`]({{< ref "../../concepts/disruption#nodepool-disruption-budgets" >}}).
5. **Write more
   [NodePools]({{< ref "../../concepts/nodepools" >}})** if you have
   workloads with different needs — GPU, ARM, dedicated tenancy. Make them
   [mutually exclusive](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html#_create_nodepools_that_are_mutually_exclusive_or_weighted)
   (different taints or labels) or use `weight` to break ties.

## 13. Cleanup

If you need to remove Karpenter — to roll back a failed install, or to
return the cluster to its pre-Karpenter static-only state — tear down in
reverse order. The `EC2NodeClass` and `NodePool` go first so Karpenter
cleanly drains its own nodes back onto your static nodegroups (or evicts the
pods if no node has capacity, in which case scale the static nodegroup up
first).

If you came back to clean up in a fresh shell, re-derive the env vars and
discovery state first (`$SUBNETS` and `$CLUSTER_SG` from Step 6 don't
survive a new shell):

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step37-cleanup-rederive-vars.sh" language="bash" %}}

Then tear down in this order. Pick the tab that matches the path you
took in Step 3:

{{< tabpane text=true right=false >}}
  {{% tab header="**Cleanup**:" disabled=true /%}}
  {{% tab header="With interruption queue" %}}
{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step38-cleanup-teardown.sh" language="bash" %}}
  {{% /tab %}}
  {{% tab header="Without interruption queue" %}}
{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step38b-cleanup-teardown-no-queue.sh" language="bash" %}}
  {{% /tab %}}
{{< /tabpane >}}

The cluster is back to its pre-Karpenter state and a follow-up reinstall
of Steps 1–9 will succeed.

{{% alert title="Note" color="primary" %}}
If `aws cloudformation wait stack-delete-complete` fails for
`Karpenter-${CLUSTER_NAME}` with `DELETE_FAILED`, the stack's IAM managed
policies are still attached to a role somewhere — usually a leftover
`KarpenterNodeRole-${CLUSTER_NAME}` (created by an earlier `eksctl create
nodegroup`, or by a Terraform module that created the node role
separately). Find and detach:

{{% script file="./content/en/{VERSION}/getting-started/adding-karpenter-to-existing-eks/scripts/step39-cleanup-detach-leftover-roles.sh" language="bash" %}}
{{% /alert %}}

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `kubectl get nodepool` shows `NodeRegistrationHealthy=Unknown` and no node ever registers | [Access entry](https://docs.aws.amazon.com/eks/latest/userguide/access-entries.html) / [`aws-auth`](https://docs.aws.amazon.com/eks/latest/userguide/auth-configmap.html) not patched (Step 5), or the node role is missing a managed policy |
| `kubectl describe nodeclaim` shows `Launched=False` with `RunInstancesAuthCheckFailed` | A customer [SCP](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_policies_scps.html) is denying `ec2:RunInstances` — check [CloudTrail](https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-user-guide.html) for the failing dry-run event |
| Controller log: `Unauthorized` against EC2 or EKS API | [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) trust policy is wrong, or the controller policies didn't attach |
| Controller log: `failed to discover subnets` | Subnets are not tagged with `karpenter.sh/discovery=${CLUSTER_NAME}` (see [`subnetSelectorTerms`]({{< ref "../../concepts/nodeclasses#specsubnetselectorterms" >}})) |
| NodeClaim launches but `Registered=Unknown` | Most often the access entry / `aws-auth` row, or a [security group not allowing kubelet → control plane](https://docs.aws.amazon.com/eks/latest/userguide/sec-group-reqs.html) on TCP/443. If you tagged a node SG that doesn't have the right ingress, untag it. |
| `kubectl get pods` for the controller shows `CrashLoopBackoff` | Almost always the IRSA annotation on the [ServiceAccount](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) is missing or pointing at the wrong role |

For deeper troubleshooting, see the upstream [Karpenter
Troubleshooting]({{< ref "../../troubleshooting" >}}) guide.

## Next steps

### Reference

- [NodePool concepts]({{< ref "../../concepts/nodepools" >}}) — full
  field reference.
- [EC2NodeClass concepts]({{< ref "../../concepts/nodeclasses" >}}) —
  AMI aliases, custom user-data, etc.
- [Disruption]({{< ref "../../concepts/disruption" >}}) — drift,
  consolidation, expiration, interruption.
- [EKS Karpenter Best Practices guide](https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html) —
  production-readiness checklist.

### Hands-on workshops

- [EKS Karpenter Workshop](https://catalog.workshops.aws/karpenter/en-US) —
  the canonical AWS-hosted lab; provisions a cluster and walks through
  consolidation, drift, spot, and disruption budgets.
- [EC2 Spot Workshop for Karpenter](https://ec2spotworkshops.com/karpenter.html) —
  focused on spot diversification and interruption handling.
- [Advanced EKS Immersion: Karpenter](https://www.eksworkshop.com/docs/autoscaling/compute/karpenter/) —
  intermediate/advanced module within the broader EKS workshop. Covers
  multi-NodePool design, GPU workloads, and Spot fallback patterns.

### Patterns and examples

- [Karpenter Blueprints](https://github.com/aws-samples/karpenter-blueprints) —
  reusable NodePool / EC2NodeClass examples for common scenarios (GPU,
  ARM, batch, multi-tenant).
- [Tutorial: Run Kubernetes Clusters for Less with EC2 Spot and Karpenter](https://community.aws/tutorials/run-kubernetes-clusters-for-less-with-amazon-ec2-spot-and-karpenter) —
  end-to-end cost-optimization walkthrough including spot interruption
  simulation.
