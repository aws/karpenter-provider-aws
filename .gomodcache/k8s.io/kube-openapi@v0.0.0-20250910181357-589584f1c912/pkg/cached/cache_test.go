/*
Copyright 2023 The Kubernetes Authors.

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

package cached_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/kube-openapi/pkg/cached"
)

func TestDataFunc(t *testing.T) {
	count := 0
	source := cached.Func(func() ([]byte, string, error) {
		count += 1
		return []byte("source"), "source", nil
	})
	if _, _, err := source.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := source.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected function called twice, called: %v", count)
	}
}

func TestDataFuncError(t *testing.T) {
	count := 0
	source := cached.Func(func() ([]byte, string, error) {
		count += 1
		return nil, "", errors.New("source error")
	})
	if _, _, err := source.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}
	if _, _, err := source.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}
	if count != 2 {
		t.Fatalf("Expected function called twice, called: %v", count)
	}
}

func TestDataFuncAlternate(t *testing.T) {
	count := 0
	source := cached.Func(func() ([]byte, string, error) {
		count += 1
		if count%2 == 0 {
			return nil, "", errors.New("source error")
		}
		return []byte("source"), "source", nil
	})
	if _, _, err := source.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := source.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}
	if _, _, err := source.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := source.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}
	if count != 4 {
		t.Fatalf("Expected function called 4x, called: %v", count)
	}
}

func TestOnce(t *testing.T) {
	count := 0
	source := cached.Once(cached.Func(func() ([]byte, string, error) {
		count += 1
		return []byte("source"), "source", nil
	}))
	if _, _, err := source.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := source.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected function called once, called: %v", count)
	}
}

func TestOnceError(t *testing.T) {
	count := 0
	source := cached.Once(cached.Func(func() ([]byte, string, error) {
		count += 1
		return nil, "", errors.New("source error")
	}))
	if _, _, err := source.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}
	if _, _, err := source.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}
	if count != 1 {
		t.Fatalf("Expected function called once, called: %v", count)
	}
}

func TestResultGet(t *testing.T) {
	source := cached.Static([]byte("source"), "etag")
	value, etag, err := source.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(value) != want {
		t.Fatalf("expected value %q, got %q", want, string(value))
	}
	if want := "etag"; etag != want {
		t.Fatalf("expected etag %q, got %q", want, etag)
	}
}

func TestResultGetError(t *testing.T) {
	source := cached.Result[[]byte]{Err: errors.New("source error")}
	value, etag, err := source.Get()
	if err == nil {
		t.Fatalf("expected error, found none")
	}
	if value != nil {
		t.Fatalf("expected nil value, got %v", value)
	}
	if etag != "" {
		t.Fatalf("expected empty etag, got %q", etag)
	}
}

func TestTransform(t *testing.T) {
	sourceCount := 0
	source := cached.Func(func() ([]byte, string, error) {
		sourceCount += 1
		return []byte("source"), "source", nil
	})
	transformerCount := 0
	transformer := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		transformerCount += 1
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed " + string(value)), "transformed " + etag, nil
	}, source)
	if _, _, err := transformer.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := transformer.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sourceCount != 2 {
		t.Fatalf("Expected source function called twice, called: %v", sourceCount)
	}
	if transformerCount != 1 {
		t.Fatalf("Expected transformer function called once, called: %v", transformerCount)
	}
}

func TestTransformChained(t *testing.T) {
	sourceCount := 0
	source := cached.Func(func() ([]byte, string, error) {
		sourceCount += 1
		return []byte("source"), "source", nil
	})
	transformer1Count := 0
	transformer1 := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		transformer1Count += 1
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed " + string(value)), etag, nil
	}, source)
	transformer2Count := 0
	transformer2 := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		transformer2Count += 1
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed " + string(value)), etag, nil
	}, transformer1)
	transformer3Count := 0
	transformer3 := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		transformer3Count += 1
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed " + string(value)), etag, nil
	}, transformer2)
	if _, _, err := transformer3.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, etag, err := transformer3.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "transformed transformed transformed source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if sourceCount != 2 {
		t.Fatalf("Expected source function called twice, called: %v", sourceCount)
	}
	if transformer1Count != 1 {
		t.Fatalf("Expected transformer function called once, called: %v", transformer1Count)
	}
	if transformer2Count != 1 {
		t.Fatalf("Expected transformer function called once, called: %v", transformer2Count)
	}
	if transformer3Count != 1 {
		t.Fatalf("Expected transformer function called once, called: %v", transformer3Count)
	}
}

func TestTransformError(t *testing.T) {
	sourceCount := 0
	source := cached.Func(func() ([]byte, string, error) {
		sourceCount += 1
		return []byte("source"), "source", nil
	})
	transformerCount := 0
	transformer := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		transformerCount += 1
		return nil, "", errors.New("transformer error")
	}, source)
	if _, _, err := transformer.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if _, _, err := transformer.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if sourceCount != 2 {
		t.Fatalf("Expected source function called twice, called: %v", sourceCount)
	}
	if transformerCount != 2 {
		t.Fatalf("Expected transformer function called twice, called: %v", transformerCount)
	}
}

func TestTransformSourceError(t *testing.T) {
	sourceCount := 0
	source := cached.Func(func() ([]byte, string, error) {
		sourceCount += 1
		return nil, "", errors.New("source error")
	})
	transformerCount := 0
	transformer := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		transformerCount += 1
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed " + string(value)), "transformed " + etag, nil
	}, source)
	if _, _, err := transformer.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if _, _, err := transformer.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if sourceCount != 2 {
		t.Fatalf("Expected source function called twice, called: %v", sourceCount)
	}
	if transformerCount != 2 {
		t.Fatalf("Expected transformer function called twice, called: %v", transformerCount)
	}
}

func TestTransformAlternateSourceError(t *testing.T) {
	sourceCount := 0
	source := cached.Func(func() ([]byte, string, error) {
		sourceCount += 1
		if sourceCount%2 == 0 {
			return nil, "", errors.New("source error")
		}
		return []byte("source"), "source", nil
	})
	transformerCount := 0
	transformer := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		transformerCount += 1
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed " + string(value)), "transformed " + etag, err
	}, source)
	result, etag, err := transformer.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "transformed source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "transformed source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if _, _, err := transformer.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	result, etag, err = transformer.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "transformed source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "transformed source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if _, _, err := transformer.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if sourceCount != 4 {
		t.Fatalf("Expected source function called 4x, called: %v", sourceCount)
	}
	if transformerCount != 4 {
		t.Fatalf("Expected transformer function called 4x, called: %v", transformerCount)
	}

}

func TestMerge(t *testing.T) {
	source1Count := 0
	source1 := cached.Func(func() ([]byte, string, error) {
		source1Count += 1
		return []byte("source1"), "source1", nil
	})
	source2Count := 0
	source2 := cached.Func(func() ([]byte, string, error) {
		source2Count += 1
		return []byte("source2"), "source2", nil
	})
	mergerCount := 0
	merger := cached.Merge(func(results map[string]cached.Result[[]byte]) ([]byte, string, error) {
		mergerCount += 1
		d := []string{}
		e := []string{}
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			d = append(d, string(result.Value))
			e = append(e, result.Etag)
		}
		sort.Strings(d)
		sort.Strings(e)
		return []byte("merged " + strings.Join(d, " and ")), "merged " + strings.Join(e, " and "), nil
	}, map[string]cached.Value[[]byte]{
		"source1": source1,
		"source2": source2,
	})
	if _, _, err := merger.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, etag, err := merger.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "merged source1 and source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "merged source1 and source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}

	if source1Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source1Count)
	}
	if source2Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source2Count)
	}
	if mergerCount != 1 {
		t.Fatalf("Expected merger function called once, called: %v", mergerCount)
	}
}

func TestMergeError(t *testing.T) {
	source1Count := 0
	source1 := cached.Func(func() ([]byte, string, error) {
		source1Count += 1
		return []byte("source1"), "source1", nil
	})
	source2Count := 0
	source2 := cached.Func(func() ([]byte, string, error) {
		source2Count += 1
		return []byte("source2"), "source2", nil
	})
	mergerCount := 0
	merger := cached.Merge(func(results map[string]cached.Result[[]byte]) ([]byte, string, error) {
		mergerCount += 1
		return nil, "", errors.New("merger error")
	}, map[string]cached.Value[[]byte]{
		"source1": source1,
		"source2": source2,
	})
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if source1Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source1Count)
	}
	if source2Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source2Count)
	}
	if mergerCount != 2 {
		t.Fatalf("Expected merger function called twice, called: %v", mergerCount)
	}
}

func TestMergeSourceError(t *testing.T) {
	source1Count := 0
	source1 := cached.Func(func() ([]byte, string, error) {
		source1Count += 1
		return nil, "", errors.New("source1 error")
	})
	source2Count := 0
	source2 := cached.Func(func() ([]byte, string, error) {
		source2Count += 1
		return []byte("source2"), "source2", nil
	})
	mergerCount := 0
	merger := cached.Merge(func(results map[string]cached.Result[[]byte]) ([]byte, string, error) {
		mergerCount += 1
		d := []string{}
		e := []string{}
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			d = append(d, string(result.Value))
			e = append(e, result.Etag)
		}
		sort.Strings(d)
		sort.Strings(e)
		return []byte("merged " + strings.Join(d, " and ")), "merged " + strings.Join(e, " and "), nil
	}, map[string]cached.Value[[]byte]{
		"source1": source1,
		"source2": source2,
	})
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if source1Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source1Count)
	}
	if source2Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source2Count)
	}
	if mergerCount != 2 {
		t.Fatalf("Expected merger function called twice, called: %v", mergerCount)
	}
}

func TestMergeAlternateSourceError(t *testing.T) {
	source1Count := 0
	source1 := cached.Func(func() ([]byte, string, error) {
		source1Count += 1
		if source1Count%2 == 0 {
			return nil, "", errors.New("source1 error")
		} else {
			return []byte("source1"), "source1", nil
		}
	})
	source2Count := 0
	source2 := cached.Func(func() ([]byte, string, error) {
		source2Count += 1
		return []byte("source2"), "source2", nil
	})
	mergerCount := 0
	merger := cached.Merge(func(results map[string]cached.Result[[]byte]) ([]byte, string, error) {
		mergerCount += 1
		d := []string{}
		e := []string{}
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			d = append(d, string(result.Value))
			e = append(e, result.Etag)
		}
		sort.Strings(d)
		sort.Strings(e)
		return []byte("merged " + strings.Join(d, " and ")), "merged " + strings.Join(e, " and "), nil
	}, map[string]cached.Value[[]byte]{
		"source1": source1,
		"source2": source2,
	})
	result, etag, err := merger.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "merged source1 and source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "merged source1 and source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	result, etag, err = merger.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "merged source1 and source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "merged source1 and source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if source1Count != 4 {
		t.Fatalf("Expected source function called 4x, called: %v", source1Count)
	}
	if source2Count != 4 {
		t.Fatalf("Expected source function called 4x, called: %v", source2Count)
	}
	if mergerCount != 4 {
		t.Fatalf("Expected merger function called 4x, called: %v", mergerCount)
	}
}

func TestAtomic(t *testing.T) {
	sourceDataCount := 0
	sourceData := cached.Func(func() ([]byte, string, error) {
		sourceDataCount += 1
		return []byte("source"), "source", nil
	})
	sourceData2Count := 0
	sourceData2 := cached.Func(func() ([]byte, string, error) {
		sourceData2Count += 1
		return []byte("source2"), "source2", nil
	})
	sourceErrCount := 0
	sourceErr := cached.Func(func() ([]byte, string, error) {
		sourceErrCount += 1
		return nil, "", errors.New("source error")
	})

	replaceable := &cached.Atomic[[]byte]{}
	replaceable.Store(sourceErr)
	if _, _, err := replaceable.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}

	replaceable.Store(sourceData)
	result, etag, err := replaceable.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}

	// replace with the same thing, shouldn't change anything
	replaceable.Store(sourceData)
	result, etag, err = replaceable.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}

	// when replacing with an error source, we see the error again
	replaceable.Store(sourceErr)
	result, etag, err = replaceable.Get()
	if err == nil {
		t.Fatalf("unexpected success")
	}

	replaceable.Store(sourceData2)
	result, etag, err = replaceable.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if sourceDataCount != 2 {
		t.Fatalf("Expected sourceData function called twice, called: %v", sourceDataCount)
	}
	if sourceData2Count != 1 {
		t.Fatalf("Expected sourceData2 function called once, called: %v", sourceData2Count)
	}
	if sourceErrCount != 2 {
		t.Fatalf("Expected error source function called once, called: %v", sourceErrCount)
	}
}

func TestLastSuccess(t *testing.T) {
	sourceDataCount := 0
	sourceData := cached.Func(func() ([]byte, string, error) {
		sourceDataCount += 1
		return []byte("source"), "source", nil
	})
	sourceData2Count := 0
	sourceData2 := cached.Func(func() ([]byte, string, error) {
		sourceData2Count += 1
		return []byte("source2"), "source2", nil
	})

	sourceErrCount := 0
	sourceErr := cached.Func(func() ([]byte, string, error) {
		sourceErrCount += 1
		return nil, "", errors.New("source error")
	})
	lastSuccess := &cached.LastSuccess[[]byte]{}
	lastSuccess.Store(sourceErr)
	if _, _, err := lastSuccess.Get(); err == nil {
		t.Fatalf("expected error, found none")
	}
	lastSuccess.Store(sourceData)
	result, etag, err := lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	// replace with the same thing, shouldn't change anything
	lastSuccess.Store(sourceData)
	result, etag, err = lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	// Even if we replace with something that fails, we continue to return the success.
	lastSuccess.Store(sourceErr)
	result, etag, err = lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	lastSuccess.Store(sourceData2)
	result, etag, err = lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if sourceDataCount != 2 {
		t.Fatalf("Expected sourceData function called twice, called: %v", sourceDataCount)
	}
	if sourceData2Count != 1 {
		t.Fatalf("Expected sourceData2 function called once, called: %v", sourceData2Count)
	}
	if sourceErrCount != 2 {
		t.Fatalf("Expected error source function called once, called: %v", sourceErrCount)
	}
}

func TestLastSuccessEtag(t *testing.T) {
	lastSuccess := &cached.LastSuccess[bool]{}
	lastSuccess.Store(cached.Func(func() (bool, string, error) {
		return false, "hash", nil
	}))
	lastSuccess.Store(cached.Static(true, "hash2"))
	result, etag, _ := lastSuccess.Get()
	if actual := etag; actual != "hash2" {
		t.Fatalf(`expected "hash2", got %q`, actual)
	}
	if result != true {
		t.Fatal(`expected "true", got "false"`)
	}
}

func TestLastSuccessAlternateError(t *testing.T) {
	sourceCount := 0
	source := cached.Func(func() ([]byte, string, error) {
		sourceCount += 1
		if sourceCount%2 == 0 {
			return nil, "", errors.New("source error")
		} else {
			return []byte("source"), "source", nil
		}
	})
	lastSuccess := &cached.LastSuccess[[]byte]{}
	lastSuccess.Store(source)
	result, etag, err := lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	result, etag, err = lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	result, etag, err = lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	result, etag, err = lastSuccess.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if sourceCount != 4 {
		t.Fatalf("Expected sourceData function called 4x, called: %v", sourceCount)
	}
}

func TestLastSuccessWithTransformer(t *testing.T) {
	lastSuccess := &cached.LastSuccess[[]byte]{}
	lastSuccess.Store(cached.Static([]byte("source"), "source"))
	transformerCount := 0
	transformed := cached.Transform[[]byte](func(value []byte, etag string, err error) ([]byte, string, error) {
		transformerCount += 1
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed " + string(value)), "transformed " + etag, nil
	}, lastSuccess)
	result, etag, err := transformed.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, etag, err = transformed.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "transformed source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "transformed source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	// replace with new cache, transformer shouldn't be affected (or called)
	lastSuccess.Store(cached.Static([]byte("source"), "source"))
	result, etag, err = transformed.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, etag, err = transformed.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "transformed source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "transformed source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	// replace with failing cache, transformer should still not be affected (or called)
	lastSuccess.Store(cached.Result[[]byte]{Err: errors.New("source error")})
	result, etag, err = transformed.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, etag, err = transformed.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "transformed source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "transformed source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}

	if transformerCount != 1 {
		t.Fatalf("Expected transformer function called once, called: %v", transformerCount)
	}
}

// Here is an example of how one can write a cache that will constantly
// be pulled, while actually recomputing the results only as needed.
func Example() {
	// Merge Json is a replaceable cache, since we'll want it to
	// change a few times.
	mergeJson := &cached.LastSuccess[[]byte]{}

	one := cached.Once(cached.Func(func() ([]byte, string, error) {
		// This one is computed lazily, only when requested, and only once.
		return []byte("one"), "one", nil
	}))
	two := cached.Func(func() ([]byte, string, error) {
		// This cache is re-computed every time.
		return []byte("two"), "two", nil
	})
	// This cache is computed once, and is not lazy at all.
	three := cached.Static([]byte("three"), "three")

	// This cache will allow us to replace a branch of the tree
	// efficiently.

	lastSuccess := &cached.LastSuccess[[]byte]{}
	lastSuccess.Store(cached.Static([]byte("four"), "four"))

	merger := func(results map[string]cached.Result[[]byte]) ([]byte, string, error) {
		var out = []json.RawMessage{}
		var resultEtag string
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			resultEtag += result.Etag
			out = append(out, result.Value)
		}
		data, err := json.Marshal(out)
		if err != nil {
			return nil, "", err
		}
		return data, resultEtag, nil
	}

	mergeJson.Store(cached.Merge(merger, map[string]cached.Value[[]byte]{
		"one":         one,
		"two":         two,
		"three":       three,
		"replaceable": lastSuccess,
	}))

	// Create a new cache that indents a buffer. This should only be
	// called if the buffer has changed.
	indented := cached.Transform[[]byte](func(js []byte, etag string, err error) ([]byte, string, error) {
		// Get the json from the previous layer of cache, before
		// we indent.
		if err != nil {
			return nil, "", err
		}
		var out bytes.Buffer
		json.Indent(&out, js, "", "\t")
		return out.Bytes(), etag, nil
	}, mergeJson)

	// We have "clients" that constantly pulls the indented format.
	go func() {
		for {
			if _, _, err := indented.Get(); err != nil {
				panic(fmt.Errorf("invalid error: %v", err))
			}
		}
	}()

	failure := cached.Result[[]byte]{Err: errors.New("Invalid cache!")}
	// Insert a new sub-cache that fails, it should just be ignored.
	mergeJson.Store(cached.Merge(merger, map[string]cached.Value[[]byte]{
		"one":         one,
		"two":         two,
		"three":       three,
		"replaceable": lastSuccess,
		"failure":     failure,
	}))

	// We can replace just a branch of the dependency tree.
	lastSuccess.Store(cached.Static([]byte("five"), "five"))

	// We can replace to remove the failure and one of the sub-cached.
	mergeJson.Store(cached.Merge(merger, map[string]cached.Value[[]byte]{
		"one":         one,
		"two":         two,
		"replaceable": lastSuccess,
	}))
}

func TestListMerger(t *testing.T) {
	source1Count := 0
	source1 := cached.Func(func() ([]byte, string, error) {
		source1Count += 1
		return []byte("source1"), "source1", nil
	})
	source2Count := 0
	source2 := cached.Func(func() ([]byte, string, error) {
		source2Count += 1
		return []byte("source2"), "source2", nil
	})
	mergerCount := 0
	merger := cached.MergeList(func(results []cached.Result[[]byte]) ([]byte, string, error) {
		mergerCount += 1
		d := []string{}
		e := []string{}
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			d = append(d, string(result.Value))
			e = append(e, result.Etag)
		}
		return []byte("merged " + strings.Join(d, " and ")), "merged " + strings.Join(e, " and "), nil
	}, []cached.Value[[]byte]{
		source1, source2,
	})
	if _, _, err := merger.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, etag, err := merger.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "merged source1 and source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "merged source1 and source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}

	if source1Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source1Count)
	}
	if source2Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source2Count)
	}
	if mergerCount != 1 {
		t.Fatalf("Expected merger function called once, called: %v", mergerCount)
	}
}

func TestMergeListSourceError(t *testing.T) {
	source1Count := 0
	source1 := cached.Func(func() ([]byte, string, error) {
		source1Count += 1
		return nil, "", errors.New("source1 error")
	})
	source2Count := 0
	source2 := cached.Func(func() ([]byte, string, error) {
		source2Count += 1
		return []byte("source2"), "source2", nil
	})
	mergerCount := 0
	merger := cached.MergeList(func(results []cached.Result[[]byte]) ([]byte, string, error) {
		mergerCount += 1
		d := []string{}
		e := []string{}
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			d = append(d, string(result.Value))
			e = append(e, result.Etag)
		}
		return []byte("merged " + strings.Join(d, " and ")), "merged " + strings.Join(e, " and "), nil
	}, []cached.Value[[]byte]{
		source1, source2,
	})
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if source1Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source1Count)
	}
	if source2Count != 2 {
		t.Fatalf("Expected source function called twice, called: %v", source2Count)
	}
	if mergerCount != 2 {
		t.Fatalf("Expected merger function called twice, called: %v", mergerCount)
	}
}

func TestMergeListAlternateSourceError(t *testing.T) {
	source1Count := 0
	source1 := cached.Func(func() ([]byte, string, error) {
		source1Count += 1
		if source1Count%2 == 0 {
			return nil, "", errors.New("source1 error")
		} else {
			return []byte("source1"), "source1", nil
		}
	})
	source2Count := 0
	source2 := cached.Func(func() ([]byte, string, error) {
		source2Count += 1
		return []byte("source2"), "source2", nil
	})
	mergerCount := 0
	merger := cached.MergeList(func(results []cached.Result[[]byte]) ([]byte, string, error) {
		mergerCount += 1
		d := []string{}
		e := []string{}
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			d = append(d, string(result.Value))
			e = append(e, result.Etag)
		}
		return []byte("merged " + strings.Join(d, " and ")), "merged " + strings.Join(e, " and "), nil
	}, []cached.Value[[]byte]{
		source1, source2,
	})
	result, etag, err := merger.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "merged source1 and source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "merged source1 and source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	result, etag, err = merger.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "merged source1 and source2"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "merged source1 and source2"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
	if _, _, err := merger.Get(); err == nil {
		t.Fatalf("expected error, none found")
	}
	if source1Count != 4 {
		t.Fatalf("Expected source function called 4x, called: %v", source1Count)
	}
	if source2Count != 4 {
		t.Fatalf("Expected source function called 4x, called: %v", source2Count)
	}
	if mergerCount != 4 {
		t.Fatalf("Expected merger function called 4x, called: %v", mergerCount)
	}
}

func TestListDAG(t *testing.T) {
	source := cached.Func(func() ([]byte, string, error) {
		return []byte("source"), "source", nil
	})
	transformer1 := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed1 " + string(value)), "transformed1 " + etag, nil
	}, source)
	transformer2 := cached.Transform(func(value []byte, etag string, err error) ([]byte, string, error) {
		if err != nil {
			return nil, "", err
		}
		return []byte("transformed2 " + string(value)), "transformed2 " + etag, nil
	}, source)
	merger := cached.MergeList(func(results []cached.Result[[]byte]) ([]byte, string, error) {
		d := []string{}
		e := []string{}
		for _, result := range results {
			if result.Err != nil {
				return nil, "", result.Err
			}
			d = append(d, string(result.Value))
			e = append(e, result.Etag)
		}
		return []byte("merged " + strings.Join(d, " and ")), "merged " + strings.Join(e, " and "), nil
	}, []cached.Value[[]byte]{
		transformer1, transformer2,
	})
	result, etag, err := merger.Get()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if want := "merged transformed1 source and transformed2 source"; string(result) != want {
		t.Fatalf("expected data = %v, got %v", want, string(result))
	}
	if want := "merged transformed1 source and transformed2 source"; etag != want {
		t.Fatalf("expected etag = %v, got %v", want, etag)
	}
}

func randomString(length uint) string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return string(bytes)

}

func NewRandomSource() cached.Value[int64] {
	return cached.Once(cached.Func(func() (int64, string, error) {
		bytes := make([]byte, 6)
		rand.Read(bytes)
		return rand.Int63(), randomString(10), nil
	}))
}

func repeatedGet(data cached.Value[int64], end time.Time, wg *sync.WaitGroup) {
	for time.Now().Before(end) {
		_, _, _ = data.Get()
	}
	wg.Done()
}

func TestThreadSafe(t *testing.T) {
	end := time.Now().Add(time.Second)
	wg := sync.WaitGroup{}
	static := NewRandomSource()
	wg.Add(1)
	go repeatedGet(static, end, &wg)
	result := cached.Static(rand.Int63(), randomString(10))
	wg.Add(1)
	go repeatedGet(result, end, &wg)
	replaceable := &cached.LastSuccess[int64]{}
	replaceable.Store(NewRandomSource())
	wg.Add(1)
	go repeatedGet(replaceable, end, &wg)
	wg.Add(1)
	go func(r cached.Replaceable[int64], end time.Time, wg *sync.WaitGroup) {
		for time.Now().Before(end) {
			r.Store(NewRandomSource())
		}
		wg.Done()
	}(replaceable, end, &wg)
	merger := cached.Merge(func(results map[string]cached.Result[int64]) (int64, string, error) {
		sum := int64(0)
		for _, result := range results {
			sum += result.Value
		}
		return sum, randomString(10), nil
	}, map[string]cached.Value[int64]{
		"one": NewRandomSource(),
		"two": NewRandomSource(),
	})
	wg.Add(1)
	go repeatedGet(merger, end, &wg)
	transformer := cached.Transform(func(value int64, etag string, err error) (int64, string, error) {
		return value + 5, randomString(10), nil
	}, NewRandomSource())
	wg.Add(1)
	go repeatedGet(transformer, end, &wg)

	listmerger := cached.MergeList(func(results []cached.Result[int64]) (int64, string, error) {
		sum := int64(0)
		for i := range results {
			sum += results[i].Value
		}
		return sum, randomString(10), nil
	}, []cached.Value[int64]{static, result, replaceable, merger, transformer})
	wg.Add(1)
	go repeatedGet(listmerger, end, &wg)

	wg.Wait()
}
