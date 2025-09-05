MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash -e
# Check if HugePages is activated
sudo cat /proc/sys/vm/nr_hugepages

# activate HugePages and set the kernel parameter value to 2048
sudo sysctl -w vm.nr_hugepages=2048

# Ensure HugePages is allocated after reboot
sudo echo "vm.nr_hugepages=2048" >> /etc/sysctl.conf
sudo grep Huge /proc/meminfo
echo "hugepages user data script has finished successfully."
--==MYBOUNDARY==