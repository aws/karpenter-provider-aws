/*
Copyright The Kubernetes Authors.

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

package nodepool

import (
	"context"
	"sort"

	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/option"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

func IsManaged(nodePool *v1.NodePool, cp cloudprovider.CloudProvider) bool {
	return lo.ContainsBy(cp.GetSupportedNodeClasses(), func(nodeClass status.Object) bool {
		return object.GVK(nodeClass).GroupKind() == nodePool.Spec.Template.Spec.NodeClassRef.GroupKind()
	})
}

func IsStatic(np *v1.NodePool) bool {
	return np.Spec.Replicas != nil
}

func GetNodeClass(ctx context.Context, c client.Client, nodePool *v1.NodePool, cp cloudprovider.CloudProvider) (status.Object, error) {
	if nodeClass, ok := lo.Find(cp.GetSupportedNodeClasses(), func(nodeClass status.Object) bool {
		return object.GVK(nodeClass).GroupKind() == nodePool.Spec.Template.Spec.NodeClassRef.GroupKind()
	}); ok {
		if err := c.Get(ctx, client.ObjectKey{Name: nodePool.Spec.Template.Spec.NodeClassRef.Name}, nodeClass); err != nil {
			return nodeClass, err
		}
		return nodeClass, nil
	}
	return nil, nil
}

// IsManagedPredicateFuncs is used to filter controller-runtime NodeClaim watches to NodeClaims managed by the given cloudprovider.
func IsManagedPredicateFuncs(cp cloudprovider.CloudProvider) predicate.Funcs {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return IsManaged(o.(*v1.NodePool), cp)
	})
}

// IsStaticPredicateFunc is used to filter controller-runtime NodePool watches to Static NodePools
func IsStaticPredicateFuncs() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return IsStatic(o.(*v1.NodePool))
	})
}

func ForNodeClass(nc status.Object) client.ListOption {
	return client.MatchingFields{
		"spec.template.spec.nodeClassRef.group": object.GVK(nc).Group,
		"spec.template.spec.nodeClassRef.kind":  object.GVK(nc).Kind,
		"spec.template.spec.nodeClassRef.name":  nc.GetName(),
	}
}

func ListManaged(ctx context.Context, c client.Client, cloudProvider cloudprovider.CloudProvider, opts ...client.ListOption) ([]*v1.NodePool, error) {
	nodePoolList := &v1.NodePoolList{}
	if err := c.List(ctx, nodePoolList, opts...); err != nil {
		return nil, err
	}
	return lo.FilterMap(nodePoolList.Items, func(np v1.NodePool, _ int) (*v1.NodePool, bool) {
		return &np, IsManaged(&np, cloudProvider)
	}), nil
}

type NodeClaimHandlerOption struct {
	staticOnly bool
	client     client.Client // used only if staticOnly && nameFilter == nil
}

func WithStaticOnly(o *NodeClaimHandlerOption) {
	o.staticOnly = true
}

func WithClient(c client.Client) func(*NodeClaimHandlerOption) {
	return func(o *NodeClaimHandlerOption) { o.client = c }
}

func NodeClaimEventHandler(opts ...option.Function[NodeClaimHandlerOption]) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		o := option.Resolve(opts...)

		name, ok := obj.GetLabels()[v1.NodePoolLabelKey]
		if !ok {
			return nil
		}

		if !o.staticOnly {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
		}

		var np v1.NodePool
		if err := o.client.Get(ctx, types.NamespacedName{Name: name}, &np); err != nil {
			return nil
		}

		if !IsStatic(&np) {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
	})
}

func NodeEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		name, ok := o.GetLabels()[v1.NodePoolLabelKey]
		if !ok {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
	})

}

// NodeClassEventHandler is a watcher on v1.NodePool that maps NodeClass to NodePools based
// on the nodeClassRef and enqueues reconcile.Requests for the NodePool
func NodeClassEventHandler(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) (requests []reconcile.Request) {
		nps := &v1.NodePoolList{}
		if err := c.List(ctx, nps, ForNodeClass(o.(status.Object))); err != nil {
			return nil
		}
		return lo.Map(nps.Items, func(np v1.NodePool, _ int) reconcile.Request {
			return reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&np),
			}
		})
	})
}

// OrderByWeight orders the NodePools in the provided slice by their priority weight in-place. This priority evaluates
// the following things in precedence order:
//  1. NodePools that have a larger weight are ordered first
//  2. If two NodePools have the same weight, then the NodePool with the name later in the alphabet will come first
func OrderByWeight(nps []*v1.NodePool) {
	sort.Slice(nps, func(a, b int) bool {
		weightA := lo.FromPtr(nps[a].Spec.Weight)
		weightB := lo.FromPtr(nps[b].Spec.Weight)
		if weightA == weightB {
			// Order NodePools by name for a consistent ordering when sorting equal weight
			return nps[a].Name > nps[b].Name
		}
		return weightA > weightB
	})
}
