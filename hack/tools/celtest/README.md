# Kubelet CEL Expression Test Tool

`celtest` previews what a kubelet CEL expression would evaluate to *before* it is applied to a cluster. It is
for the `maxPods`, `kubeReserved`, and `systemReserved` fields of an `EC2NodeClass`, which accept CEL
expressions evaluated per instance type.

The tool evaluates expressions through the same code the controller runs — `pkg/cel` plus
`instancetype.PreviewKubeletExpressions` — so the value it prints is the value a real launch would compute,
including the per-field unit suffix and the rules that drop a failed or out-of-range result. It never writes to
a cluster, and its only network call is a read-only `DescribeInstanceTypes`.

## Usage

Offline, with no AWS credentials or network. Supply the instance type's inputs directly:

```bash
go run ./hack/tools/celtest --max-pods-expr 'min(max_pods, vcpus * 10)' \
  --vcpus 2 --memory-mib 8192 --default-enis 3 --ips-per-eni 10
```

Against real instance types, which looks the inputs up from EC2. This is the mode to trust before you apply,
since a surprising result is usually a surprising *input* rather than a bad expression:

```bash
go run ./hack/tools/celtest --instance-type t3.micro,m5.large,c6g.16xlarge --region us-west-2 \
  --max-pods-expr 'min(max_pods, vcpus * 8)' \
  --kube-reserved cpu='max(60, vcpus * 30)' --kube-reserved memory='memory_mib / 100'
```

```
m5.large
  inputs: vcpus=2 memory_mib=8192 default_enis=3 ips_per_eni=10 max_pods=29 instance_type="m5.large"
  note:   max_pods is 29 in the maxPods expression (the AMI-family default) but 16 in the reserved expressions (the resolved maxPods)
  FIELD                 EXPRESSION                RESULT
  maxPods               min(max_pods, vcpus * 8)  16
  kubeReserved[cpu]     max(60, vcpus * 30)       60m
  kubeReserved[memory]  memory_mib / 100          96Mi
```

The tool exits non-zero if any expression would be dropped, so it can be used as a pre-apply check in a script.

## Things the output will tell you

- **Units are implied by the field, and results are rounded up.** An expression returns a bare number in the
  unit that key's static quantities use: millicores for `cpu`, MiB for `memory`, GiB for `ephemeral-storage`,
  a plain count for `pid`. `max(60, vcpus * 30)` on 2 vCPUs is `60m`, not 60 cores. `cpu` is rounded up to the
  nearest 10m and `memory` to the nearest 16Mi, so `memory_mib / 100` of `82` reports as `96Mi`.
- **`max_pods` means two different things.** In a `maxPods` expression it is the AMI-family default (an
  expression can't reference its own result). In a `kubeReserved`/`systemReserved` expression it is the
  *resolved* `maxPods`, including any `--pods-per-core` cap. The tool prints a `note:` line whenever the two
  differ.
- **Failures are dropped, not fatal.** An expression that fails to evaluate, goes negative, or (for `maxPods`)
  overflows int32 is reported as `DROPPED` with the reason, and the field falls back to its default.
- **Static values are not evaluated.** A value that parses as a resource quantity (e.g. `100Mi`) is passed
  through unchanged, so it does not appear in the report.

## Flags

| Flag | Description |
| --- | --- |
| `--max-pods-expr` | CEL expression for `maxPods` |
| `--kube-reserved key=expr` | `kubeReserved` entry, repeatable |
| `--system-reserved key=expr` | `systemReserved` entry, repeatable |
| `--instance-type` | Comma-separated instance types to look up from EC2; omit to run offline |
| `--region` | Region for the lookup, defaulting to the ambient AWS config |
| `--pods-per-core` | Kubelet `podsPerCore`, which caps a resolved `maxPods` |
| `--ami-family` | AMI family determining the default `max_pods` calculation (default `AL2023`) |
| `--reserved-enis` | Matches the controller's `--reserved-enis` setting |
| `--vcpus`, `--memory-mib`, `--default-enis`, `--ips-per-eni` | Offline inputs, used when `--instance-type` is not set |

## Available variables

`vcpus`, `memory_mib`, `default_enis`, `ips_per_eni`, `max_pods`, `instance_type`, plus `min()` and `max()`
over ints and doubles. An expression must return an int or a double.
