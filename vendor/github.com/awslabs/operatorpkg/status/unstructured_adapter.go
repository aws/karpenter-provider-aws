package status

import (
	"encoding/json"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// unstructuredAdapter is an adapter for the status.Object interface. unstructuredAdapter
// makes the assumption that status conditions are found on status.conditions path.
type unstructuredAdapter[T client.Object] struct {
	unstructured.Unstructured
}

func NewUnstructuredAdapter[T client.Object](obj client.Object) *unstructuredAdapter[T] {
	u := unstructured.Unstructured{Object: lo.Must(runtime.DefaultUnstructuredConverter.ToUnstructured(obj))}
	ua := &unstructuredAdapter[T]{Unstructured: u}
	ua.SetGroupVersionKind(object.GVK(obj))
	return ua
}

func (u *unstructuredAdapter[T]) GetObjectKind() schema.ObjectKind {
	return u
}
func (u *unstructuredAdapter[T]) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	u.Unstructured.SetGroupVersionKind(gvk)
}
func (u *unstructuredAdapter[T]) GroupVersionKind() schema.GroupVersionKind {
	return object.GVK(object.New[T]())
}

func (u *unstructuredAdapter[T]) GetConditions() []Condition {
	conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	return lo.Map(conditions, func(condition interface{}, _ int) Condition {
		var newCondition Condition
		cond := condition.(map[string]interface{})
		jsonStr, _ := json.Marshal(cond)
		json.Unmarshal(jsonStr, &newCondition)
		return newCondition
	})
}
func (u *unstructuredAdapter[T]) SetConditions(conditions []Condition) {
	unstructured.SetNestedSlice(u.Object, lo.Map(conditions, func(condition Condition, _ int) interface{} {
		var b map[string]interface{}
		j, _ := json.Marshal(&condition)
		json.Unmarshal(j, &b)
		return b
	}), "status", "conditions")
}

func (u *unstructuredAdapter[T]) StatusConditions() ConditionSet {
	conditionTypes := lo.Map(u.GetConditions(), func(condition Condition, _ int) string {
		return condition.Type
	})
	return NewReadyConditions(conditionTypes...).For(u)
}
