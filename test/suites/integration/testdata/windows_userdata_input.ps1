<powershell>
[string]$EKSBootstrapScriptFile = "$env:ProgramFiles\Amazon\EKS\Start-EKSBootstrap.ps1"
& $EKSBootstrapScriptFile -EKSClusterName "%s" -APIServerEndpoint "%s" -Base64ClusterCA "%s" -ContainerRuntime "docker"
</powershell>