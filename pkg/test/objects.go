package test

import (
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Object interface {
	runtime.Object
	metav1.Object
}

type HappyableObject interface {
	Object
	IsHappy() bool
}

func Pod(node string, namespace string, cpu int32, memoryGi int32) *v1.Pod {
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
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", cpu)),
						v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", memoryGi)),
					},
				},
			}},
		},
	}
}

func Node(labels map[string]string, cpu int32, memoryGi int32) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   strings.ToLower(randomdata.SillyName()),
			Labels: labels,
		},
		Spec: v1.NodeSpec{},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", cpu)),
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", memoryGi)),
			},
		},
	}
}
