/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package predicate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("Predicate", func() {
	var pod *corev1.Pod
	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "biz", Name: "baz"},
		}
	})

	Describe("Funcs", func() {
		failingFuncs := predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool {
				defer GinkgoRecover()
				Fail("Did not expect CreateFunc to be called.")
				return false
			},
			DeleteFunc: func(event.DeleteEvent) bool {
				defer GinkgoRecover()
				Fail("Did not expect DeleteFunc to be called.")
				return false
			},
			UpdateFunc: func(event.UpdateEvent) bool {
				defer GinkgoRecover()
				Fail("Did not expect UpdateFunc to be called.")
				return false
			},
			GenericFunc: func(event.GenericEvent) bool {
				defer GinkgoRecover()
				Fail("Did not expect GenericFunc to be called.")
				return false
			},
		}

		It("should call Create", func() {
			instance := failingFuncs
			instance.CreateFunc = func(evt event.CreateEvent) bool {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
				return false
			}
			evt := event.CreateEvent{
				Object: pod,
			}
			Expect(instance.Create(evt)).To(BeFalse())

			instance.CreateFunc = func(evt event.CreateEvent) bool {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
				return true
			}
			Expect(instance.Create(evt)).To(BeTrue())

			instance.CreateFunc = nil
			Expect(instance.Create(evt)).To(BeTrue())
		})

		It("should call Update", func() {
			newPod := pod.DeepCopy()
			newPod.Name = "baz2"
			newPod.Namespace = "biz2"

			instance := failingFuncs
			instance.UpdateFunc = func(evt event.UpdateEvent) bool {
				defer GinkgoRecover()
				Expect(evt.ObjectOld).To(Equal(pod))
				Expect(evt.ObjectNew).To(Equal(newPod))
				return false
			}
			evt := event.UpdateEvent{
				ObjectOld: pod,
				ObjectNew: newPod,
			}
			Expect(instance.Update(evt)).To(BeFalse())

			instance.UpdateFunc = func(evt event.UpdateEvent) bool {
				defer GinkgoRecover()
				Expect(evt.ObjectOld).To(Equal(pod))
				Expect(evt.ObjectNew).To(Equal(newPod))
				return true
			}
			Expect(instance.Update(evt)).To(BeTrue())

			instance.UpdateFunc = nil
			Expect(instance.Update(evt)).To(BeTrue())
		})

		It("should call Delete", func() {
			instance := failingFuncs
			instance.DeleteFunc = func(evt event.DeleteEvent) bool {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
				return false
			}
			evt := event.DeleteEvent{
				Object: pod,
			}
			Expect(instance.Delete(evt)).To(BeFalse())

			instance.DeleteFunc = func(evt event.DeleteEvent) bool {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
				return true
			}
			Expect(instance.Delete(evt)).To(BeTrue())

			instance.DeleteFunc = nil
			Expect(instance.Delete(evt)).To(BeTrue())
		})

		It("should call Generic", func() {
			instance := failingFuncs
			instance.GenericFunc = func(evt event.GenericEvent) bool {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
				return false
			}
			evt := event.GenericEvent{
				Object: pod,
			}
			Expect(instance.Generic(evt)).To(BeFalse())

			instance.GenericFunc = func(evt event.GenericEvent) bool {
				defer GinkgoRecover()
				Expect(evt.Object).To(Equal(pod))
				return true
			}
			Expect(instance.Generic(evt)).To(BeTrue())

			instance.GenericFunc = nil
			Expect(instance.Generic(evt)).To(BeTrue())
		})
	})

	Describe("When checking a ResourceVersionChangedPredicate", func() {
		instance := predicate.ResourceVersionChangedPredicate{}

		Context("Where the old object doesn't have a ResourceVersion or metadata", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "1",
					}}

				failEvnt := event.UpdateEvent{
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).Should(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).Should(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).Should(BeTrue())
				Expect(instance.Update(failEvnt)).Should(BeFalse())
			})
		})

		Context("Where the new object doesn't have a ResourceVersion or metadata", func() {
			It("should return false", func() {
				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "1",
					}}

				failEvnt := event.UpdateEvent{
					ObjectOld: oldPod,
				}
				Expect(instance.Create(event.CreateEvent{})).Should(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).Should(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).Should(BeTrue())
				Expect(instance.Update(failEvnt)).Should(BeFalse())
				Expect(instance.Update(failEvnt)).Should(BeFalse())
			})
		})

		Context("Where the ResourceVersion hasn't changed", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "v1",
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "v1",
					}}

				failEvnt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).Should(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).Should(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).Should(BeTrue())
				Expect(instance.Update(failEvnt)).Should(BeFalse())
				Expect(instance.Update(failEvnt)).Should(BeFalse())
			})
		})

		Context("Where the ResourceVersion has changed", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "v1",
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "v2",
					}}
				passEvt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).Should(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).Should(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).Should(BeTrue())
				Expect(instance.Update(passEvt)).Should(BeTrue())
			})
		})

		Context("Where the objects or metadata are missing", func() {

			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "v1",
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "baz",
						Namespace:       "biz",
						ResourceVersion: "v1",
					}}

				failEvt1 := event.UpdateEvent{ObjectOld: oldPod}
				failEvt2 := event.UpdateEvent{ObjectNew: newPod}
				failEvt3 := event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}
				Expect(instance.Create(event.CreateEvent{})).Should(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).Should(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).Should(BeTrue())
				Expect(instance.Update(failEvt1)).Should(BeFalse())
				Expect(instance.Update(failEvt2)).Should(BeFalse())
				Expect(instance.Update(failEvt3)).Should(BeFalse())
			})
		})

	})

	Describe("When checking a GenerationChangedPredicate", func() {
		instance := predicate.GenerationChangedPredicate{}
		Context("Where the old object doesn't have a Generation or metadata", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 1,
					}}

				failEvnt := event.UpdateEvent{
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvnt)).To(BeFalse())
			})
		})

		Context("Where the new object doesn't have a Generation or metadata", func() {
			It("should return false", func() {
				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 1,
					}}

				failEvnt := event.UpdateEvent{
					ObjectOld: oldPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvnt)).To(BeFalse())
			})
		})

		Context("Where the Generation hasn't changed", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 1,
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 1,
					}}

				failEvnt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvnt)).To(BeFalse())
			})
		})

		Context("Where the Generation has changed", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 1,
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 2,
					}}
				passEvt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(passEvt)).To(BeTrue())
			})
		})

		Context("Where the objects or metadata are missing", func() {

			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 1,
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "baz",
						Namespace:  "biz",
						Generation: 1,
					}}

				failEvt1 := event.UpdateEvent{ObjectOld: oldPod}
				failEvt2 := event.UpdateEvent{ObjectNew: newPod}
				failEvt3 := event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvt1)).To(BeFalse())
				Expect(instance.Update(failEvt2)).To(BeFalse())
				Expect(instance.Update(failEvt3)).To(BeFalse())
			})
		})

	})

	// AnnotationChangedPredicate has almost identical test cases as LabelChangedPredicates,
	// so the duplication linter should be muted on both two test suites.
	Describe("When checking an AnnotationChangedPredicate", func() {
		instance := predicate.AnnotationChangedPredicate{}
		Context("Where the old object is missing", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
						},
					}}

				failEvnt := event.UpdateEvent{
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvnt)).To(BeFalse())
			})
		})

		Context("Where the new object is missing", func() {
			It("should return false", func() {
				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
						},
					}}

				failEvnt := event.UpdateEvent{
					ObjectOld: oldPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvnt)).To(BeFalse())
			})
		})

		Context("Where the annotations are empty", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
					}}

				failEvnt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvnt)).To(BeFalse())
			})
		})

		Context("Where the annotations haven't changed", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
						},
					}}

				failEvnt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(failEvnt)).To(BeFalse())
			})
		})

		Context("Where an annotation value has changed", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "weez",
						},
					}}

				passEvt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(passEvt)).To(BeTrue())
			})
		})

		Context("Where an annotation has been added", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
							"zooz": "qooz",
						},
					}}

				passEvt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(passEvt)).To(BeTrue())
			})
		})

		Context("Where an annotation has been removed", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
							"zooz": "qooz",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Annotations: map[string]string{
							"booz": "wooz",
						},
					}}

				passEvt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(passEvt)).To(BeTrue())
			})
		})
	})

	// LabelChangedPredicates has almost identical test cases as AnnotationChangedPredicates,
	// so the duplication linter should be muted on both two test suites.
	Describe("When checking a LabelChangedPredicate", func() {
		instance := predicate.LabelChangedPredicate{}
		Context("Where the old object is missing", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
						},
					}}

				evt := event.UpdateEvent{
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(evt)).To(BeFalse())
			})
		})

		Context("Where the new object is missing", func() {
			It("should return false", func() {
				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
						},
					}}

				evt := event.UpdateEvent{
					ObjectOld: oldPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(evt)).To(BeFalse())
			})
		})

		Context("Where the labels are empty", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
					}}

				evt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(evt)).To(BeFalse())
			})
		})

		Context("Where the labels haven't changed", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
						},
					}}

				evt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(evt)).To(BeFalse())
			})
		})

		Context("Where a label value has changed", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bee",
						},
					}}

				evt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(evt)).To(BeTrue())
			})
		})

		Context("Where a label has been added", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
							"faa": "bor",
						},
					}}

				evt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(evt)).To(BeTrue())
			})
		})

		Context("Where a label has been removed", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
							"faa": "bor",
						},
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
						Labels: map[string]string{
							"foo": "bar",
						},
					}}

				evt := event.UpdateEvent{
					ObjectOld: oldPod,
					ObjectNew: newPod,
				}
				Expect(instance.Create(event.CreateEvent{})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{})).To(BeTrue())
				Expect(instance.Update(evt)).To(BeTrue())
			})
		})
	})

	Context("With a boolean predicate", func() {
		funcs := func(pass bool) predicate.Funcs {
			return predicate.Funcs{
				CreateFunc: func(event.CreateEvent) bool {
					return pass
				},
				DeleteFunc: func(event.DeleteEvent) bool {
					return pass
				},
				UpdateFunc: func(event.UpdateEvent) bool {
					return pass
				},
				GenericFunc: func(event.GenericEvent) bool {
					return pass
				},
			}
		}
		passFuncs := funcs(true)
		failFuncs := funcs(false)

		Describe("When checking an And predicate", func() {
			It("should return false when one of its predicates returns false", func() {
				a := predicate.And(passFuncs, failFuncs)
				Expect(a.Create(event.CreateEvent{})).To(BeFalse())
				Expect(a.Update(event.UpdateEvent{})).To(BeFalse())
				Expect(a.Delete(event.DeleteEvent{})).To(BeFalse())
				Expect(a.Generic(event.GenericEvent{})).To(BeFalse())
			})
			It("should return true when all of its predicates return true", func() {
				a := predicate.And(passFuncs, passFuncs)
				Expect(a.Create(event.CreateEvent{})).To(BeTrue())
				Expect(a.Update(event.UpdateEvent{})).To(BeTrue())
				Expect(a.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(a.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
		Describe("When checking an Or predicate", func() {
			It("should return true when one of its predicates returns true", func() {
				o := predicate.Or(passFuncs, failFuncs)
				Expect(o.Create(event.CreateEvent{})).To(BeTrue())
				Expect(o.Update(event.UpdateEvent{})).To(BeTrue())
				Expect(o.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(o.Generic(event.GenericEvent{})).To(BeTrue())
			})
			It("should return false when all of its predicates return false", func() {
				o := predicate.Or(failFuncs, failFuncs)
				Expect(o.Create(event.CreateEvent{})).To(BeFalse())
				Expect(o.Update(event.UpdateEvent{})).To(BeFalse())
				Expect(o.Delete(event.DeleteEvent{})).To(BeFalse())
				Expect(o.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
		Describe("When checking a Not predicate", func() {
			It("should return false when its predicate returns true", func() {
				n := predicate.Not(passFuncs)
				Expect(n.Create(event.CreateEvent{})).To(BeFalse())
				Expect(n.Update(event.UpdateEvent{})).To(BeFalse())
				Expect(n.Delete(event.DeleteEvent{})).To(BeFalse())
				Expect(n.Generic(event.GenericEvent{})).To(BeFalse())
			})
			It("should return true when its predicate returns false", func() {
				n := predicate.Not(failFuncs)
				Expect(n.Create(event.CreateEvent{})).To(BeTrue())
				Expect(n.Update(event.UpdateEvent{})).To(BeTrue())
				Expect(n.Delete(event.DeleteEvent{})).To(BeTrue())
				Expect(n.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("NewPredicateFuncs with a namespace filter function", func() {
		byNamespaceFilter := func(namespace string) func(object client.Object) bool {
			return func(object client.Object) bool {
				return object.GetNamespace() == namespace
			}
		}
		byNamespaceFuncs := predicate.NewPredicateFuncs(byNamespaceFilter("biz"))
		Context("Where the namespace is matching", func() {
			It("should return true", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
					}}
				passEvt1 := event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}
				Expect(byNamespaceFuncs.Create(event.CreateEvent{Object: newPod})).To(BeTrue())
				Expect(byNamespaceFuncs.Delete(event.DeleteEvent{Object: oldPod})).To(BeTrue())
				Expect(byNamespaceFuncs.Generic(event.GenericEvent{Object: newPod})).To(BeTrue())
				Expect(byNamespaceFuncs.Update(passEvt1)).To(BeTrue())
			})
		})

		Context("Where the namespace is not matching", func() {
			It("should return false", func() {
				newPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "bizz",
					}}

				oldPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz",
						Namespace: "biz",
					}}
				failEvt1 := event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}
				Expect(byNamespaceFuncs.Create(event.CreateEvent{Object: newPod})).To(BeFalse())
				Expect(byNamespaceFuncs.Delete(event.DeleteEvent{Object: newPod})).To(BeFalse())
				Expect(byNamespaceFuncs.Generic(event.GenericEvent{Object: newPod})).To(BeFalse())
				Expect(byNamespaceFuncs.Update(failEvt1)).To(BeFalse())
			})
		})
	})

	Describe("When checking a LabelSelectorPredicate", func() {
		instance, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}})
		if err != nil {
			Fail("Improper Label Selector passed during predicate instantiation.")
		}

		Context("When the Selector does not match the event labels", func() {
			It("should return false", func() {
				failMatch := &corev1.Pod{}
				Expect(instance.Create(event.CreateEvent{Object: failMatch})).To(BeFalse())
				Expect(instance.Delete(event.DeleteEvent{Object: failMatch})).To(BeFalse())
				Expect(instance.Generic(event.GenericEvent{Object: failMatch})).To(BeFalse())
				Expect(instance.Update(event.UpdateEvent{ObjectNew: failMatch})).To(BeFalse())
			})
		})

		Context("When the Selector matches the event labels", func() {
			It("should return true", func() {
				successMatch := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"foo": "bar"},
					},
				}
				Expect(instance.Create(event.CreateEvent{Object: successMatch})).To(BeTrue())
				Expect(instance.Delete(event.DeleteEvent{Object: successMatch})).To(BeTrue())
				Expect(instance.Generic(event.GenericEvent{Object: successMatch})).To(BeTrue())
				Expect(instance.Update(event.UpdateEvent{ObjectNew: successMatch})).To(BeTrue())
			})
		})
	})
})
