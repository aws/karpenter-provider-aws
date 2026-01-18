//go:build go1.23

package miter_test

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/onsi/gomega/matchers/internal/miter"
)

var _ = Describe("iterator function types", func() {

	When("detecting iterator functions", func() {

		It("doesn't match a nil value", func() {
			Expect(IsIter(nil)).To(BeFalse())
		})

		It("doesn't match a range-able numeric value", func() {
			Expect(IsIter(42)).To(BeFalse())
		})

		It("doesn't match a non-iter function", func() {
			Expect(IsIter(func(yabadabadu string) {})).To(BeFalse())
		})

		It("matches an iter.Seq-like iter function", func() {
			Expect(IsIter(func(yield func(v int) bool) {})).To(BeTrue())
			var nilIter func(func(string) bool)
			Expect(IsIter(nilIter)).To(BeTrue())
		})

		It("matches an iter.Seq2-like iter function", func() {
			Expect(IsIter(func(yield func(k uint, v string) bool) {})).To(BeTrue())
			var nilIter2 func(func(string, bool) bool)
			Expect(IsIter(nilIter2)).To(BeTrue())
		})

	})

	It("detects iter.Seq2", func() {
		Expect(IsSeq2(42)).To(BeFalse())
		Expect(IsSeq2(func(func(int) bool) {})).To(BeFalse())
		Expect(IsSeq2(func(func(int, int) bool) {})).To(BeTrue())

		var nilIter2 func(func(string, bool) bool)
		Expect(IsSeq2(nilIter2)).To(BeTrue())
	})

	When("getting iterator function K, V types", func() {

		It("has no types when nil", func() {
			k, v := IterKVTypes(nil)
			Expect(k).To(BeNil())
			Expect(v).To(BeNil())
		})

		It("has no types for range-able numbers", func() {
			k, v := IterKVTypes(42)
			Expect(k).To(BeNil())
			Expect(v).To(BeNil())
		})

		It("returns correct reflection type for the iterator's V", func() {
			type foo uint
			k, v := IterKVTypes(func(yield func(v foo) bool) {})
			Expect(k).To(BeNil())
			Expect(v).To(Equal(reflect.TypeOf(foo(42))))
		})

		It("returns correct reflection types for the iterator's K and V", func() {
			type foo uint
			type bar string
			k, v := IterKVTypes(func(yield func(k foo, v bar) bool) {})
			Expect(k).To(Equal(reflect.TypeOf(foo(42))))
			Expect(v).To(Equal(reflect.TypeOf(bar(""))))
		})

	})

	When("iterating single value reflections", func() {

		iterelements := []string{"foo", "bar", "baz"}

		it := func(yield func(v string) bool) {
			for _, el := range iterelements {
				if !yield(el) {
					break
				}
			}
		}

		It("doesn't loop over a nil iterator", func() {
			Expect(func() {
				IterateV(nil, func(v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("doesn't loop over a typed-nil iterator", func() {
			var nilIter func(func(string) bool)
			Expect(func() {
				IterateV(nilIter, func(v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("doesn't loop over a non-iterator value", func() {
			Expect(func() {
				IterateV(42, func(v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("doesn't loop over an iter.Seq2", func() {
			Expect(func() {
				IterateV(
					func(k uint, v string) bool { panic("it.Seq2 must not be called") },
					func(v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("yields all reflection values", func() {
			els := []string{}
			IterateV(it, func(v reflect.Value) bool {
				els = append(els, v.String())
				return true
			})
			Expect(els).To(ConsistOf(iterelements))
		})

		It("stops yielding reflection values before reaching THE END", func() {
			els := []string{}
			IterateV(it, func(v reflect.Value) bool {
				els = append(els, v.String())
				return len(els) < 2
			})
			Expect(els).To(ConsistOf(iterelements[:2]))
		})

	})

	When("iterating key-value reflections", func() {

		type kv struct {
			k uint
			v string
		}

		iterelements := []kv{
			{k: 42, v: "foo"},
			{k: 66, v: "bar"},
			{k: 666, v: "baz"},
		}

		it := func(yield func(k uint, v string) bool) {
			for _, el := range iterelements {
				if !yield(el.k, el.v) {
					break
				}
			}
		}

		It("doesn't loop over a nil iterator", func() {
			Expect(func() {
				IterateKV(nil, func(k, v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("doesn't loop over a typed-nil iterator", func() {
			var nilIter2 func(func(int, string) bool)
			Expect(func() {
				IterateKV(nilIter2, func(k, v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("doesn't loop over a non-iterator value", func() {
			Expect(func() {
				IterateKV(42, func(k, v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("doesn't loop over an iter.Seq", func() {
			Expect(func() {
				IterateKV(
					func(v string) bool { panic("it.Seq must not be called") },
					func(k, v reflect.Value) bool { panic("reflection yield must not be called") })
			}).NotTo(Panic())
		})

		It("yields all reflection key-values", func() {
			els := []kv{}
			IterateKV(it, func(k, v reflect.Value) bool {
				els = append(els, kv{k: uint(k.Uint()), v: v.String()})
				return true
			})
			Expect(els).To(ConsistOf(iterelements))
		})

		It("stops yielding reflection key-values before reaching THE END", func() {
			els := []kv{}
			IterateKV(it, func(k, v reflect.Value) bool {
				els = append(els, kv{k: uint(k.Uint()), v: v.String()})
				return len(els) < 2
			})
			Expect(els).To(ConsistOf(iterelements[:2]))
		})

	})

})
