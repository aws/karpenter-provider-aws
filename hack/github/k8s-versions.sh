#!/usr/bin/env bash
set -euo pipefail

# Usage: k8s-versions.sh <rev> [--check]
#
# Prints the k8s versions that the E2E workflows test at <rev>, as GITHUB_OUTPUT lines read from e2e-matrix.yaml:
#   K8S_VERSION   the default version (the workflow_call default)
#   K8S_VERSIONS  every supported version as a JSON list (the workflow_dispatch options)
# Files are read with git show, so nothing from <rev> is checked out or run. The values flow into workflow expressions,
# so anything that isn't a 1.N version fails.
# --check also fails if the list has gaps or another file that pins a k8s version disagrees with e2e-matrix.yaml.
# Needs git, yq and jq.

rev="$1"
check="${2:-}"
matrix=".github/workflows/e2e-matrix.yaml"

read_yaml() {
  git show "${rev}:$1" | yq -o=json -I=0 "$2"
}

version="$(read_yaml "${matrix}" '.on.workflow_call.inputs.k8s_version.default')"
versions="$(read_yaml "${matrix}" '.on.workflow_dispatch.inputs.k8s_version.options')"
# -n with --argjson makes the result depend on every input, and fails if either read yields more than one value
# (a multi-document file). length <= 12 caps the credentialed versionCompatibility matrix fan-out
if ! jq -en --argjson v "${version}" --argjson vs "${versions}" '($v | type) == "string" and ($vs | type) == "array"
    and ($vs | length) > 0 and ($vs | length) <= 12
    and all($vs[]; type == "string" and test("^1\\.[0-9]{1,3}\\z")) and ($vs | index($v)) != null' > /dev/null; then
  echo "::error::${matrix} at ${rev}: k8s_version default ${version} and options ${versions} must be 1.N versions, and the options must include the default" >&2
  exit 1
fi
version="$(jq -r . <<< "${version}")"
echo "K8S_VERSION=${version}"
echo "K8S_VERSIONS=${versions}"

[[ "${check}" == "--check" ]] || exit 0

failed=false
expect() {
  if [[ "$3" != "$4" ]]; then
    echo "::error file=$1::$2 is $3, expected $4" >&2
    failed=true
  fi
}

# The options run oldest to newest with no gaps, and the default is the newest
minors="$(jq -c 'map(split(".")[1] | tonumber)' <<< "${versions}")"
expect "${matrix}" "k8s_version options (minor versions)" "${minors}" "$(jq -c '[range(.[0]; .[-1] + 1)]' <<< "${minors}")"
expect "${matrix}" "k8s_version default" "${version}" "$(jq -r '.[-1]' <<< "${versions}")"

# MaxK8sVersion may lead the e2e default (a staged bump), but must not trail it
max="$(git show "${rev}:pkg/providers/version/version.go" | sed -nE 's/^[[:space:]]*MaxK8sVersion[[:space:]]*=[[:space:]]*"([^"]*)".*/\1/p')"
if [[ ! "${max}" =~ ^1\.[0-9]{1,3}$ ]] || (( 10#${max#1.} < 10#${version#1.} )); then
  echo "::error file=pkg/providers/version/version.go::MaxK8sVersion is ${max}, expected ${version} or newer" >&2
  failed=true
fi

for f in .github/workflows/e2e.yaml .github/workflows/e2e-upgrade.yaml; do
  expect "${f}" "workflow_call k8s_version default" "$(read_yaml "${f}" '.on.workflow_call.inputs.k8s_version.default')" "\"${version}\""
  expect "${f}" "workflow_dispatch k8s_version default" "$(read_yaml "${f}" '.on.workflow_dispatch.inputs.k8s_version.default')" "\"${version}\""
  expect "${f}" "workflow_dispatch k8s_version options" "$(read_yaml "${f}" '.on.workflow_dispatch.inputs.k8s_version.options')" "${versions}"
done
expect "${matrix}" "workflow_dispatch k8s_version default" "$(read_yaml "${matrix}" '.on.workflow_dispatch.inputs.k8s_version.default')" "\"${version}\""
for f in .github/actions/e2e/install-karpenter/action.yaml .github/actions/e2e/setup-cluster/action.yaml .github/actions/e2e/run-tests-private-cluster/action.yaml; do
  expect "${f}" "k8s_version default" "$(read_yaml "${f}" '.inputs.k8s_version.default')" "\"${version}\""
done
expect .github/actions/install-deps/action.yaml "k8sVersion default" "$(read_yaml .github/actions/install-deps/action.yaml '.inputs.k8sVersion.default')" "\"${version}.x\""
expect .github/workflows/ci-test.yaml "k8sVersion matrix" "$(read_yaml .github/workflows/ci-test.yaml '.jobs["ci-test"].strategy.matrix.k8sVersion')" "$(jq -c 'map(. + ".x")' <<< "${versions}")"
expect .github/workflows/ci-test.yaml "coverage k8sVersion" "$(read_yaml .github/workflows/ci-test.yaml '.jobs["ci-test"].steps[] | select(.name == "Send coverage") | .if')" "\"matrix.k8sVersion == '${version}.x'\""

[[ "${failed}" == "false" ]]
