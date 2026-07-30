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

// kubeletschema_gen derives an OpenAPI v3 schema for spec.kubelet from the upstream
// kubeletconfigv1beta1.KubeletConfiguration struct that Karpenter compiles against, and
// writes it as a YAML fragment for injection into the EC2NodeClass CRD.
//
// spec.kubelet is an open map in Go (see v1.KubeletConfiguration) so that Karpenter never
// has to know about individual kubelet fields, and so unset is distinguishable from zero
// for the fields it reads for scheduling. Enumerating the fields in the CRD schema is what
// lets the API server reject unknown fields and type errors at admission time rather than
// during node registration. A CEL rule cannot substitute for this: the API server refuses
// to compile x-kubernetes-validations against a map with x-kubernetes-preserve-unknown-fields.
//
// Bumping k8s.io/kubelet in go.mod and re-running this generator is what makes newly
// released kubelet fields available to users.
package main

import (
	"flag"
	"log"
	"os"
	"reflect"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/samber/lo"
)

// quantityPattern matches a serialized resource.Quantity. It mirrors the pattern
// controller-gen emits for Quantity fields, and the one in hack/validation/kubelet.sh.
const quantityPattern = `^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`

var (
	durationType = reflect.TypeOf(metav1.Duration{})
	quantityType = reflect.TypeOf(resource.Quantity{})
	timeType     = reflect.TypeOf(metav1.Time{})
)

// opaqueTypes are upstream types whose Go definitions can't be faithfully expressed as a
// structural schema, so they're accepted as opaque objects instead of being enumerated.
// Validating their contents is the kubelet's job; Karpenter only needs to pass them through.
//
//   - LoggingConfiguration embeds resource.QuantityValue and a TimeOrMetaDuration field
//     carrying no JSON tag, neither of which has a schema representation.
var opaqueTypes = map[string]string{
	"k8s.io/component-base/logs/api/v1.LoggingConfiguration": "logging",
}

// celExpressionFields widen fields where Karpenter deliberately accepts more than the upstream
// Go type does, because the value may be a CEL expression evaluated per instance type before it
// ever reaches the kubelet. Reflection reports the upstream type, so an intentional divergence
// like this can't be derived and has to be stated.
//
// Only maxPods needs widening: upstream types it int32, while Karpenter accepts either an
// integer or an expression string. kubeReserved and systemReserved are already map[string]string
// upstream, so an expression fits their generated schema as-is -- what they need instead is for
// hack/validation/kubelet.sh to not constrain their values to a resource.Quantity pattern.
//
// Keyed by top-level JSON field name. Applied in main rather than during recursion so a nested
// field that happens to share a name can't be caught by accident.
var celExpressionFields = map[string]apiextensionsv1.JSONSchemaProps{
	"maxPods": {
		XIntOrString: true,
		AnyOf: []apiextensionsv1.JSONSchemaProps{
			{Type: "integer"},
			{Type: "string"},
		},
	},
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalf("usage: %s <output-file>", os.Args[0])
	}
	out := flag.Arg(0)

	schema := schemaForStruct(reflect.TypeOf(kubeletconfigv1beta1.KubeletConfiguration{}))
	// spec.kubelet is an inline fragment of a NodeClass, not a standalone object, so the
	// embedded TypeMeta fields aren't meaningful here.
	delete(schema.Properties, "kind")
	delete(schema.Properties, "apiVersion")

	for name, widened := range celExpressionFields {
		// Fail rather than silently skip: if upstream renames or removes the field, the override
		// has gone stale and the CRD would quietly stop accepting expressions for it.
		if _, ok := schema.Properties[name]; !ok {
			log.Fatalf("celExpressionFields names %q, which the upstream kubelet type no longer "+
				"has; update or drop the override", name)
		}
		schema.Properties[name] = widened
	}

	data, err := yaml.Marshal(schema)
	if err != nil {
		log.Fatalf("marshaling schema, %v", err)
	}
	// No generated-file header: the output is a schema fragment spliced into the CRD, and a
	// YAML comment here would be carried into the CRD along with it.
	if err := os.WriteFile(out, data, 0o644); err != nil {
		log.Fatalf("writing %s, %v", out, err)
	}
	log.Printf("wrote %s (%d kubelet fields)", out, len(schema.Properties))
}

