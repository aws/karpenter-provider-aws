package cloudprovider

import (
	"context"
	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
)

type AmiDrifter struct {
	provisioner *v1alpha5.Provisioner
	ctx context.Context
	c	*CloudProvider
}

func (ad *AmiDrifter) GetDriftedNodes(nodes []v1.Node) []v1.Node {
	if ad.c == nil || ad.provisioner == nil {
		//check, create panic ?
	}
	provisioner := ad.provisioner
	instanceTypes, _ := ad.c.GetInstanceTypes(context.Background(), provisioner)
	aws, err := ad.c.getProvider(ad.ctx, provisioner.Spec.Provider, provisioner.Spec.ProviderRef)
	if err != nil {
		return []v1.Node{}
	}
	amis, err := ad.c.instanceProvider.launchTemplateProvider.GetAmisForProvisioner(ad.ctx, aws, provisioner.Spec.ProviderRef, instanceTypes)
	if err != nil {
		logging.FromContext(ad.ctx).Errorf("getting drift amis from provisioner %w", err)
		return []v1.Node{}
	}
	if len(amis) == 0 {
		//Ideally this would only happen if the provisioner/awsnodetemplate are wrongly configured, just return with no drift.
		logging.FromContext(ad.ctx).Infof("no amis while calculating drift")
		return []v1.Node{}
	}
	logging.FromContext(ad.ctx).Infof("%v amis found for drift", amis)
	return lo.Filter(nodes, func(node v1.Node, _ int) bool {
		nodeAmi, exists := node.Labels[v1alpha1.LabelInstanceAMIID]
		if nodeAmi == "" || !exists {
			logging.FromContext(ad.ctx).Debugf("No ami id found on node %s, while calculating drift", node.Name)
			return false
		}
		return !lo.Contains(amis, nodeAmi)
	})
}

