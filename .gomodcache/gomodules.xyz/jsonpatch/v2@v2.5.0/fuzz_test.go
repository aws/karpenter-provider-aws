package jsonpatch_test

import (
	"encoding/json"
	"testing"

	jp "github.com/evanphx/json-patch"
	"github.com/stretchr/testify/assert"
	"gomodules.xyz/jsonpatch/v2"
)

func FuzzCreatePatch(f *testing.F) {
	add := func(a, b string) {
		f.Add([]byte(a), []byte(b))
	}
	add(simpleA, simpleB)
	add(superComplexBase, superComplexA)
	add(hyperComplexBase, hyperComplexA)
	add(arraySrc, arrayDst)
	add(empty, simpleA)
	add(point, lineString)
	f.Fuzz(func(t *testing.T, a, b []byte) {
		checkFuzz(t, a, b)
	})
}

func checkFuzz(t *testing.T, src, dst []byte) {
	t.Logf("Test: %v -> %v", string(src), string(dst))
	patch, err := jsonpatch.CreatePatch(src, dst)
	if err != nil {
		// Ok to error, src or dst may be invalid
		t.Skip()
	}

	// Applying library only works with arrays and structs, no primitives
	// We still do CreatePatch to make sure it doesn't panic
	if isPrimitive(src) || isPrimitive(dst) {
		return
	}

	for _, p := range patch {
		if p.Path == "" {
			// json-patch doesn't handle this properly, but it is valid
			return
		}
	}

	data, err := json.Marshal(patch)
	assert.Nil(t, err)

	t.Logf("Applying patch %v", string(data))
	p2, err := jp.DecodePatch(data)
	assert.Nil(t, err)

	d2, err := p2.Apply(src)
	assert.Nil(t, err)

	assert.JSONEq(t, string(dst), string(d2))
}

func isPrimitive(data []byte) bool {
	return data[0] != '{' && data[0] != '['
}
