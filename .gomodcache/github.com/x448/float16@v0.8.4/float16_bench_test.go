// Copyright 2019 Montgomery Edwards⁴⁴⁸ and Faye Amacker

package float16_test

import (
	"math"
	"testing"

	"github.com/x448/float16"
)

// prevent compiler optimizing out code by assigning to these
var resultF16 float16.Float16
var resultF32 float32
var resultStr string
var pcn float16.Precision

func BenchmarkFloat32pi(b *testing.B) {
	result := float32(0)
	pi32 := float32(math.Pi)
	pi16 := float16.Fromfloat32(pi32)
	for i := 0; i < b.N; i++ {
		f16 := float16.Frombits(uint16(pi16))
		result = f16.Float32()
	}
	resultF32 = result
}

func BenchmarkFrombits(b *testing.B) {
	result := float16.Float16(0)
	pi32 := float32(math.Pi)
	pi16 := float16.Fromfloat32(pi32)
	for i := 0; i < b.N; i++ {
		result = float16.Frombits(uint16(pi16))
	}
	resultF16 = result
}

func BenchmarkFromFloat32pi(b *testing.B) {
	result := float16.Float16(0)

	pi := float32(math.Pi)
	for i := 0; i < b.N; i++ {
		result = float16.Fromfloat32(pi)
	}
	resultF16 = result
}

func BenchmarkFromFloat32nan(b *testing.B) {
	result := float16.Float16(0)

	nan := float32(math.NaN())
	for i := 0; i < b.N; i++ {
		result = float16.Fromfloat32(nan)
	}
	resultF16 = result
}

func BenchmarkFromFloat32subnorm(b *testing.B) {
	result := float16.Float16(0)

	subnorm := math.Float32frombits(0x007fffff)
	for i := 0; i < b.N; i++ {
		result = float16.Fromfloat32(subnorm)
	}
	resultF16 = result
}

func BenchmarkPrecisionFromFloat32(b *testing.B) {
	var result float16.Precision

	//pi := float32(math.Pi)
	for i := 0; i < b.N; i++ {
		f32 := float32(0.00001) + float32(0.00001)
		result = float16.PrecisionFromfloat32(f32)
	}
	pcn = result
}

func BenchmarkString(b *testing.B) {
	result := "1.5"

	pi32 := float32(math.Pi)
	pi16 := float16.Fromfloat32(pi32)
	for i := 0; i < b.N; i++ {
		result = pi16.String()
	}
	resultStr = result
}
