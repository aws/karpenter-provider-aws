package bench

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	v1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EmptinessTestSuite implements a test that scales the deployment to zero
type EmptinessTestSuite struct {
	Defintion
}

// Description returns a human-readable description of the test suite
func (s *EmptinessTestSuite) Description() string {
	return "scale down to 0 pods"
}

// Run executes the test suite
func (e *EmptinessTestSuite) Run(ctx context.Context, kubeClient client.Client, deployment *v1.Deployment) error {
	fmt.Println("Scaling deployment down to 0 replicas...")

	// Get the deployment
	// Get the deployment
	stored := deployment.DeepCopy()

	// Update the replicas
	deployment.Spec.Replicas = lo.ToPtr(int32(0))

	// Update the deployment
	if err := kubeClient.Patch(ctx, deployment, client.StrategicMergeFrom(stored)); err != nil {
		return fmt.Errorf("error scaling down deployment: %v", err)
	}

	return nil
}
