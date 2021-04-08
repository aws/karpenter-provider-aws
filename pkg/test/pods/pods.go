package pods

import (
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Options customizes a Pod.
type Options struct {
	Name             string
	Namespace        string
	Image            string
	NodeName         string
	ResourceRequests v1.ResourceList
	NodeSelector     map[string]string
	Tolerations      []v1.Toleration
	Conditions       []v1.PodCondition
}

func defaults(options Options) *v1.Pod {
	if options.Name == "" {
		options.Name = strings.ToLower(randomdata.SillyName())
	}
	if options.Namespace == "" {
		options.Namespace = "default"
	}
	if options.Image == "" {
		options.Image = "k8s.gcr.io/pause"
	}
	if len(options.Conditions) < 1 {
		options.Conditions = []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}}
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: options.Namespace,
		},
		Spec: v1.PodSpec{
			NodeSelector: options.NodeSelector,
			Tolerations:  options.Tolerations,
			Containers: []v1.Container{{
				Name:  options.Name,
				Image: options.Image,
				Resources: v1.ResourceRequirements{
					Requests: options.ResourceRequests,
				},
			}},
			NodeName: options.NodeName,
		},
		Status: v1.PodStatus{Conditions: options.Conditions},
	}
}

// Pending creates a pending test pod with the minimal set of other
// fields defaulted to something sane.
func Pending() *v1.Pod {
	return defaults(Options{})
}

// PendingWith creates a pending test pod with fields overridden by
// options.
func PendingWith(options Options) *v1.Pod {
	return With(Pending(), options)
}

// With overrides, in-place, pod with any non-zero elements of
// options. It returns the same pod simply for ease of use.
func With(pod *v1.Pod, options Options) *v1.Pod {
	if err := mergo.Merge(pod, defaults(options), mergo.WithOverride); err != nil {
		panic(fmt.Sprintf("unexpected error in test code: %v", err))
	}
	return pod
}
