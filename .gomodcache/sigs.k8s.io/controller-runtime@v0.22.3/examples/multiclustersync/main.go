package main

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

const (
	sourceNamespace = "namespace-to-sync-all-secrets-from"
	targetNamespace = "namespace-to-sync-all-secrets-to"
)

func run() error {
	log.SetLogger(zap.New())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		return fmt.Errorf("failed to construct manager: %w", err)
	}

	allTargets := map[string]cluster.Cluster{}

	cluster, err := cluster.New(ctrl.GetConfigOrDie())
	if err != nil {
		return fmt.Errorf("failed to construct clusters: %w", err)
	}
	if err := mgr.Add(cluster); err != nil {
		return fmt.Errorf("failed to add cluster to manager: %w", err)
	}

	// Add more target clusters here as needed
	allTargets["self"] = cluster

	b := builder.TypedControllerManagedBy[request](mgr).
		Named("secret-sync").
		// Watch secrets in the source namespace of the source cluster and
		// create requests for each target cluster
		WatchesRawSource(source.TypedKind(
			mgr.GetCache(),
			&corev1.Secret{},
			handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, s *corev1.Secret) []request {
				if s.Namespace != sourceNamespace {
					return nil
				}

				result := make([]request, 0, len(allTargets))
				for targetCluster := range allTargets {
					result = append(result, request{
						NamespacedName: types.NamespacedName{Namespace: s.Namespace, Name: s.Name},
						clusterName:    targetCluster,
					})
				}

				return result
			}),
		)).
		WithOptions(controller.TypedOptions[request]{MaxConcurrentReconciles: 10})

	for targetClusterName, targetCluster := range allTargets {
		// Watch secrets in the target namespace of each target cluster
		// and create a request for itself.
		b = b.WatchesRawSource(source.TypedKind(
			targetCluster.GetCache(),
			&corev1.Secret{},
			handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, s *corev1.Secret) []request {
				if s.Namespace != targetNamespace {
					return nil
				}

				return []request{{
					NamespacedName: types.NamespacedName{Namespace: sourceNamespace, Name: s.Name},
					clusterName:    targetClusterName,
				}}
			}),
		))
	}

	clients := make(map[string]client.Client, len(allTargets))
	for targetClusterName, targetCluster := range allTargets {
		clients[targetClusterName] = targetCluster.GetClient()
	}

	if err := b.Complete(&secretSyncReconciler{
		source:  mgr.GetClient(),
		targets: clients,
	}); err != nil {
		return fmt.Errorf("failed to build reconciler: %w", err)
	}

	ctx := signals.SetupSignalHandler()
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	return nil
}

type request struct {
	types.NamespacedName
	clusterName string
}

// secretSyncReconciler is a simple reconciler that keeps all secrets in the source namespace of a given
// source cluster in sync with the secrets in the target namespace of all target clusters.
type secretSyncReconciler struct {
	source  client.Client
	targets map[string]client.Client
}

func (s *secretSyncReconciler) Reconcile(ctx context.Context, req request) (reconcile.Result, error) {
	targetClient, found := s.targets[req.clusterName]
	if !found {
		return reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("target cluster %s not found", req.clusterName))
	}

	var reference corev1.Secret
	if err := s.source.Get(ctx, req.NamespacedName, &reference); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed to get secret %s from reference cluster: %w", req.String(), err)
		}
		if err := targetClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: targetNamespace,
		}}); err != nil {
			if !apierrors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("failed to delete secret %s/%s in cluster %s: %w", targetNamespace, req.Name, req.clusterName, err)
			}

			return reconcile.Result{}, nil
		}

		log.FromContext(ctx).Info("Deleted secret", "cluster", req.clusterName, "namespace", targetNamespace, "name", req.Name)
		return reconcile.Result{}, nil
	}

	target := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      reference.Name,
		Namespace: targetNamespace,
	}}
	result, err := controllerutil.CreateOrUpdate(ctx, targetClient, target, func() error {
		target.Data = reference.Data
		return nil
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to upsert target secret %s/%s: %w", target.Namespace, target.Name, err)
	}

	if result != controllerutil.OperationResultNone {
		log.FromContext(ctx).Info("Upserted secret", "cluster", req.clusterName, "namespace", targetNamespace, "name", req.Name, "result", result)
	}

	return reconcile.Result{}, nil
}
