#!/usr/bin/env bash

TAG="v1.1.3"
URL="https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/${TAG}/pkg/aws/vpc/limits.go"

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 path/to/output.go"
  exit 1
fi

OUTPUT="${1}"
TMP=$(mktemp)
curl "${URL}" -o "${TMP}"
awk '/package vpc/{p=1}{if(p){print}}' "${TMP}" | \
 sed 's/var Limits/var limits/' | \
 sed 's/package vpc/package aws/' > "${OUTPUT}"
rm "${TMP}"

