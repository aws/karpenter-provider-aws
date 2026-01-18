package mergo

import (
	"reflect"
	"testing"
	"time"
)

type transformer struct {
}

func (s *transformer) Transformer(t reflect.Type) func(dst, src reflect.Value) error {
	return nil
}

func Test_deepMergeTransformerInvalidDestination(t *testing.T) {
	foo := time.Time{}
	src := reflect.ValueOf(foo)
	deepMerge(reflect.Value{}, src, make(map[uintptr]*visit), 0, &Config{
		Transformers: &transformer{},
	})
	// this test is intentionally not asserting on anything, it's sole
	// purpose to verify deepMerge doesn't panic when a transformer is
	// passed and the destination is invalid.
}
