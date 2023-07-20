package kwok_test

import (
	"context"
	"testing"

	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/cmd/controller-kwok/kwok"
	"github.com/aws/karpenter/pkg/apis/settings"
)

func TestCloudProviderDescribeInstanceTypes(t *testing.T) {
	env := test.NewEnvironment(scheme.Scheme)
	cp := kwok.NewCloudProvider(env.KubernetesInterface)

	ctx := settings.ToContext(context.Background(), &settings.Settings{
		ClusterName:                "clustername",
		ClusterEndpoint:            "clusterendpoint",
		DefaultInstanceProfile:     "profilename",
		EnablePodENI:               false,
		EnableENILimitedPodDensity: false,
		IsolatedVPC:                false,
		VMMemoryOverheadPercent:    0,
		InterruptionQueueName:      "",
		Tags:                       nil,
		ReservedENIs:               0,
	})
	prov := test.Provisioner()
	instanceTypes, err := cp.GetInstanceTypes(ctx, prov)
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	if len(instanceTypes) == 0 {
		t.Errorf("expected > 0 instance types")
	}
}
