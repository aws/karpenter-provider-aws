---
title: "Deployment Guide"
linkTitle: "Deployment Guide"
weight: 50
---
# notes
- scope to AWS, AWS requirements

# Resources Karpenter Expects/Creates

- Cluster on AWS
    - min version 1.16? 1.18?
    - must be EKS?
- IAM Roles
    - document instance profile vs roles? instance profile that contains a role
    - `KarpenterNodeInstanceProfile-${ClusterName}` -- run containers, configure networking
        - name does matter
        - instance profile wrapper around a role
    - `KarpenterControllerRole-<cluster-name>` -- provision new instances
    - IRSA vs kube2iam?
- Security Groups
    - EKS does by default?
    - can be customized by customers?
    - more than one is a problem? breaks LB controller?
    - exactly one - tag with `k.io/cluster/<name>`
    - attach this SG to every node created
    - nodes can have more than 1. 
    - 2 different mechanisms in Karpenter, provisioner docs
        - add your own SGs to nodes, by name or tag key
    - rules needed
        - created by EKS?
        - access ECR?
        - access to control plane, API server,  
    - tldr, nodes need a firewall rule, speak to control plane (eg eks provided control plane)
        - if not using EKS, on your own. 
- Service Accounts
    - `Karpenter` -- associated with `Karpenter` IAM Role
    - k8s RBAC - link to github
        - installed as part of helm chart
    - https://github.com/awslabs/karpenter/blob/main/charts/karpenter/templates/webhook/rbac.yaml
    - https://github.com/awslabs/karpenter/blob/main/charts/karpenter/templates/controller/rbac.yaml
- Tagged Subnets? 
    - Karpenter discovers subnets using tag
    - strict requirement
    - look at all subnets tagged, auto selected based on matching?
    - must be subnet in that zone of region?
    - does workload want specific zone? pick subnet with that tag?
    - multiple in zone, then choose randomly
- EKS?
    - configure cluster to accept new instances as nodes 
    - must use eksctl? find cfn?
    - all amazon based k
        - config map 
        - ` k get configmaps -n kube-system aws-auth -oyaml`
        - mapping of roles? if you have this role ARN, then the cluster will admit you
        - eksctl
            ```
            eksctl create iamidentitymapping \
            --username system:node:{{EC2PrivateDNSName}} \
            --cluster  ${CLUSTER_NAME} \
            --arn arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME} \
            --group system:bootstrappers \
            --group system:nodes
            ```
- Launch template
    - handwritten notes, 10AUG2021


# Deploy Karpenter

- everything is in helm chart, except auth and subnets, possibly SGs

- Helm Chart
    - manual option? no
    - pulls image from ECR
    - creates deployment
    - config bits of helm chart: https://github.com/awslabs/karpenter/blob/main/charts/karpenter/values.yaml

# Karpenter Changes Bx of Kubernetes?
- `k delete node`
- provisioning
    - watches for event
    - optimistic behaviors
    - preregisters node -- is unusual?
    - preassigns pods to expected node -- unusual?
- deprovisioning 
    - node deletion process might be suprising
    - uninstall karpenter, manually delete finalizer
    - pod disruption budget?
    - finalizer on nodes?
    - expiration of nodes?
    - deletes empty nodes?


- unusually high ratio of pods to per node daemonsets
    - not always true
- potentially larger instances than you used previously 
