package hashstructure

import (
	"fmt"
)

func ExampleHash() {
	type ComplexStruct struct {
		Name     string
		Age      uint
		Metadata map[string]interface{}
	}

	v := ComplexStruct{
		Name: "mitchellh",
		Age:  64,
		Metadata: map[string]interface{}{
			"car":      true,
			"location": "California",
			"siblings": []string{"Bob", "John"},
		},
	}

	hash, err := Hash(v, FormatV2, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%d", hash)
	// Output:
	// 1839806922502695369
}

func ExampleHash_v1() {
	type ComplexStruct struct {
		Name     string
		Age      uint
		Metadata map[string]interface{}
	}

	v := ComplexStruct{
		Name: "mitchellh",
		Age:  64,
		Metadata: map[string]interface{}{
			"car":      true,
			"location": "California",
			"siblings": []string{"Bob", "John"},
		},
	}

	hash, err := Hash(v, FormatV1, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%d", hash)
	// Output:
	// 6691276962590150517
}
