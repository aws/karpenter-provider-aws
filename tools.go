// +build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/ko/cmd/ko"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kubebuilder/cmd"
)
