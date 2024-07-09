/*
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

package debug

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EventClient struct {
	start      time.Time
	kubeClient client.Client
}

func NewEventClient(kubeClient client.Client) *EventClient {
	return &EventClient{
		start:      time.Now(),
		kubeClient: kubeClient,
	}
}

func (c *EventClient) DumpEvents(ctx context.Context) error {
	return multierr.Combine(
		c.dumpPodEvents(ctx),
		c.dumpNodeEvents(ctx),
	)

}

func (c *EventClient) dumpPodEvents(ctx context.Context) error {
	el := &corev1.EventList{}
	if err := c.kubeClient.List(ctx, el, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{"involvedObject.kind": "Pod"}),
	}); err != nil {
		return err
	}
	events := lo.Filter(filterTestEvents(el.Items, c.start), func(e corev1.Event, _ int) bool {
		return e.InvolvedObject.Namespace != "kube-system"
	})
	for k, v := range coallateEvents(events) {
		fmt.Print(getEventInformation(k, v))
	}
	return nil
}

func (c *EventClient) dumpNodeEvents(ctx context.Context) error {
	el := &corev1.EventList{}
	if err := c.kubeClient.List(ctx, el, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{"involvedObject.kind": "Node"}),
	}); err != nil {
		return err
	}
	for k, v := range coallateEvents(filterTestEvents(el.Items, c.start)) {
		fmt.Print(getEventInformation(k, v))
	}
	return nil
}

func filterTestEvents(events []corev1.Event, startTime time.Time) []corev1.Event {
	return lo.Filter(events, func(e corev1.Event, _ int) bool {
		if !e.EventTime.IsZero() {
			if e.EventTime.BeforeTime(&metav1.Time{Time: startTime}) {
				return false
			}
		} else if e.FirstTimestamp.Before(&metav1.Time{Time: startTime}) {
			return false
		}
		return true
	})
}

func coallateEvents(events []corev1.Event) map[corev1.ObjectReference]*corev1.EventList {
	eventMap := map[corev1.ObjectReference]*corev1.EventList{}
	for i := range events {
		elem := events[i]
		objectKey := corev1.ObjectReference{Kind: elem.InvolvedObject.Kind, Namespace: elem.InvolvedObject.Namespace, Name: elem.InvolvedObject.Name}
		if _, ok := eventMap[objectKey]; !ok {
			eventMap[objectKey] = &corev1.EventList{}
		}
		eventMap[objectKey].Items = append(eventMap[objectKey].Items, elem)
	}
	return eventMap
}

// Partially copied from
// https://github.com/kubernetes/kubernetes/blob/04ee339c7a4d36b4037ce3635993e2a9e395ebf3/staging/src/k8s.io/kubectl/pkg/describe/describe.go#L4232
func getEventInformation(o corev1.ObjectReference, el *corev1.EventList) string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("------- %s/%s%s EVENTS -------\n",
		strings.ToLower(o.Kind), lo.Ternary(o.Namespace != "", o.Namespace+"/", ""), o.Name))
	if len(el.Items) == 0 {
		return sb.String()
	}
	for _, e := range el.Items {
		source := e.Source.Component
		if source == "" {
			source = e.ReportingController
		}
		eventTime := e.EventTime
		if eventTime.IsZero() {
			eventTime = metav1.NewMicroTime(e.FirstTimestamp.Time)
		}
		sb.WriteString(fmt.Sprintf("time=%s type=%s reason=%s from=%s message=%s\n",
			eventTime.Format(time.RFC3339),
			e.Type,
			e.Reason,
			source,
			strings.TrimSpace(e.Message)),
		)
	}
	return sb.String()
}
