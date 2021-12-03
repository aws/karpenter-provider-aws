#!/bin/bash
# Extract from helm-testing-action: https://github.com/buttahtoast/helm-testing-action/blob/v2.0.1/scripts/docs.sh#L24

set -eu -o pipefail

CHART_PATH="$(pwd)/charts/karpenter"

main() {
    generateREADMEShasum
    runHelmDocsAndCheck
}

generateREADMEShasum() {
    SHASUM=$(shasum ${CHART_PATH}/README.md)
}

runHelmDocsAndCheck() {
    helm-docs > /dev/null

    if [[ $(shasum "${CHART_PATH}/README.md") == "${SHASUM}" ]]; then
        echo "Documentation up to date ✔"
        exit 0
    else
        echo -e "Checksums did not match - Documentation outdated! ❌\n  Before: ${SHASUM}\n  After: $(shasum ${CHART_PATH}/README.md)\n  ↳ $ Execute helm-docs and push again"
        "${CHART_PATH}/README.md.sum"
        exit 1
    fi
}

main "$@"