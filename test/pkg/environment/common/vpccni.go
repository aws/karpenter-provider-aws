/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (env *Environment) RequireVPCCNI(need string) {
	GinkgoHelper()
	err := env.Client.Get(env.Context, types.NamespacedName{Namespace: "kube-system", Name: "aws-node"}, &appsv1.DaemonSet{})
	if errors.IsNotFound(err) {
		Skip(fmt.Sprintf("[unsupported:vpc-cni-configurable] %s needs the kube-system/aws-node DaemonSet", need))
	}
	Expect(err).ToNot(HaveOccurred())
}
