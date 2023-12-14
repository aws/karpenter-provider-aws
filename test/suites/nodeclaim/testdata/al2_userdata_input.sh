MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="BOUNDARY"

--BOUNDARY
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1
/etc/eks/bootstrap.sh '%s' --apiserver-endpoint '%s' --b64-cluster-ca '%s' \
--use-max-pods false \
--container-runtime containerd \
--kubelet-extra-args '--node-labels=karpenter.sh/nodepool=%s,testing/cluster=unspecified'

--BOUNDARY--
