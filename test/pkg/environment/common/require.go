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
	"strings"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (env *Environment) RequireCRD(obj client.Object) {
	GinkgoHelper()
	gvk := lo.Must(apiutil.GVKForObject(obj, env.Client.Scheme()))
	if _, err := env.Client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version); meta.IsNoMatchError(err) {
		Skip(fmt.Sprintf("[unsupported:%s-crd] %s isn't installed in this cluster", strings.ToLower(gvk.Kind), gvk.GroupKind()))
	} else {
		Expect(err).ToNot(HaveOccurred())
	}
}
