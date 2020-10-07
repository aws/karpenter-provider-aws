package test

import (
	"strings"

	"github.com/Pallinder/go-randomdata"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Pod(node string, namespace string, resources v1.ResourceList) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(randomdata.SillyName()),
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			NodeName: node,
			Containers: []v1.Container{{
				Name:  "pause",
				Image: "k8s.gcr.io/pause",
				Resources: v1.ResourceRequirements{
					Requests: resources,
				},
			}},
		},
	}
}

func Node(labels map[string]string, resources v1.ResourceList) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   strings.ToLower(randomdata.SillyName()),
			Labels: labels,
		},
		Spec: v1.NodeSpec{},
		Status: v1.NodeStatus{
			Capacity: resources,
		},
	}
}
