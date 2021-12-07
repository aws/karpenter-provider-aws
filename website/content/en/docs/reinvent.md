# Karpenter re:Invent 2021 Builders Session
​
![](https://github.com/aws/karpenter/raw/main/website/static/banner.png)
​
## Prerequisites
Please install the following tools before starting:
- [AWS CLI](https://aws.amazon.com/cli/). If you're on macOS and have [Homebrew](https://brew.sh/) installed, simply `brew install awscli`. Otherwise, follow the AWS CLI [user's guide](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html).
- [Helm](https://helm.sh/docs/intro/install/), the Kubernetes package manager. If you're on macOS, feel free to simply `brew install helm`. Otherwise, follow the [Helm installation guide](https://helm.sh/docs/intro/install/).
​
## Get Started
Once you have all the necessary tools installed, configure your shell with the credentials for the temporary AWS account created for this session by:
1. Navigating to the Event Engine team dashboard and clicking on the "☁️ AWS Console" button
2. Configuring your shell with the credentials required by copy and pasting the command for your operating system.
3. Running the following to set your `AWS_ACCOUNT_ID` environmental variable:
    ```bash
    export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
    ```
4. Updating your local Kubernetes configuration (`kubeconfig`) by running:
    ```bash
    aws eks update-kubeconfig --name karpenter-demo --role-arn arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterEESetupRole-karpenter-demo
    ```
5. Creating an AWS [IAM service-linked role](https://docs.aws.amazon.com/IAM/latest/UserGuide/using-service-linked-roles.html) so that Karpenter can provision Spot EC2 instances with the following command:
    ```bash
    aws iam create-service-linked-role --aws-service-name spot.amazonaws.com
    ```
    _**N.B.** If the role was created previously, you will see:_
    ```bash
    # An error occurred (InvalidInput) when calling the CreateServiceLinkedRole operation: Service role name AWSServiceRoleForEC2Spot has been taken in this account, please try a different suffix.
    ```
​
If you can run the following command and see the pods running in your EKS cluster, you're all set! If not, please ask for help from one of the speakers in the session and they'll get you squared away. For your reference, the cluster name is `karpenter-demo`.
```bash
kubectl get pods -A
```
​
Congratulations! You now have access to an Amazon EKS cluster with an EKS Managed Node Group as well as all the AWS infrastructure necessary to use Karpenter.
Happy Building 🔨!
​
## Install Karpenter
 Use the following command to install Karpenter into your cluster:
```bash
helm repo add karpenter https://charts.karpenter.sh
helm repo update
helm upgrade --install karpenter karpenter/karpenter --namespace karpenter \
  --create-namespace --set serviceAccount.create=false --version {{< param "latest_release_version" >}} \
  --set controller.clusterName=karpenter-demo \
  --set controller.clusterEndpoint=$(aws eks describe-cluster --name karpenter-demo --query "cluster.endpoint" --output json) \
  --wait # for the defaulting webhook to install before creating a Provisioner
```
​
## Next Steps
If you're a Kubernetes expert, feel free to start exploring how Karpenter works on your own and if you have any questions, one of the AWS speakers will be happy to answer them.
​
If you'd like a guided walkthrough of Karpenter's features and capabilities, you can follow the Karpenter Getting Started guide starting at the ["Provisioner" step](https://karpenter.sh/docs/getting-started/#provisioner). Please don't hesitate to ask your AWS speaker any questions you might have!
