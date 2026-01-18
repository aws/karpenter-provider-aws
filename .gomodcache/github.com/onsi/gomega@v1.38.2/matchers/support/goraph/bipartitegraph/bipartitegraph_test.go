package bipartitegraph_test

import (
	"fmt"
	"reflect"

	. "github.com/onsi/gomega/matchers/support/goraph/bipartitegraph"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bipartitegraph", func() {
	Context("tiny graphs", func() {
		var (
			empty, _        = NewBipartiteGraph([]any{}, []any{}, func(x, y any) (bool, error) { return true, nil })
			oneLeft, _      = NewBipartiteGraph([]any{1}, []any{}, func(x, y any) (bool, error) { return true, nil })
			oneRight, _     = NewBipartiteGraph([]any{}, []any{1}, func(x, y any) (bool, error) { return true, nil })
			twoSeparate, _  = NewBipartiteGraph([]any{1}, []any{1}, func(x, y any) (bool, error) { return false, nil })
			twoConnected, _ = NewBipartiteGraph([]any{1}, []any{1}, func(x, y any) (bool, error) { return true, nil })
		)

		It("Computes the correct largest matching", func() {
			Ω(empty.LargestMatching()).Should(BeEmpty())
			Ω(oneLeft.LargestMatching()).Should(BeEmpty())
			Ω(oneRight.LargestMatching()).Should(BeEmpty())
			Ω(twoSeparate.LargestMatching()).Should(BeEmpty())

			Ω(twoConnected.LargestMatching()).Should(HaveLen(1))
		})
	})

	Context("small yet complex graphs", func() {
		var (
			neighbours = func(x, y any) (bool, error) {
				switch x.(string) + y.(string) {
				case "aw", "bw", "bx", "cy", "cz", "dx", "ew":
					return true, nil
				default:
					return false, nil
				}
			}
			graph, _ = NewBipartiteGraph(
				[]any{"a", "b", "c", "d", "e"},
				[]any{"w", "x", "y", "z"},
				neighbours,
			)
		)

		It("Computes the correct largest matching", func() {
			// largest matching: "aw", "bx", "cy"
			Ω(graph.LargestMatching()).Should(HaveLen(3))
		})

		Describe("FreeLeftRight", func() {
			When("all edges are given", func() {
				It("returns correct free left and right values", func() {
					freeLeft, freeRight := graph.FreeLeftRight(graph.Edges)
					Expect(freeLeft).To(BeEmpty())
					Expect(freeRight).To(BeEmpty())
				})
			})
			When("largest matching edges are given", func() {
				It("returns correct free left and right values", func() {
					edges := graph.LargestMatching()
					freeLeft, freeRight := graph.FreeLeftRight(edges)
					Expect(freeLeft).To(ConsistOf("d", "e"))
					Expect(freeRight).To(ConsistOf("z"))
				})
			})
		})
	})

	When("node values are unhashable types", func() {
		var (
			neighbours = func(x, y any) (bool, error) {
				return reflect.DeepEqual(x, y), nil
			}
			graph, _ = NewBipartiteGraph(
				[]any{[]int{1, 2}, []int{3, 4}},
				[]any{[]int{1, 2}},
				neighbours,
			)
		)
		Describe("FreeLeftRight", func() {
			It("returns correct free left and right values", func() {
				edges := graph.LargestMatching()
				freeLeft, freeRight := graph.FreeLeftRight(edges)
				Expect(freeLeft).To(HaveLen(1))
				Expect(freeLeft[0]).To(Equal([]int{3, 4}))
				Expect(freeRight).To(BeEmpty())
			})
		})
	})

	Context("large yet simple graphs", func() {
		var (
			half                = make([]any, 100)
			discreteNeighbours  = func(x, y any) (bool, error) { return false, nil }
			completeNeighbours  = func(x, y any) (bool, error) { return true, nil }
			bijectionNeighbours = func(x, y any) (bool, error) {
				return x.(int) == y.(int), nil
			}
			discrete, complete, bijection *BipartiteGraph
		)

		BeforeEach(func() {
			for i := 0; i < len(half); i++ {
				half[i] = i
			}
			discrete, _ = NewBipartiteGraph(half, half, discreteNeighbours)
			complete, _ = NewBipartiteGraph(half, half, completeNeighbours)
			bijection, _ = NewBipartiteGraph(half, half, bijectionNeighbours)
		})

		It("Computes the correct largest matching", func() {
			Ω(discrete.LargestMatching()).Should(BeEmpty())
			Ω(complete.LargestMatching()).Should(HaveLen(100))
			Ω(bijection.LargestMatching()).Should(HaveLen(100))
		})
	})

	Context("large graphs that are unpleasant for the algorithm", func() {
		var (
			half        = make([]any, 100)
			neighbours1 = func(x, y any) (bool, error) {
				if x.(int) < 33 {
					return x.(int) == y.(int), nil
				} else if x.(int) < 66 {
					return true, nil
				} else {
					return false, nil
				}
			}
			neighbours2 = func(x, y any) (bool, error) {
				if x.(int) == 50 {
					return true, nil
				} else if x.(int) < 90 {
					return x.(int) == y.(int), nil
				} else {
					return false, nil
				}
			}
			neighbours3 = func(x, y any) (bool, error) {
				if y.(int) < x.(int)-20 {
					return true, nil
				} else {
					return false, nil
				}
			}
			graph1, graph2, graph3 *BipartiteGraph
		)

		BeforeEach(func() {
			for i := 0; i < len(half); i++ {
				half[i] = i
			}
			graph1, _ = NewBipartiteGraph(half, half, neighbours1)
			graph2, _ = NewBipartiteGraph(half, half, neighbours2)
			graph3, _ = NewBipartiteGraph(half, half, neighbours3)
		})

		It("Computes the correct largest matching", func() {
			Ω(graph1.LargestMatching()).Should(HaveLen(66))
			Ω(graph2.LargestMatching()).Should(HaveLen(90))
			Ω(graph3.LargestMatching()).Should(HaveLen(79))
		})
	})

	Describe("Edge case in Issue #765", func() {
		It("is now resolved", func() {
			knownEdges := map[string]bool{
				"1A": true,
				"1B": true,
				"1C": true,
				"1D": true,
				"1E": true,
				"2A": true,
				"2D": true,
				"3B": true,
				"3D": true,
				"4B": true,
				"4D": true,
				"4E": true,
				"5A": true,
			}

			edgesFunc := func(l, r any) (bool, error) {
				return knownEdges[fmt.Sprintf("%v%v", l, r)], nil
			}

			vertices := []any{"1", "2", "3", "4", "5", "A", "B", "C", "D", "E"}
			leftPart := vertices[:5]
			rightPart := vertices[5:]

			bipartiteGraph, err := NewBipartiteGraph(leftPart, rightPart, edgesFunc)
			Ω(err).ShouldNot(HaveOccurred())
			edgeSet := bipartiteGraph.LargestMatching()
			Ω(edgeSet).Should(HaveLen(5))

			result := []string{}
			for _, edge := range edgeSet {
				result = append(result, fmt.Sprintf("%v%v", vertices[edge.Node1], vertices[edge.Node2]))
			}
			Ω(result).Should(ConsistOf("1C", "2D", "3B", "4E", "5A"))

		})
	})
})
