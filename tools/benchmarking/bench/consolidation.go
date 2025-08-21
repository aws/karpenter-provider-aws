package bench

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	v1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HalfScaleTestSuite implements a test that scales the deployment to half the original replicas
type ConsolidationTestSuite struct {
	Defintion
}

// Description returns a human-readable description of the test suite
func (s *ConsolidationTestSuite) Description() string {
	return "scale down to half the original pods"
}

// Run executes the test suite
func (s *ConsolidationTestSuite) Run(ctx context.Context, kubeClient client.Client, deployment *v1.Deployment) error {
	// Calculate half the replicas (minimum 1)
	halfReplicas := s.Replicas / 2
	if halfReplicas < 1 {
		halfReplicas = 1
	}
	fmt.Printf("Scaling deployment down to %d replicas...\n", halfReplicas)

	// Get the deployment
	stored := deployment.DeepCopy()

	// Update the replicas
	deployment.Spec.Replicas = lo.ToPtr(halfReplicas)

	// Update the deployment
	if err := kubeClient.Patch(ctx, deployment, client.StrategicMergeFrom(stored)); err != nil {
		return fmt.Errorf("error scaling down deployment: %v", err)
	}

	return nil
}
