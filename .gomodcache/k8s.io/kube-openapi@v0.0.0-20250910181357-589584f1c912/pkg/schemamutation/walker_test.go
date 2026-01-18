/*
Copyright 2019 The Kubernetes Authors.

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

package schemamutation

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"k8s.io/kube-openapi/pkg/util/jsontesting"
	"k8s.io/kube-openapi/pkg/util/sets"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/randfill"
)

func fuzzFuncs(f *randfill.Filler, refFunc func(ref *spec.Ref, c randfill.Continue, visible bool)) {
	invisible := 0 // == 0 means visible, > 0 means invisible
	depth := 0
	maxDepth := 3
	nilChance := func(depth int) float64 {
		return math.Pow(0.9, math.Max(0.0, float64(maxDepth-depth)))
	}
	updateFuzzer := func(depth int) {
		f.NilChance(nilChance(depth))
		f.NumElements(0, max(0, maxDepth-depth))
	}
	updateFuzzer(depth)
	enter := func(o interface{}, recursive bool, c randfill.Continue) {
		if recursive {
			depth++
			updateFuzzer(depth)
		}

		invisible++
		c.FillNoCustom(o)
		invisible--
	}
	leave := func(recursive bool) {
		if recursive {
			depth--
			updateFuzzer(depth)
		}
	}
	f.Funcs(
		func(ref *spec.Ref, c randfill.Continue) {
			refFunc(ref, c, invisible == 0)
		},
		func(sa *spec.SchemaOrStringArray, c randfill.Continue) {
			*sa = spec.SchemaOrStringArray{}
			if c.Bool() {
				c.Fill(&sa.Schema)
			} else {
				c.Fill(&sa.Property)
			}
			if sa.Schema == nil && len(sa.Property) == 0 {
				*sa = spec.SchemaOrStringArray{Schema: &spec.Schema{}}
			}
		},
		func(url *spec.SchemaURL, c randfill.Continue) {
			*url = spec.SchemaURL("http://url")
		},
		func(s *spec.Swagger, c randfill.Continue) {
			enter(s, false, c)
			defer leave(false)

			// only fuzz those fields we walk into with invisible==false
			c.Fill(&s.Parameters)
			c.Fill(&s.Responses)
			c.Fill(&s.Definitions)
			c.Fill(&s.Paths)
		},
		func(p *spec.PathItem, c randfill.Continue) {
			enter(p, false, c)
			defer leave(false)

			// only fuzz those fields we walk into with invisible==false
			c.Fill(&p.Parameters)
			c.Fill(&p.Delete)
			c.Fill(&p.Get)
			c.Fill(&p.Head)
			c.Fill(&p.Options)
			c.Fill(&p.Patch)
			c.Fill(&p.Post)
			c.Fill(&p.Put)
		},
		func(p *spec.Parameter, c randfill.Continue) {
			enter(p, false, c)
			defer leave(false)

			// only fuzz those fields we walk into with invisible==false
			c.Fill(&p.Ref)
			c.Fill(&p.Schema)
			if c.Bool() {
				p.Items = &spec.Items{}
				c.Fill(&p.Items.Ref)
			} else {
				p.Items = nil
			}
		},
		func(s *spec.Response, c randfill.Continue) {
			enter(s, false, c)
			defer leave(false)

			// only fuzz those fields we walk into with invisible==false
			c.Fill(&s.Ref)
			c.Fill(&s.Description)
			c.Fill(&s.Schema)
			c.Fill(&s.Examples)
		},
		func(s *spec.Dependencies, c randfill.Continue) {
			enter(s, false, c)
			defer leave(false)

			// and nothing with invisible==false
		},
		func(p *spec.SimpleSchema, c randfill.Continue) {
			// randfill is broken and calls this even for *SimpleSchema fields, ignoring NilChance, leading to infinite recursion
			if c.Float64() > nilChance(depth) {
				return
			}

			enter(p, true, c)
			defer leave(true)

			c.FillNoCustom(p)
		},
		func(s *spec.SchemaProps, c randfill.Continue) {
			// randfill is broken and calls this even for *SchemaProps fields, ignoring NilChance, leading to infinite recursion
			if c.Float64() > nilChance(depth) {
				return
			}

			enter(s, true, c)
			defer leave(true)

			c.FillNoCustom(s)
		},
		func(i *interface{}, c randfill.Continue) {
			// do nothing for examples and defaults. These are free form JSON fields.
		},
	)
}

func TestReplaceReferences(t *testing.T) {
	visibleRE, err := regexp.Compile("\"\\$ref\":\"(http://ref-[^\"]*)\"")
	if err != nil {
		t.Fatalf("failed to compile ref regex: %v", err)
	}
	invisibleRE, err := regexp.Compile("\"\\$ref\":\"(http://invisible-[^\"]*)\"")
	if err != nil {
		t.Fatalf("failed to compile ref regex: %v", err)
	}

	for i := 0; i < 1000; i++ {
		var visibleRefs, invisibleRefs sets.String
		var seed int64
		var randSource rand.Source
		var s *spec.Swagger
		for {
			visibleRefs = sets.NewString()
			invisibleRefs = sets.NewString()

			f := randfill.New()
			seed = time.Now().UnixNano()
			//seed = int64(1549012506261785182)
			randSource = rand.New(rand.NewSource(seed))
			f.RandSource(randSource)

			visibleRefsNum := 0
			invisibleRefsNum := 0
			fuzzFuncs(f,
				func(ref *spec.Ref, c randfill.Continue, visible bool) {
					var url string
					if visible {
						// this is a ref that is seen by the walker (we have some exceptions where we don't walk into)
						url = fmt.Sprintf("http://ref-%d", visibleRefsNum)
						visibleRefsNum++
					} else {
						// this is a ref that is not seen by the walker (we have some exceptions where we don't walk into)
						url = fmt.Sprintf("http://invisible-%d", invisibleRefsNum)
						invisibleRefsNum++
					}

					r, err := spec.NewRef(url)
					if err != nil {
						t.Fatalf("failed to fuzz ref: %v", err)
					}
					*ref = r
				},
			)

			// create random swagger spec with random URL references, but at least one ref
			s = &spec.Swagger{}
			f.Fill(s)

			// clone spec to normalize (fuzz might generate objects which do not roundtrip json marshalling
			var err error
			s, err = cloneSwagger(s)
			if err != nil {
				t.Fatalf("failed to normalize swagger after fuzzing: %v", err)
			}

			// find refs
			bs, err := s.MarshalJSON()
			if err != nil {
				t.Fatalf("failed to marshal swagger: %v", err)
			}
			for _, m := range invisibleRE.FindAllStringSubmatch(string(bs), -1) {
				invisibleRefs.Insert(m[1])
			}
			if res := visibleRE.FindAllStringSubmatch(string(bs), -1); len(res) > 0 {
				for _, m := range res {
					visibleRefs.Insert(m[1])
				}
				break
			}
		}

		t.Run(fmt.Sprintf("iteration %d", i), func(t *testing.T) {
			mutatedRefs := sets.NewString()
			mutationProbability := rand.New(randSource).Float64()
			for _, vr := range visibleRefs.List() {
				if rand.New(randSource).Float64() > mutationProbability {
					mutatedRefs.Insert(vr)
				}
			}

			origString, err := s.MarshalJSON()
			if err != nil {
				t.Fatalf("failed to marshal swagger: %v", err)
			}
			t.Logf("created schema with %d walked refs, %d invisible refs, mutating %v, seed %d: %s", visibleRefs.Len(), invisibleRefs.Len(), mutatedRefs.List(), seed, string(origString))

			// convert to json string, replace one of the refs, and unmarshal back
			mutatedString := string(origString)
			for _, r := range mutatedRefs.List() {
				mr := strings.Replace(r, "ref", "mutated", -1)
				mutatedString = strings.Replace(mutatedString, "\""+r+"\"", "\""+mr+"\"", -1)
			}
			mutatedViaJSON := &spec.Swagger{}
			if err := json.Unmarshal([]byte(mutatedString), mutatedViaJSON); err != nil {
				t.Fatalf("failed to unmarshal mutated spec: %v", err)
			}

			// replay the same mutation using the mutating walker
			seenRefs := sets.NewString()
			walker := Walker{
				RefCallback: func(ref *spec.Ref) *spec.Ref {
					seenRefs.Insert(ref.String())
					if mutatedRefs.Has(ref.String()) {
						r, err := spec.NewRef(strings.Replace(ref.String(), "ref", "mutated", -1))
						if err != nil {
							t.Fatalf("failed to create ref: %v", err)
						}
						return &r
					}
					return ref
				},
				SchemaCallback: SchemaCallBackNoop,
			}
			mutatedViaWalker := walker.WalkRoot(s)

			// compare that we got the same
			if !reflect.DeepEqual(mutatedViaJSON, mutatedViaWalker) {
				t.Errorf("mutation via walker differ from JSON text replacement (got A, expected B): %s", objectDiff(mutatedViaWalker, mutatedViaJSON))
			}
			if !seenRefs.HasAll(visibleRefs.List()...) {
				t.Errorf("expected to see the same refs in the walker as during fuzzing. Not seen: %v", visibleRefs.Difference(seenRefs).List())
			}
			if shouldNotSee := seenRefs.Intersection(invisibleRefs); shouldNotSee.Len() > 0 {
				t.Errorf("refs seen that the walker is not expected to see: %v", shouldNotSee.List())
			}
		})
	}
}

func TestReplaceSchema(t *testing.T) {
	for i := 0; i < 1000; i++ {
		t.Run(fmt.Sprintf("iteration-%d", i), func(t *testing.T) {
			seed := time.Now().UnixNano()
			f := randfill.NewWithSeed(seed).NilChance(0).MaxDepth(5)
			rootSchema := &spec.Schema{}
			f.Funcs(func(s *spec.Schema, c randfill.Continue) {
				c.Fill(&s.Description)
				s.Description += " original"
				if c.Bool() {
					// append enums
					var enums []string
					c.Fill(&enums)
					for _, enum := range enums {
						s.Enum = append(s.Enum, enum)
					}
				}
				if c.Bool() {
					c.Fill(&s.Properties)
				}
				if c.Bool() {
					c.Fill(&s.AdditionalProperties)
				}
				if c.Bool() {
					c.Fill(&s.PatternProperties)
				}
				if c.Bool() {
					c.Fill(&s.AdditionalItems)
				}
				if c.Bool() {
					c.Fill(&s.AnyOf)
				}
				if c.Bool() {
					c.Fill(&s.AllOf)
				}
				if c.Bool() {
					c.Fill(&s.OneOf)
				}
				if c.Bool() {
					c.Fill(&s.Not)
				}
				if c.Bool() {
					c.Fill(&s.Definitions)
				}
				if c.Bool() {
					items := new(spec.SchemaOrArray)
					if c.Bool() {
						c.Fill(&items.Schema)
					} else {
						c.Fill(&items.Schemas)
					}
					s.Items = items
				}
			})
			f.Fill(rootSchema)
			w := &Walker{SchemaCallback: func(schema *spec.Schema) *spec.Schema {
				s := *schema
				s.Description = strings.Replace(s.Description, "original", "modified", -1)
				return &s
			}, RefCallback: RefCallbackNoop}
			newSchema := w.WalkSchema(rootSchema)
			origBytes, err := json.Marshal(rootSchema)
			if err != nil {
				t.Fatalf("cannot marshal original schema: %v", err)
			}
			origJSON := string(origBytes)
			mutatedWithString := strings.Replace(origJSON, "original", "modified", -1)
			newBytes, err := json.Marshal(newSchema)
			if err != nil {
				t.Fatalf("cannot marshal mutated schema: %v", err)
			}
			if err := jsontesting.JsonCompare(newBytes, []byte(mutatedWithString)); err != nil {
				t.Error(err)
			}
			if !strings.Contains(origJSON, `"enum":[`) {
				t.Logf("did not contain enum, skipping enum checks")
				return
			}
			// test enum removal
			w = &Walker{SchemaCallback: func(schema *spec.Schema) *spec.Schema {
				s := *schema
				s.Enum = nil
				return &s
			}, RefCallback: RefCallbackNoop}
			newSchema = w.WalkSchema(rootSchema)
			newBytes, err = json.Marshal(newSchema)
			if err != nil {
				t.Fatalf("cannot marshal mutated schema: %v", err)
			}
			if strings.Contains(string(newBytes), `"enum":[`) {
				t.Errorf("enum still exists in %q", newBytes)
			}
		})
	}
}

func cloneSwagger(orig *spec.Swagger) (*spec.Swagger, error) {
	bs, err := orig.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("error marshaling: %v", err)
	}
	s := &spec.Swagger{}
	if err := json.Unmarshal(bs, s); err != nil {
		return nil, fmt.Errorf("error unmarshaling: %v", err)
	}
	return s, nil
}

// stringDiff diffs a and b and returns a human readable diff.
func stringDiff(a, b string) string {
	ba := []byte(a)
	bb := []byte(b)
	out := []byte{}
	i := 0
	for ; i < len(ba) && i < len(bb); i++ {
		if ba[i] != bb[i] {
			break
		}
		out = append(out, ba[i])
	}
	out = append(out, []byte("\n\nA: ")...)
	out = append(out, ba[i:]...)
	out = append(out, []byte("\n\nB: ")...)
	out = append(out, bb[i:]...)
	out = append(out, []byte("\n\n")...)
	return string(out)
}

// objectDiff writes the two objects out as JSON and prints out the identical part of
// the objects followed by the remaining part of 'a' and finally the remaining part of 'b'.
// For debugging tests.
func objectDiff(a, b interface{}) string {
	ab, err := json.Marshal(a)
	if err != nil {
		panic(fmt.Sprintf("a: %v", err))
	}
	bb, err := json.Marshal(b)
	if err != nil {
		panic(fmt.Sprintf("b: %v", err))
	}
	return stringDiff(string(ab), string(bb))
}

func max(i, j int) int {
	if i > j {
		return i
	}
	return j
}
