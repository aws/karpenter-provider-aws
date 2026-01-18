package spec3

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/kube-openapi/pkg/internal"
	jsontesting "k8s.io/kube-openapi/pkg/util/jsontesting"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/randfill"
)

// cmp.Diff panics when reflecting unexported fields under jsonreference.Ref
// a custom comparator is required
var swaggerDiffOptions = []cmp.Option{cmp.Comparer(func(a spec.Ref, b spec.Ref) bool {
	return a.String() == b.String()
})}

func TestOpenAPIV3RoundTrip(t *testing.T) {
	var fuzzer *randfill.Filler
	fuzzer = randfill.NewWithSeed(1646791953)
	// Make sure we have enough depth such that maps do not yield nil elements
	fuzzer.MaxDepth(22).NilChance(0.5).NumElements(1, 7)
	fuzzer.Funcs(OpenAPIV3FuzzFuncs...)
	expected := &OpenAPI{}
	fuzzer.Fill(expected)

	j, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var actual *OpenAPI
	err = json.Unmarshal(j, &actual)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal(cmp.Diff(expected, actual, swaggerDiffOptions...))
	}
}

func TestOpenAPIV3Deserialize(t *testing.T) {
	swagFile, err := os.Open("./testdata/appsv1spec.json")
	if err != nil {
		t.Fatal(err)
	}
	defer swagFile.Close()
	originalJSON, err := io.ReadAll(swagFile)
	if err != nil {
		t.Fatal(err)
	}
	internal.UseOptimizedJSONUnmarshalingV3 = false

	var result1 *OpenAPI

	if err := json.Unmarshal(originalJSON, &result1); err != nil {
		t.Fatal(err)
	}
	internal.UseOptimizedJSONUnmarshalingV3 = true
	var result2 *OpenAPI
	if err := json.Unmarshal(originalJSON, &result2); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result1, result2) {
		t.Fatal(cmp.Diff(result1, result2, swaggerDiffOptions...))
	}
}

func TestOpenAPIV3Serialize(t *testing.T) {
	swagFile, err := os.Open("./testdata/appsv1spec.json")
	if err != nil {
		t.Fatal(err)
	}
	defer swagFile.Close()
	originalJSON, err := io.ReadAll(swagFile)
	if err != nil {
		t.Fatal(err)
	}
	var openapi *OpenAPI
	if err := json.Unmarshal(originalJSON, &openapi); err != nil {
		t.Fatal(err)
	}

	internal.UseOptimizedJSONUnmarshalingV3 = false
	want, err := json.Marshal(openapi)
	if err != nil {
		t.Fatal(err)
	}
	internal.UseOptimizedJSONUnmarshalingV3 = true
	got, err := openapi.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := jsontesting.JsonCompare(want, got); err != nil {
		t.Errorf("marshal doesn't match: %v", err)
	}
}

func TestOpenAPIV3SerializeFuzzed(t *testing.T) {
	var fuzzer *randfill.Filler
	fuzzer = randfill.NewWithSeed(1646791953)
	fuzzer.MaxDepth(13).NilChance(0.075).NumElements(1, 2)
	fuzzer.Funcs(OpenAPIV3FuzzFuncs...)

	for i := 0; i < 100; i++ {
		openapi := &OpenAPI{}
		fuzzer.Fill(openapi)

		internal.UseOptimizedJSONUnmarshalingV3 = false
		want, err := json.Marshal(openapi)
		if err != nil {
			t.Fatal(err)
		}
		internal.UseOptimizedJSONUnmarshalingV3 = true
		got, err := openapi.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if err := jsontesting.JsonCompare(want, got); err != nil {
			t.Errorf("fuzzed marshal doesn't match: %v", err)
		}
	}
}

func TestOpenAPIV3SerializeStable(t *testing.T) {
	swagFile, err := os.Open("./testdata/appsv1spec.json")
	if err != nil {
		t.Fatal(err)
	}
	defer swagFile.Close()
	originalJSON, err := io.ReadAll(swagFile)
	if err != nil {
		t.Fatal(err)
	}
	var openapi *OpenAPI
	if err := json.Unmarshal(originalJSON, &openapi); err != nil {
		t.Fatal(err)
	}

	internal.UseOptimizedJSONUnmarshalingV3 = true
	for i := 0; i < 5; i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			want, err := openapi.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			got, err := openapi.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if err := jsontesting.JsonCompare(want, got); err != nil {
				t.Errorf("marshal doesn't match: %v", err)
			}
		})
	}
}

