package status_test

import (
	"github.com/awslabs/operatorpkg/status"
	"github.com/awslabs/operatorpkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Unstructured Adapter", func() {
	It("Get unstructured conditions condition values values", func() {
		testObject := &unstructured.Unstructured{Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "TestType",
						"status":  "False",
						"message": "test message",
						"reason":  "test reason",
					},
					map[string]interface{}{
						"type":    "TestType2",
						"status":  "True",
						"message": "test message 2",
						"reason":  "test reason 2",
					},
				},
			},
		}}
		testObject.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "testGroup",
			Version: "testVersion",
			Kind:    "testKind",
		})

		conditionObj := status.NewUnstructuredAdapter[*test.CustomObject](testObject)
		Expect(conditionObj).ToNot(BeNil())
		Expect(conditionObj.StatusConditions().Get("TestType").Message).To(Equal("test message"))
		Expect(conditionObj.StatusConditions().Get("TestType").Status).To(Equal(metav1.ConditionFalse))
		Expect(conditionObj.StatusConditions().Get("TestType").Reason).To(Equal("test reason"))
		Expect(conditionObj.StatusConditions().Get("TestType").Type).To(Equal("TestType"))
		Expect(conditionObj.StatusConditions().Get("TestType").ObservedGeneration).To(Equal(int64(0)))

		Expect(conditionObj.StatusConditions().Get("TestType2").Message).To(Equal("test message 2"))
		Expect(conditionObj.StatusConditions().Get("TestType2").Status).To(Equal(metav1.ConditionTrue))
		Expect(conditionObj.StatusConditions().Get("TestType2").Reason).To(Equal("test reason 2"))
		Expect(conditionObj.StatusConditions().Get("TestType2").Type).To(Equal("TestType2"))
		Expect(conditionObj.StatusConditions().Get("TestType2").ObservedGeneration).To(Equal(int64(0)))
	})
	It("Set unstructured Conditions", func() {
		testObject := &unstructured.Unstructured{Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		}}
		testObject.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "testGroup",
			Version: "testVersion",
			Kind:    "testKind",
		})

		conditions := []status.Condition{
			{
				Type:    status.ConditionSucceeded,
				Status:  metav1.ConditionFalse,
				Reason:  "test reason",
				Message: "test message",
			},
		}
		conditionObj := status.NewUnstructuredAdapter[*test.CustomObject](testObject)
		conditionObj.SetConditions(conditions)
		c, found, err := unstructured.NestedSlice(conditionObj.Object, "status", "conditions")
		Expect(err).To(BeNil())
		Expect(found).To(BeTrue())
		Expect(len(c)).To(BeEquivalentTo(1))
		Expect(conditionObj.StatusConditions().Get(status.ConditionSucceeded).Message).To(Equal("test message"))
		Expect(conditionObj.StatusConditions().Get(status.ConditionSucceeded).Status).To(Equal(metav1.ConditionFalse))
		Expect(conditionObj.StatusConditions().Get(status.ConditionSucceeded).Reason).To(Equal("test reason"))
		Expect(conditionObj.StatusConditions().Get(status.ConditionSucceeded).Type).To(Equal(status.ConditionSucceeded))
		Expect(conditionObj.StatusConditions().Get(status.ConditionSucceeded).ObservedGeneration).To(Equal(int64(0)))
	})
})
