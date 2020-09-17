package test

import (
	"context"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	"github.com/Pallinder/go-randomdata"
	"github.com/ellistarn/karpenter/pkg/utils"
	"github.com/ellistarn/karpenter/pkg/utils/project"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Namespace struct {
	v1.Namespace
}

// Returns a test namespace
func NewNamespace(client client.Client) *Namespace {
	namespace := &Namespace{
		Namespace: v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: strings.ToLower(randomdata.SillyName()),
			},
		},
	}

	var _ = ginkgo.BeforeEach(func() {
		gomega.Expect(client.Create(context.Background(), &namespace.Namespace)).Should(gomega.Succeed(), "Failed to create namespace")
	})
	var _ = ginkgo.AfterEach(func() {
		gomega.Expect(client.Delete(context.Background(), &namespace.Namespace)).Should(gomega.Succeed(), "Failed to delete namespace")
	})
	return namespace
}

// Instantiates a test resource from YAML
func (n *Namespace) ParseResource(path string, object runtime.Object) error {
	data, err := ioutil.ReadFile(project.RelativeToRoot(path))
	if err != nil {
		return errors.Wrapf(err, "reading file %s", path)
	}
	if err := parseFromYaml(data, object); err != nil {
		return errors.Wrapf(err, "")
	}

	if field := reflect.ValueOf(object).Elem().FieldByName("Namespace"); field.IsValid() {
		field.SetString(n.Namespace.Name)
	}
	return nil
}

func parseFromYaml(data []byte, object runtime.Object) error {
	errs := []error{}
	for _, document := range strings.Split(string(data), "---") {
		if err := yaml.UnmarshalStrict([]byte(document), object); err == nil {
			return nil
		}
	}
	return errors.Wrap(utils.FirstNonNilError(errs), "parsing YAML")
}
