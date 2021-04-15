package apiobject

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PodNamespacedNames(pods []*v1.Pod) []types.NamespacedName {
	namespacedNames := []types.NamespacedName{}
	for _, pod := range pods {
		namespacedNames = append(namespacedNames, NamespacedName(pod))
	}
	return namespacedNames
}

func NamespacedName(o client.Object) types.NamespacedName {
	return types.NamespacedName{Namespace: o.GetNamespace(), Name: o.GetName()}
}
