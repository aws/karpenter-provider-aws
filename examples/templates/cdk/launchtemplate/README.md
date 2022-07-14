## Launch Template CDK Example

CDK example stack to create a custom Karpenter launch configuration. 

### Setup:

1: [install CDK](https://docs.aws.amazon.com/cdk/v2/guide/work-with.html).

2: Configure the environment: 
```
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cdk synth #debug
```
3: configure parameters inside `cdk.context.json`

### Deploy

```
pip install -r requirements.txt
cdk deploy KarpenterLaunchTemplateStack
```

Parameters can be overridden via CLI: 

```
cdk deploy KarpenterLaunchTemplateStack -c pImageId=ami-0d4d6a41cfd1a7d94 -c pClusterName=TestCluster -c pSecurityGroupIDs=sg-016f9639674e51285,sg-01da7b4d552125e6c
```
