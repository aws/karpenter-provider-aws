#!/bin/bash
echo "Step 0"
source step00-karpenter-version.sh
echo "Step 1"
source step01-config.sh
echo "Step 2"
source step02-create-cluster.sh
echo "Step 3"
source step03-iam-cloud-formation.sh
echo "Step 4"
source step04-grant-access.sh
echo "Step 5"
source step05-controller-iam.sh
echo "Step 6"
source step06-install-helm-chart.sh
echo "Step 7"
source step07-apply-helm-charts.sh
