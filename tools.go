// +build tools

package tools

import (
	_ "github.com/google/ko/cmd/ko"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
