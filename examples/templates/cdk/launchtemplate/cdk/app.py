from aws_cdk import aws_ec2 as ec2
from aws_cdk import aws_iam as iam
from aws_cdk import core


class KarpenterLaunchTemplateStack(core.Stack):
    def _build_userdata(self):
        user_data = (
            "#!/bin/bash\n"
            f"/etc/eks/bootstrap.sh '{self.cluster_name}'\n"
            f"--kubelet-extra-args '{self.kubelet_extra_args}'\n"
            f"--b64-cluster-ca '{self.b64_cluster_ca}'\n"
            f"--apiserver-endpoint '{self.apiserver_endpoint}'"
        )
        return user_data

    def __init__(self, scope: core.Construct, id: str, **kwargs) -> None:
        super().__init__(scope, id, **kwargs)

        self.image_id = self.node.try_get_context("pImageId")
        self.cluster_name = self.node.try_get_context("pClusterName")
        self.kubelet_extra_args = self.node.try_get_context("pKubeletExtraArgs")
        self.b64_cluster_ca = self.node.try_get_context("pB64ClusterCa")
        self.apiserver_endpoint = self.node.try_get_context("pApiServiceEndpoint")
        self.security_group_ids = self.node.try_get_context("pSecurityGroupIDs").split(
            ","
        )
        self.instance_tag = f"kubernetes.io/cluster/{self.cluster_name}"

        self.karpenter_node_role = iam.Role(
            scope=self,
            id="KarpenterNodeRole",
            role_name="KarpenterNodeRole",
            assumed_by=iam.ServicePrincipal(f"ec2.{core.Aws.URL_SUFFIX}"),
            managed_policies=[
                iam.ManagedPolicy.from_aws_managed_policy_name(
                    "AmazonEKSWorkerNodePolicy"
                ),
                iam.ManagedPolicy.from_aws_managed_policy_name("AmazonEKS_CNI_Policy"),
                iam.ManagedPolicy.from_aws_managed_policy_name(
                    "AmazonEC2ContainerRegistryReadOnly"
                ),
                iam.ManagedPolicy.from_aws_managed_policy_name(
                    "AmazonSSMManagedInstanceCore"
                ),
            ],
        )

        self.karpenter_node_instance_profile = iam.CfnInstanceProfile(
            scope=self,
            id="KarpenterNodeInstanceProfile",
            roles=[self.karpenter_node_role.role_name],
            instance_profile_name="KarpenterNodeInstanceProfile",
        )

        self.karpenter_launch_template = ec2.CfnLaunchTemplate(
            scope=self,
            id="KarpenterLaunchTemplate",
            launch_template_name="KarpenterCustomLaunchTemplate",
            launch_template_data=ec2.CfnLaunchTemplate.LaunchTemplateDataProperty(
                image_id=self.image_id,
                iam_instance_profile=ec2.CfnLaunchTemplate.IamInstanceProfileProperty(
                    arn=self.karpenter_node_instance_profile.attr_arn
                ),
                user_data=core.Fn.base64(self._build_userdata()),
                block_device_mappings=[
                    ec2.CfnLaunchTemplate.BlockDeviceMappingProperty(
                        device_name="/dev/xvda",
                        ebs=ec2.CfnLaunchTemplate.EbsProperty(
                            volume_size=80, volume_type="gp3"
                        ),
                    )
                ],
                security_group_ids=self.security_group_ids,
                tag_specifications=[
                    ec2.CfnLaunchTemplate.TagSpecificationProperty(
                        resource_type="instance",
                        tags=[core.CfnTag(key=self.instance_tag, value="owned")],
                    )
                ],
            ),
        )


app = core.App()

KarpenterLaunchTemplateStack(scope=app, id=f"KarpenterLaunchTemplateStack")

app.synth()
