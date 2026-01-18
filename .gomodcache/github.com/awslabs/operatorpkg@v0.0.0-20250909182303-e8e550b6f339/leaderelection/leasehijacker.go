package leaderelection

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

/*
LeaseHijacker implements lease stealing to accelerate development workflows.
When starting your controller manager, your local process will forcibly
become leader. This is useful when developing locally against a cluster
which already has a controller running in it

TODO: migrate to https://kubernetes.io/docs/concepts/cluster-administration/coordinated-leader-election/ when it's past alpha.

Include this in your controller manager as follows:
```
controllerruntime.NewManager(..., controllerruntime.Options{

// Used if HIJACK_LEASE is not set
LeaderElectionID:                    name,
LeaderElectionNamespace:             "namespace",

// Used if HIJACK_LEASE=true
LeaderElectionResourceLockInterface: leaderelection.LeaseHijacker(...)
}
```
*/
func LeaseHijacker(ctx context.Context, config *rest.Config, namespace string, name string) resourcelock.Interface {
	if os.Getenv("HIJACK_LEASE") != "true" {
		return nil // If not set, fallback to other controller-runtime lease settings
	}
	kubeClient := coordinationv1client.NewForConfigOrDie(config)
	lease := lo.Must(kubeClient.Leases(namespace).Get(ctx, name, metav1.GetOptions{}))

	untilElection := time.Until(lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second))

	lease.Spec.HolderIdentity = lo.ToPtr(fmt.Sprintf("%s_%s", lo.Must(os.Hostname()), uuid.NewUUID()))
	lease.Spec.AcquireTime = lo.ToPtr(metav1.NowMicro())
	lease.Spec.RenewTime = lo.ToPtr(metav1.NowMicro())
	*lease.Spec.LeaseDurationSeconds += 5 // Make our lease longer to guarantee we win the next election
	*lease.Spec.LeaseTransitions += 1
	lo.Must(kubeClient.Leases(namespace).Update(ctx, lease, metav1.UpdateOptions{}))

	log.FromContext(ctx).Info(fmt.Sprintf("hijacked lease, waiting %s for election", untilElection), "namespace", namespace, "name", name)
	time.Sleep(untilElection)

	return lo.Must(resourcelock.New(
		resourcelock.LeasesResourceLock,
		namespace,
		name,
		corev1client.NewForConfigOrDie(config),
		coordinationv1client.NewForConfigOrDie(config),
		resourcelock.ResourceLockConfig{Identity: *lease.Spec.HolderIdentity},
	))
}
