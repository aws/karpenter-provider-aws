# Allocatable Diff Tool

The allocatable diff tool iterates through your list of currently deployed nodes and compares them to Karpenter's expectation of the capacity and allocatable on these nodes. It outputs a CSV file that can be used for further analysis to compare the values like expected capacity and allocatable capacity to determine values like vmMemoryOverheadPercent in the AWS cloudprovider.

## Usage

```bash
export CLUSTER_NAME=karpenter-demo
./allocatable-diff --cluster-name=$CLUSTER_NAME --out-file=allocatable-diff.csv
```