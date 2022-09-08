package node

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/utils/pod"
)

// GetNodePods gets the list of schedulable pods from a node based on nodeName
func GetNodePods(ctx context.Context, kubeClient client.Client, nodeName string) ([]*v1.Pod, error) {
	var podList v1.PodList
	if err := kubeClient.List(ctx, &podList, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return nil, fmt.Errorf("listing pods, %w", err)
	}
	var pods []*v1.Pod
	for i := range podList.Items {
		// these pods don't need to be rescheduled
		if pod.IsOwnedByNode(&podList.Items[i]) ||
			pod.IsOwnedByDaemonSet(&podList.Items[i]) ||
			pod.IsTerminal(&podList.Items[i]) {
			continue
		}
		pods = append(pods, &podList.Items[i])
	}
	return pods, nil
}