// schemaForStruct builds an object schema from a struct's JSON-tagged exported fields.
func schemaForStruct(t reflect.Type) apiextensionsv1.JSONSchemaProps {
	schema := apiextensionsv1.JSONSchemaProps{
		Type:       "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
	}
	for i := range t.NumField() {
		field := t.Field(i)
		// Embedded structs (e.g. metav1.TypeMeta) contribute their own fields inline.
		if field.Anonymous {
			for name, prop := range schemaForStruct(field.Type).Properties {
				schema.Properties[name] = prop
			}
			continue
		}
		name, ok := jsonName(field)
		if !ok {
			continue
		}
		schema.Properties[name] = schemaForType(field.Type, name)
	}
	return schema
}

// jsonName returns a field's serialized name, reporting false if the field isn't
// serialized at all.
//
// Nothing is marked required. Upstream expresses optionality with a +optional comment
// marker rather than the omitempty tag option (KubeletAuthentication.X509, for instance, is
// +optional but has no omitempty), and comment markers aren't visible through reflection.
// Treating a missing omitempty as required would reject valid partial configuration such as
// an authentication block that only sets anonymous.enabled. Since every field of
// spec.kubelet is optional from Karpenter's perspective, omitting required entirely is both
// correct and simpler.
func jsonName(field reflect.StructField) (name string, ok bool) {
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		return "", false
	}
	name, _, _ = strings.Cut(tag, ",")
	if name == "" {
		return "", false
	}
	return name, true
}

// schemaForType maps a Go type to its OpenAPI schema. fieldName is used only to look up
// opaque-type overrides for better error messages.
//
//nolint:gocyclo
func schemaForType(t reflect.Type, fieldName string) apiextensionsv1.JSONSchemaProps {
	// Pointers are indistinguishable from their element type once serialized.
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Types with custom JSON marshaling need their serialized form described explicitly,
	// since their Go structure doesn't reflect what appears on the wire.
	switch t {
	case durationType:
		// metav1.Duration marshals as a Go duration string, e.g. "30s".
		return apiextensionsv1.JSONSchemaProps{Type: "string"}
	case timeType:
		return apiextensionsv1.JSONSchemaProps{Type: "string", Format: "date-time"}
	case quantityType:
		// Quantity accepts either a bare number or a suffixed string, e.g. 1 or "100Mi".
		return apiextensionsv1.JSONSchemaProps{
			XIntOrString: true,
			AnyOf: []apiextensionsv1.JSONSchemaProps{
				{Type: "integer"},
				{Type: "string"},
			},
			Pattern: quantityPattern,
		}
	}
	if name := t.PkgPath() + "." + t.Name(); opaqueTypes[name] == fieldName {
		return apiextensionsv1.JSONSchemaProps{
			Type:                   "object",
			XPreserveUnknownFields: lo.ToPtr(true),
		}
	}

	switch t.Kind() {
	case reflect.Bool:
		return apiextensionsv1.JSONSchemaProps{Type: "boolean"}
	case reflect.Int32, reflect.Uint32:
		return apiextensionsv1.JSONSchemaProps{Type: "integer", Format: "int32"}
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
		return apiextensionsv1.JSONSchemaProps{Type: "integer", Format: "int64"}
	case reflect.Float32, reflect.Float64:
		return apiextensionsv1.JSONSchemaProps{Type: "number"}
	case reflect.String:
		return apiextensionsv1.JSONSchemaProps{Type: "string"}
	case reflect.Slice, reflect.Array:
		// []byte serializes as a base64 string rather than an array.
		if t.Elem().Kind() == reflect.Uint8 {
			return apiextensionsv1.JSONSchemaProps{Type: "string", Format: "byte"}
		}
		return apiextensionsv1.JSONSchemaProps{
			Type: "array",
			Items: &apiextensionsv1.JSONSchemaPropsOrArray{
				Schema: lo.ToPtr(schemaForType(t.Elem(), fieldName)),
			},
		}
	case reflect.Map:
		return apiextensionsv1.JSONSchemaProps{
			Type: "object",
			AdditionalProperties: &apiextensionsv1.JSONSchemaPropsOrBool{
				Allows: true,
				Schema: lo.ToPtr(schemaForType(t.Elem(), fieldName)),
			},
		}
	case reflect.Struct:
		return schemaForStruct(t)
	case reflect.Interface:
		// An interface can hold anything, so it can't be constrained further.
		return apiextensionsv1.JSONSchemaProps{XPreserveUnknownFields: lo.ToPtr(true)}
	default:
		log.Fatalf("unsupported kind %s for field %q; the upstream kubelet type added a "+
			"construct this generator doesn't handle", t.Kind(), fieldName)
		return apiextensionsv1.JSONSchemaProps{}
	}
}