func BenchmarkOpenAPIV3Deserialize(b *testing.B) {
	benchcases := []struct {
		file string
	}{
		{
			file: "appsv1spec.json",
		},
		{
			file: "authorizationv1spec.json",
		},
	}
	for _, bc := range benchcases {
		swagFile, err := os.Open("./testdata/" + bc.file)
		if err != nil {
			b.Fatal(err)
		}
		defer swagFile.Close()
		originalJSON, err := io.ReadAll(swagFile)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		b.Run(fmt.Sprintf("%s jsonv1", bc.file), func(b2 *testing.B) {
			b2.ReportAllocs()
			internal.UseOptimizedJSONUnmarshaling = false
			internal.UseOptimizedJSONUnmarshalingV3 = false
			for i := 0; i < b2.N; i++ {
				var result *OpenAPI
				if err := json.Unmarshal(originalJSON, &result); err != nil {
					b2.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("%s jsonv2 via jsonv1 schema only", bc.file), func(b2 *testing.B) {
			b2.ReportAllocs()
			internal.UseOptimizedJSONUnmarshaling = true
			internal.UseOptimizedJSONUnmarshalingV3 = false
			for i := 0; i < b2.N; i++ {
				var result *OpenAPI
				if err := json.Unmarshal(originalJSON, &result); err != nil {
					b2.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("%s jsonv2 via jsonv1 full spec", bc.file), func(b2 *testing.B) {
			b2.ReportAllocs()
			internal.UseOptimizedJSONUnmarshaling = true
			internal.UseOptimizedJSONUnmarshalingV3 = true
			for i := 0; i < b2.N; i++ {
				var result *OpenAPI
				if err := json.Unmarshal(originalJSON, &result); err != nil {
					b2.Fatal(err)
				}
			}
		})

		b.Run("jsonv2", func(b2 *testing.B) {
			b2.ReportAllocs()
			internal.UseOptimizedJSONUnmarshaling = true
			internal.UseOptimizedJSONUnmarshalingV3 = true
			for i := 0; i < b2.N; i++ {
				var result *OpenAPI
				if err := result.UnmarshalJSON(originalJSON); err != nil {
					b2.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOpenAPIV3Serialize(b *testing.B) {
	benchcases := []struct {
		file string
	}{
		{
			file: "appsv1spec.json",
		},
		{
			file: "authorizationv1spec.json",
		},
	}
	for _, bc := range benchcases {
		swagFile, err := os.Open("./testdata/" + bc.file)
		if err != nil {
			b.Fatal(err)
		}
		defer swagFile.Close()
		originalJSON, err := io.ReadAll(swagFile)
		if err != nil {
			b.Fatal(err)
		}
		var openapi *OpenAPI
		if err := json.Unmarshal(originalJSON, &openapi); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		b.Run(fmt.Sprintf("%s jsonv1", bc.file), func(b2 *testing.B) {
			b2.ReportAllocs()
			internal.UseOptimizedJSONMarshalingV3 = false
			for i := 0; i < b2.N; i++ {
				if _, err := json.Marshal(openapi); err != nil {
					b2.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("%s jsonv2 via jsonv1 full spec", bc.file), func(b2 *testing.B) {
			b2.ReportAllocs()
			internal.UseOptimizedJSONMarshalingV3 = true
			for i := 0; i < b2.N; i++ {
				if _, err := json.Marshal(openapi); err != nil {
					b2.Fatal(err)
				}
			}
		})

		b.Run("jsonv2", func(b2 *testing.B) {
			b2.ReportAllocs()
			internal.UseOptimizedJSONMarshalingV3 = true
			for i := 0; i < b2.N; i++ {
				if _, err := openapi.MarshalJSON(); err != nil {
					b2.Fatal(err)
				}
			}
		})
	}
}
