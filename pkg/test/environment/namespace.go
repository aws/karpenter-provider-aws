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

package environment

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"go.uber.org/multierr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ellistarn/karpenter/pkg/utils/project"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

var (
	YAMLDocumentDelimiter = regexp.MustCompile(`\n---\n`)
)

type Namespace struct {
	client.Client
	v1.Namespace
}

// Returns a test namespace
func NewNamespace(client client.Client) *Namespace {
	return &Namespace{
		Client: client,
		Namespace: v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: strings.ToLower(randomdata.SillyName()),
			},
		},
	}
}

// Instantiates any number of test resources from a given YAML file
func (n *Namespace) ParseResources(path string, objects ...runtime.Object) error {
	var err error
	for _, object := range objects {
		err = multierr.Append(err, n.ParseResource(path, object))
	}
	return err
}

// Instantiates a test resources from a given YAML file
func (n *Namespace) ParseResource(path string, object runtime.Object) error {
	data, err := ioutil.ReadFile(project.RelativeToRoot(path))
	if err != nil {
		return fmt.Errorf("reading file %s, %w", path, err)
	}
	if err := parseFromYaml(data, object); err != nil {
		return fmt.Errorf("parsing yaml, %w", err)
	}

	if field := reflect.ValueOf(object).Elem().FieldByName("Namespace"); field.IsValid() {
		field.SetString(n.Name)
	}
	return nil
}

// Attempts to parse a resource from a YAML manifest that potentially contains
// multiple objects. Succeeds on the first successful resource.
func parseFromYaml(data []byte, object runtime.Object) error {
	var result error
	for _, document := range YAMLDocumentDelimiter.Split(string(data), -1) {
		if err := yaml.UnmarshalStrict([]byte(document), object); err != nil {
			result = multierr.Append(result, err)
		} else {
			return nil
		}
	}
	return result
}
