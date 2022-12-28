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

package adoption

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/rand"
)

// Adheres to the names.NameGenerator interface
// https://pkg.go.dev/k8s.io/apiserver/pkg/storage/names
type machineNameGenerator struct{}

var MachineNameGenerator machineNameGenerator

var (
	maxNameLength          = 63
	randomLength           = 12
	MaxGeneratedNameLength = maxNameLength - randomLength
)

func (machineNameGenerator) GenerateName(base string) string {
	if len(base) > MaxGeneratedNameLength {
		base = base[:MaxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, rand.String(randomLength))
}
