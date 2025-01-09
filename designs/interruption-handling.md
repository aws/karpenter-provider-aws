# Spot Interruption Handling in Karpenter

**Author: Brandon Wagner (wagnerbm@)**

## Goals

* Gracefully drain EC2 instances when a Spot Interruption Notification is received

## Background

[Spot](https://aws.amazon.com/ec2/spot/) is an offering within EC2 where spare VM capacity is sold for steep discounts. Spot instances are the same underlying hardware as on-demand instances, however Spot instances can be interrupted with a 2-minute notification sent by EC2. Kubernetes (K8s) users generally handle interruptions by cordoning the node and draining pods from the node using the[K8s pod eviction API.](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/) 

The eviction API respects Pod Disruption Budgets (PDBs) and the pod’s `terminationGracePeriod` which is the amount of time the pod requires to gracefully shut down. The pod shutdown process usually involves catching the SIGINT signal and performing connection draining or data checkpointing. The eviction API allows `terminationGracePeriod` to be overridden per pod. 

Another Spot event that users receive is a Rebalance Recommendation, which is a signal indicating a Spot instance is at an increased risk of interruption.  Rebalance Recommendations are always sent, but in the worst case will be sent at the same time as a Spot Interruption Notification. Generally, Rebalance Recommendations are sent 10-20 minutes before an interruption. The main use-case is to provide applications that are interruption tolerant, but need a longer shutdown window than the 2-minutes that a Spot Interruption Notification gives. 

Users generally handle Rebalance Recommendations in a couple of ways. One of which is to cordon and drain the node similar to a Spot Interruption Notification. Another way is to only cordon the node to prevent new workloads from scheduling on the capacity that is at higher risk for interruption, but leaving the existing workloads running so that they have a chance to finish before being interrupted. Alternatively, users can choose to completely ignore the recommendations.

EKS Managed Node Groups (MNG) have built-in support for Spot interruptions and Rebalance Recommendation handling, via EC2 Auto-Scaling Group (ASG) lifecycle termination hooks and ASGs [capacity-rebalance](https://docs.aws.amazon.com/autoscaling/ec2/userguide/ec2-auto-scaling-capacity-rebalancing.html) feature.   

A popular tool to handle Spot interruptions within K8s that are not running on EKS MNG is the [aws-node-termination-handler](https://github.com/aws/aws-node-termination-handler) (NTH). NTH is vended as a helm chart, plain yaml manifests, and offered as an add-on within kOps. As of July 2022, NTH has 250M+ container pulls and 1.2k stars on Github, making it a popular choice for users not running on MNG.  

## How are Spot Interruption Notifications and Rebalance Recommendations vended?

There are two ways in-which Spot interruption notifications and Rebalance Recommendations are vended:

**1:  EC2 Instance Metadata Service** 

EC2 IMDS is an HTTP API that can only be locally accessed from an EC2 instance.

```
# Termination Check
curl 169.254.169.254/latest/meta-data/spot/instance-action
{
    "action": "terminate",
    "time": "2022-07-11T17:11:44Z"
}

# Rebalance Check
curl 169.254.169.254/latest/meta-data/events/recommendations/rebalance
{
    "noticeTime": "2022-07-16T19:18:24Z"
}

```

**2:  EventBridge**

EventBridge is an Event Bus service within AWS that allows users to set rules on events to capture and then target destinations for those events. Relevant targets for Spot interruption notifications include SQS, Lambda, and EC2-Terminate-Instance.

```
# Example spot interruption notification EventBridge rule
aws events put-rule \
 --name MyK8sSpotTermRule \
 --event-pattern "{\"source\": [\"aws.ec2\"],\"detail-type\": [\"EC2 Spot Instance Interruption\"]}"

# Example rebalance recommendation EventBridge rule
aws events put-rule \
 --name MyK8sRebalanceRule \
 --event-pattern "{\"source\": [\"aws.ec2\"],\"detail-type\": [\"EC2 Instance Rebalance Recommendation\"]}"

# Example targeting an SQS queue
aws events put-targets --rule MyK8sSpotTermRule \
 --targets "Id=1,Arn=arn:aws:sqs:us-east-1:123456789012:MyK8sTermQueue"
```



## Proposed  Solutions

### Option 1 - Karpenter Managed Queueing  Infrastructure

#### No API Changes.

Karpenter dynamically provisions the queueing infrastructure to handle Spot Interruption Notifications, Rebalance Recommendations, and AWS Health Events. 

We would need to setup:

1. EventBridge Rules for Spot and Health events
2. An SQS Queue for the rule target to send to

Karpenter would monitor the Queue(s) for interruption notifications and perform a graceful cordon and drain of the interrupted node with the existing termination flow. The controller to setup the SQS queue, EventBridge rules, and fetch the events would need to reside within the AWS Cloud Provider. When a Spot Interruption Notification is received within the controller, it will emit a K8s event and delete the node triggering the finalizer to perform a graceful drain. 

Another consideration with EventBridge rules is that it is not possible to filter Spot Interruption Notifications for instances with a specific tag, so Karpenter’s queue would receive all Spot interruption notifications within a region for the account it is deployed in. Karpenter would simply delete Spot Interruption Notifications on the queue that do not belong to a node it manages.

Rebalance Recommendations would be consumed but only emitted as a K8s Event, at least in the initial release. There’s not a safe default action when handling Rebalance Recommendations so it’s best that we default to off until we have some demand for it. We will likely need to add a parameter to the AWSNodeTemplate CRD to provide the action to take on a Rebalance Recommendation (cordon or drain). The implementation will be trivial at that point since Karpenter will already be consuming the Rebalance Recommendation events.   

SQS Queues use a [consumption pricing model](https://aws.amazon.com/sqs/pricing/) where the first 1 million requests per month are free across all SQS API requests. It would be unlikely to breach this free tier by Karpenter alone. The next pricing tier of requests is priced at $0.40 per million requests, so the cost of this approach is fairly small.  EventBridge rules are [free](https://aws.amazon.com/eventbridge/pricing/) to be received from AWS systems.

SQS exposes a VPC Endpoint which will fulfill the isolated VPC use-case. 

*Example EventBridge Spot Interruption Notification:*

```
{
  "version": "0",
  "id": "1e5527d7-bb36-4607-3370-4164db56a40e",
  "detail-type": "EC2 Spot Instance Interruption Warning",
  "source": "aws.ec2",
  "account": "123456789012",
  "time": "2022-08-11T14:00:00Z",
  "region": "us-east-2",
  "resources": [
    "arn:aws:ec2:us-east-2:instance/i-0123456789"
  ],
  "detail": {
    "instance-id": "i-0123456789",
    "instance-action": "terminate"
  }
}
```

#### Security Implications:

Dynamically creating the SQS infrastructure and EventBridge rules means that Karpenter’s IAM role would need permissions to SQS and EventBridge:

```
"sqs:GetQueueUrl",
"sqs:ListQueues",
"sqs:ReceiveMessage",
"sqs:CreateQueue",
"sqs:DeleteMessage",
"events:ListRules",
"events:DescribeRule",
"events:PutRule", 
"events:PutTargets",
"events:DeleteRule",
"events:RemoveTargets"
```

The policy can be setup with a predefined name based on the cluster name. For example, `karpenter-events-${CLUSTER_NAME}` which would allow for a more constrained resource policy. 

When Karpenter is uninstalled, queue deletion should occur. This is difficult to achieve since we won’t know if the controller is being upgraded or Karpenter is legitimately being removed. One potential solution is to Delete the queue and EventBridge resources when all Provisioners have been deleted. 

### Option 2 - System Daemon

**No API changes.**

A small DaemonSet or system daemon can be deployed to all nodes, or only Spot nodes, that monitors the IMDS endpoint for Spot interruption notifications. When the IMDS endpoint returns a Spot interruption timestamp, the daemonset would trigger a cordon and drain for the node. This could also be a label or taint on the node that Karpenter reconciles to reuse Karpenter’s termination logic and use less rbac permissions. This is basically NTH running in IMDS mode. 

There are several paths that could be taken with this approach outlined below:

**3A: NTH IMDS mode as a Helm Chart Dependency**

The simplest option is to include [NTH IMDS mode](https://quip-amazon.com/EUgPAQpKMbAj/Spot-Interruption-Handling-in-Karpenter#temp:C:fAR987d60b057584fdf84525fba6) as a dependent helm chart to install alongside Karpenter with a node selector targeting `karpenter.sh/capacity-type: spot` . This would require minimal dev work and would give us access to the NTH team for support.

**3B: Build a System Daemon (nthd)**

An option to transparently handle spot interruption notifications is to build a system daemon in a separate repo that performs the IMDS monitoring and triggers an instance shutdown when an interruption is observed. This would rely on K8s’ new [graceful shutdown](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown) feature which went beta in K8s 1.21. 

With graceful shutdown, the kubelet registers [systemd-inhibitor-locks](https://www.freedesktop.org/wiki/Software/systemd/inhibit/) to stop the shutdown flow until locks are relinquished, which in this case would be when the kubelet has drained pods off of the node. Two parameters were added to the kubelet to tune the drain timeouts:  `shutdownGracePeriod` & `shutdownGracePeriodCriticalPods`

Upon receiving a spot interruption notification, the system daemon would use the [dbus](https://www.freedesktop.org/wiki/Software/dbus/),  a mechanism for interprocess communication, to send a `power-off-multiple-sessions` to the operating system. The systemd-inhibitor locks would stop the shutdown flow while the kubelet cordons and gracefully drains the pods from the node.  

There are security considerations with a system daemon having permissions to shutdown the whole operating system. The daemon can run as an unprivileged user and gain permissions to send the `power-off-multiple-sessions` signal via [PolicyKit](https://www.freedesktop.org/software/polkit/docs/latest/polkit.8.html) (PolKit), which is an authorization API used for dbus. The attack surface is minimal though. An example attack would be a malicious actor replacing the IMDS response from the `/spot` path. But this would require root permissions to load an iptables rule and so the malicious actor would already have permissions to shutdown the instance.

Another consideration for running a system-daemon without K8s permissions is the observability of Spot related events. There would be no way to convey that a node was interrupted versus some sort of system failure. 

The project would be in a stand-alone github repo or within the NTH repo and be a general purpose AWS Spot Interruption handling mechanism that could be installed via user-data or baked into AMIs. 

Unfortunately, AL2’s default version of systemd does not support inhibitor locks properly, so K8s graceful shutdown will not work.

**3C: Build a Spot Interruption Node Problem Detector monitor**

The [Node Problem Detector](https://github.com/kubernetes/node-problem-detector) (NPD) is a DaemonSet within the Kubernetes github org that identifies common node problems like Kernel Deadlock, NTP issues, Kubelet health issues, etc. and then emits K8s `Events` or adds a `NodeCondition`. 

GKE uses NPD as a default add-on for all nodes.

A daemon similar to nthd can be built to identify Spot interruption notifications and integrated w/ the NPD. Currently, NPD embeds the monitor daemons and runs them as go-routines. There’s a plan to break the monitors into separate containers and run within the context of a single pod, but this is not yet implemented. 

If we go this route, I think it would make sense to build the aws-node-problem-detector which uses the upstream NPD, but adds AWS specific monitors, one of which would be Spot interruption notifications. Other monitors that could be built for AWS is scheduled maintenance events and rebalance recommendations. We can work with upstream to split into a cloud-provider model as well which would make this easier. 

aws-NPD could be setup as a helm chart dependency or run within the aws-node DaemonSet. 

Karpenter would need to be updated to watch NodeConditions and terminate when a `SpotInterruption` condition is observed. Rebalance Recommendations would be emitted as a K8s Event. 

## Metrics

Users often track metrics around Spot interruptions and Rebalance Recommendations by capacity pool (instance type, zone). Karpenter will emit Prometheus metrics on those dimensions and provide an aggregate interruption rate for the cluster.  

## Health Events 

When running K8s nodes within the context of an EC2 Auto-Scaling Group (ASG), health events for instances such as Status Checks, System Checks, and Scheduled Maintenance Events are handled automatically by ASG.  When the ASG observes one of these events, it will trigger a Termination of the instance, which you are able to setup a lifecycle hook to deal with gracefully. 

Karpenter does not currently receive health related events from EC2 for instances. Proposed solutions that monitor IMDS or EventBridge can receive Scheduled Maintenance Events only.

## Recommendation

I’d recommend *Option 1 - Karpenter Managed Queueing Infrastructure* since it is the most native Karpenter Spot Interruption Handling approach. There’s no additional daemons that users need to install on nodes and there is no infrastructure to setup. In addition, this approach solves for Spot events and health events. 

## EDIT

During implementation we discovered that some users wanted to manage their own SQS and EventBridge infrastructure, so we opted to implement Option 1 but without creating and managing the infrastructure.
