MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="BOUNDARY"

--BOUNDARY
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
cd
sudo amazon-linux-extras install epel -y
sudo yum update -y
aws s3 cp --recursive s3://ec2-amd-linux-drivers/latest/ . --no-sign-request
tar -xf amdgpu-pro-*rhel*.tar.xz
cd amdgpu-pro-20.20-1184451-rhel-7.8
./amdgpu-pro-install -y --opencl=pal,legacy
(sleep 30; sudo reboot) &
--BOUNDARY-- 