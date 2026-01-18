package mergo_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/imdario/mergo"
)

type structWithTime struct {
	Birth time.Time
}

type timeTransfomer struct {
	overwrite bool
}

func (t timeTransfomer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(time.Time{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				if t.overwrite {
					isZero := src.MethodByName("IsZero")

					result := isZero.Call([]reflect.Value{})
					if !result[0].Bool() {
						dst.Set(src)
					}
				} else {
					isZero := dst.MethodByName("IsZero")

					result := isZero.Call([]reflect.Value{})
					if result[0].Bool() {
						dst.Set(src)
					}
				}
			}
			return nil
		}
	}
	return nil
}

func TestOverwriteZeroSrcTime(t *testing.T) {
	now := time.Now()
	dst := structWithTime{now}
	src := structWithTime{}

	if err := mergo.MergeWithOverwrite(&dst, src); err != nil {
		t.FailNow()
	}

	if !dst.Birth.IsZero() {
		t.Errorf("dst should have been overwritten: dst.Birth(%v) != now(%v)", dst.Birth, now)
	}
}

func TestOverwriteZeroSrcTimeWithTransformer(t *testing.T) {
	now := time.Now()
	dst := structWithTime{now}
	src := structWithTime{}

	if err := mergo.MergeWithOverwrite(&dst, src, mergo.WithTransformers(timeTransfomer{true})); err != nil {
		t.FailNow()
	}

	if dst.Birth.IsZero() {
		t.Errorf("dst should not have been overwritten: dst.Birth(%v) != now(%v)", dst.Birth, now)
	}
}

func TestOverwriteZeroDstTime(t *testing.T) {
	now := time.Now()
	dst := structWithTime{}
	src := structWithTime{now}

	if err := mergo.MergeWithOverwrite(&dst, src); err != nil {
		t.FailNow()
	}

	if dst.Birth.IsZero() {
		t.Errorf("dst should have been overwritten: dst.Birth(%v) != zero(%v)", dst.Birth, time.Time{})
	}
}

func TestZeroDstTime(t *testing.T) {
	now := time.Now()
	dst := structWithTime{}
	src := structWithTime{now}

	if err := mergo.Merge(&dst, src); err != nil {
		t.FailNow()
	}

	if !dst.Birth.IsZero() {
		t.Errorf("dst should not have been overwritten: dst.Birth(%v) != zero(%v)", dst.Birth, time.Time{})
	}
}

func TestZeroDstTimeWithTransformer(t *testing.T) {
	now := time.Now()
	dst := structWithTime{}
	src := structWithTime{now}

	if err := mergo.Merge(&dst, src, mergo.WithTransformers(timeTransfomer{})); err != nil {
		t.FailNow()
	}

	if dst.Birth.IsZero() {
		t.Errorf("dst should have been overwritten: dst.Birth(%v) != now(%v)", dst.Birth, now)
	}
}
