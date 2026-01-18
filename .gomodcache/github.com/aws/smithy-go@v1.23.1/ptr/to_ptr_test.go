package ptr

import (
	"testing"
	"time"
)

func TestBool(t *testing.T) {
	var v *bool
	v = Bool(true)
	if !*v {
		t.Errorf("expected %t, but received %t", true, *v)
	}
	v = Bool(false)
	if *v {
		t.Errorf("expected %t, but received %t", false, *v)
	}
}

func TestBoolSlice(t *testing.T) {
	s := []bool{true, false}
	ps := BoolSlice(s)
	if len(ps) != 2 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if !*ps[0] {
		t.Errorf("expected %t, but received %t", true, *ps[0])
	}
	if *ps[1] {
		t.Errorf("expected %t, but received %t", false, *ps[1])
	}
}

func TestBoolMap(t *testing.T) {
	s := map[string]bool{
		"true":  true,
		"false": false,
	}
	ps := BoolMap(s)
	if len(ps) != 2 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if !*ps["true"] {
		t.Errorf("expected %t, but received %t", true, *ps["true"])
	}
	if *ps["false"] {
		t.Errorf("expected %t, but received %t", false, *ps["false"])
	}
}

func TestByte(t *testing.T) {
	v := Byte(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestByteSlice(t *testing.T) {
	s := []byte{1, 1, 2, 3, 5, 8, 13, 21}
	ps := ByteSlice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestByteMap(t *testing.T) {
	s := map[string]byte{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := ByteMap(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestString(t *testing.T) {
	v := String("foo")
	if *v != "foo" {
		t.Errorf("expected %q, but received %q", "foo", *v)
	}
}

func TestStringSlice(t *testing.T) {
	s := []string{"foo", "bar", "fizz", "buzz", "hoge", "fuga"}
	ps := StringSlice(s)
	if len(ps) != 6 {
		t.Errorf("expected %d, but received %d", 6, len(ps))
	}
	if *ps[0] != "foo" {
		t.Errorf("expected %q, but received %q", "foo", *ps[0])
	}
	if *ps[5] != "fuga" {
		t.Errorf("expected %q, but received %q", "fuga", *ps[5])
	}
}

func TestStringMap(t *testing.T) {
	s := map[string]string{
		"foo":  "bar",
		"fizz": "buzz",
		"hoge": "fuga",
	}
	ps := StringMap(s)
	if len(ps) != 3 {
		t.Errorf("expected %d, but received %d", 3, len(ps))
	}
	if *ps["foo"] != "bar" {
		t.Errorf("expected %q, but received %q", "foo", *ps["foo"])
	}
	if *ps["hoge"] != "fuga" {
		t.Errorf("expected %q, but received %q", "fuga", *ps["hoge"])
	}
}

func TestInt(t *testing.T) {
	v := Int(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestIntSlice(t *testing.T) {
	s := []int{1, 1, 2, 3, 5, 8, 13, 21}
	ps := IntSlice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestIntMap(t *testing.T) {
	s := map[string]int{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := IntMap(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestInt8(t *testing.T) {
	v := Int8(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestInt8Slice(t *testing.T) {
	s := []int8{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Int8Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestInt8Map(t *testing.T) {
	s := map[string]int8{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := Int8Map(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestInt16(t *testing.T) {
	v := Int16(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestIntSlice16(t *testing.T) {
	s := []int16{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Int16Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestInt16Map(t *testing.T) {
	s := map[string]int16{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := Int16Map(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestInt32(t *testing.T) {
	v := Int32(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestInt32Slice(t *testing.T) {
	s := []int32{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Int32Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestInt32Map(t *testing.T) {
	s := map[string]int32{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := Int32Map(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestInt64(t *testing.T) {
	v := Int64(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestInt64Slice(t *testing.T) {
	s := []int64{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Int64Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestInt64Map(t *testing.T) {
	s := map[string]int64{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := Int64Map(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestUint(t *testing.T) {
	v := Uint(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestUintSlice(t *testing.T) {
	s := []uint{1, 1, 2, 3, 5, 8, 13, 21}
	ps := UintSlice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestUintMap(t *testing.T) {
	s := map[string]uint{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := UintMap(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestUint8(t *testing.T) {
	v := Uint8(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestUint8Slice(t *testing.T) {
	s := []uint8{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Uint8Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestUint8Map(t *testing.T) {
	s := map[string]uint8{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := Uint8Map(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestUint16(t *testing.T) {
	v := Uint16(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestUintSlice16(t *testing.T) {
	s := []uint16{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Uint16Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestUint16Map(t *testing.T) {
	s := map[string]uint{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := UintMap(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestUint32(t *testing.T) {
	v := Uint32(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestUint32Slice(t *testing.T) {
	s := []uint32{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Uint32Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestUint32Map(t *testing.T) {
	s := map[string]uint32{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := Uint32Map(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestUint64(t *testing.T) {
	v := Uint64(42)
	if *v != 42 {
		t.Errorf("expected %d, but received %d", 42, *v)
	}
}

func TestUint64Slice(t *testing.T) {
	s := []uint64{1, 1, 2, 3, 5, 8, 13, 21}
	ps := Uint64Slice(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 8, len(ps))
	}
	if *ps[0] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps[0])
	}
	if *ps[7] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps[7])
	}
}

func TestUint64Map(t *testing.T) {
	s := map[string]uint64{
		"F0": 1,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"F4": 5,
		"F5": 8,
		"F6": 13,
		"F7": 21,
	}
	ps := Uint64Map(s)
	if len(ps) != 8 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if *ps["F0"] != 1 {
		t.Errorf("expected %d, but received %d", 1, *ps["F0"])
	}
	if *ps["F7"] != 21 {
		t.Errorf("expected %d, but received %d", 21, *ps["F7"])
	}
}

func TestFloat32(t *testing.T) {
	v := Float32(0.5)
	if *v != 0.5 {
		t.Errorf("expected %f, but received %f", 0.5, *v)
	}
}

func TestFloat32Slice(t *testing.T) {
	s := []float32{0.5, 0.25, 0.125, 0.0625}
	ps := Float32Slice(s)
	if len(ps) != 4 {
		t.Errorf("expected %d, but received %d", 4, len(ps))
	}
	if *ps[0] != 0.5 {
		t.Errorf("expected %f, but received %f", 0.5, *ps[0])
	}
	if *ps[3] != 0.0625 {
		t.Errorf("expected %f, but received %f", 0.0625, *ps[7])
	}
}

func TestFloat32Map(t *testing.T) {
	s := map[string]float32{
		"F0": 0.5,
		"F1": 0.25,
		"F2": 0.125,
		"F3": 0.0625,
	}
	ps := Float32Map(s)
	if len(ps) != 4 {
		t.Errorf("expected %d, but received %d", 4, len(ps))
	}
	if *ps["F0"] != 0.5 {
		t.Errorf("expected %f, but received %f", 0.5, *ps["F0"])
	}
	if *ps["F3"] != 0.0625 {
		t.Errorf("expected %f, but received %f", 0.0625, *ps["F3"])
	}
}

func TestFloat64(t *testing.T) {
	v := Float64(0.5)
	if *v != 0.5 {
		t.Errorf("expected %f, but received %f", 0.5, *v)
	}
}

func TestFloat64Slice(t *testing.T) {
	s := []float64{0.5, 0.25, 0.125, 0.0625}
	ps := Float64Slice(s)
	if len(ps) != 4 {
		t.Errorf("expected %d, but received %d", 4, len(ps))
	}
	if *ps[0] != 0.5 {
		t.Errorf("expected %f, but received %f", 0.5, *ps[0])
	}
	if *ps[3] != 0.0625 {
		t.Errorf("expected %f, but received %f", 0.0625, *ps[7])
	}
}

func TestFloat64Map(t *testing.T) {
	s := map[string]float64{
		"F0": 0.5,
		"F1": 0.25,
		"F2": 0.125,
		"F3": 0.0625,
	}
	ps := Float64Map(s)
	if len(ps) != 4 {
		t.Errorf("expected %d, but received %d", 4, len(ps))
	}
	if *ps["F0"] != 0.5 {
		t.Errorf("expected %f, but received %f", 0.5, *ps["F0"])
	}
	if *ps["F3"] != 0.0625 {
		t.Errorf("expected %f, but received %f", 0.0625, *ps["F3"])
	}
}

func TestTime(t *testing.T) {
	v := Time(time.Unix(1234567890, 0))
	if v.Unix() != 1234567890 {
		t.Errorf("expected %d, but received %d", 1234567890, v.Unix())
	}
}

func TestTimeSlice(t *testing.T) {
	s := []time.Time{time.Unix(1234567890, 0), time.Unix(2147483647, 0)}
	ps := TimeSlice(s)
	if len(ps) != 2 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if ps[0].Unix() != 1234567890 {
		t.Errorf("expected %d, but received %d", 1234567890, ps[0].Unix())
	}
	if ps[1].Unix() != 2147483647 {
		t.Errorf("expected %d, but received %d", 2147483647, ps[1].Unix())
	}
}

func TestTimeMap(t *testing.T) {
	s := map[string]time.Time{
		"2009-02-13T23:31:30Z": time.Unix(1234567890, 0),
		"2038-01-19T03:14:07Z": time.Unix(2147483647, 0),
	}
	ps := TimeMap(s)
	if len(ps) != 2 {
		t.Errorf("expected %d, but received %d", 2, len(ps))
	}
	if ps["2009-02-13T23:31:30Z"].Unix() != 1234567890 {
		t.Errorf("expected %d, but received %d", 1234567890, ps["2009-02-13T23:31:30Z"].Unix())
	}
	if ps["2038-01-19T03:14:07Z"].Unix() != 2147483647 {
		t.Errorf("expected %d, but received %d", 2147483647, ps["2038-01-19T03:14:07Z"].Unix())
	}
}
