package mergo_test

import (
	"reflect"
	"testing"

	"github.com/imdario/mergo"
)

type Record struct {
	Data    map[string]interface{}
	Mapping map[string]string
}

func StructToRecord(in interface{}) *Record {
	rec := Record{}
	rec.Data = make(map[string]interface{})
	rec.Mapping = make(map[string]string)
	typ := reflect.TypeOf(in)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		dbFieldName := field.Tag.Get("db")
		if dbFieldName != "" {
			rec.Mapping[field.Name] = dbFieldName
		}
	}

	if err := mergo.Map(&rec.Data, in); err != nil {
		panic(err)
	}
	return &rec
}

func TestStructToRecord(t *testing.T) {
	type A struct {
		Name string `json:"name" db:"name"`
		CIDR string `json:"cidr" db:"cidr"`
	}
	type Record struct {
		Data    map[string]interface{}
		Mapping map[string]string
	}
	a := A{Name: "David", CIDR: "10.0.0.0/8"}
	rec := StructToRecord(a)
	if len(rec.Mapping) < 2 {
		t.Fatalf("struct to record failed, no mapping, struct missing tags?, rec: %+v, a: %+v ", rec, a)
	}
}
