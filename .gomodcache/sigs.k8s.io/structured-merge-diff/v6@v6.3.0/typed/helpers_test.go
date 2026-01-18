package typed_test

import (
	"strings"
	"testing"

	"sigs.k8s.io/structured-merge-diff/v6/internal/fixture"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

func TestInvalidOverride(t *testing.T) {
	// Exercises code path for invalidly specifying a scalar type is atomic
	parser, err := typed.NewParser(`
    types:
    - name: type
      map:
        fields:
          - name: field
            type:
              scalar: numeric
              elementRelationship: atomic
      `)

	if err != nil {
		t.Fatal(err)
	}

	sameVersionParser := fixture.SameVersionParser{T: parser.Type("type")}

	test := fixture.TestCase{
		Ops: []fixture.Operation{
			fixture.Apply{
				Manager: "apply_one",
				Object: `
                        field: 1
                    `,
				APIVersion: "v1",
			},
		},
		APIVersion: "v1",
	}

	if err := test.Test(sameVersionParser); err == nil ||
		!strings.Contains(err.Error(), "no type found matching: inlined type") {
		t.Fatal(err)
	}
}
