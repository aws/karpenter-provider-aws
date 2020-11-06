package node

import v1 "k8s.io/api/core/v1"

func IsReadyAndSchedulable(node v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue && !node.Spec.Unschedulable
		}
	}
	return false
}
