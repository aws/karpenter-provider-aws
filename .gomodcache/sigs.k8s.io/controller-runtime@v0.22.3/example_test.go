/*
Copyright 2018 The Kubernetes Authors.

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

package controllerruntime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	// since we invoke tests with -ginkgo.junit-report we need to import ginkgo.
	_ "github.com/onsi/ginkgo/v2"
)

// This example creates a simple application Controller that is configured for ReplicaSets and Pods.
//
// * Create a new application for ReplicaSets that manages Pods owned by the ReplicaSet and calls into
// ReplicaSetReconciler.
//
// * Start the application.
func Example() {
	log := ctrl.Log.WithName("builder-examples")

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	err = ctrl.
		NewControllerManagedBy(manager). // Create the Controller
		For(&appsv1.ReplicaSet{}).       // ReplicaSet is the Application API
		Owns(&corev1.Pod{}).             // ReplicaSet owns Pods created by it
		Complete(&ReplicaSetReconciler{Client: manager.GetClient()})
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}

type ExampleCRDWithConfigMapRef struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	ConfigMapRef      corev1.LocalObjectReference `json:"configMapRef"`
}

func deepCopyObject(arg any) runtime.Object {
	// DO NOT use this code in production code, this is only for presentation purposes.
	// in real code you should generate DeepCopy methods by using controller-gen CLI tool.
	argBytes, err := json.Marshal(arg)
	if err != nil {
		panic(err)
	}
	out := &ExampleCRDWithConfigMapRefList{}
	if err := json.Unmarshal(argBytes, out); err != nil {
		panic(err)
	}
	return out
}

// DeepCopyObject implements client.Object.
func (in *ExampleCRDWithConfigMapRef) DeepCopyObject() runtime.Object {
	return deepCopyObject(in)
}

type ExampleCRDWithConfigMapRefList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExampleCRDWithConfigMapRef `json:"items"`
}

// DeepCopyObject implements client.ObjectList.
func (in *ExampleCRDWithConfigMapRefList) DeepCopyObject() runtime.Object {
	return deepCopyObject(in)
}

// This example creates a simple application Controller that is configured for ExampleCRDWithConfigMapRef CRD.
// Any change in the configMap referenced in this Custom Resource will cause the re-reconcile of the parent ExampleCRDWithConfigMapRef
// due to the implementation of the .Watches method of "sigs.k8s.io/controller-runtime/pkg/builder".Builder.
func Example_customHandler() {
	log := ctrl.Log.WithName("builder-examples")

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	err = ctrl.
		NewControllerManagedBy(manager).
		For(&ExampleCRDWithConfigMapRef{}).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, cm client.Object) []ctrl.Request {
			// map a change from referenced configMap to ExampleCRDWithConfigMapRef, which causes its re-reconcile
			crList := &ExampleCRDWithConfigMapRefList{}
			if err := manager.GetClient().List(ctx, crList); err != nil {
				manager.GetLogger().Error(err, "while listing ExampleCRDWithConfigMapRefs")
				return nil
			}

			reqs := make([]ctrl.Request, 0, len(crList.Items))
			for _, item := range crList.Items {
				if item.ConfigMapRef.Name == cm.GetName() {
					reqs = append(reqs, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Namespace: item.GetNamespace(),
							Name:      item.GetName(),
						},
					})
				}
			}

			return reqs
		})).
		Complete(reconcile.Func(func(ctx context.Context, r reconcile.Request) (reconcile.Result, error) {
			// Your business logic to implement the API by creating, updating, deleting objects goes here.
			return reconcile.Result{}, nil
		}))
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}

// This example creates a simple application Controller that is configured for ReplicaSets and Pods.
// This application controller will be running leader election with the provided configuration in the manager options.
// If leader election configuration is not provided, controller runs leader election with default values.
// Default values taken from: https://github.com/kubernetes/component-base/blob/master/config/v1alpha1/defaults.go
// * defaultLeaseDuration = 15 * time.Second
// * defaultRenewDeadline = 10 * time.Second
// * defaultRetryPeriod   = 2 * time.Second
//
// * Create a new application for ReplicaSets that manages Pods owned by the ReplicaSet and calls into
// ReplicaSetReconciler.
//
// * Start the application.
func Example_updateLeaderElectionDurations() {
	log := ctrl.Log.WithName("builder-examples")
	leaseDuration := 100 * time.Second
	renewDeadline := 80 * time.Second
	retryPeriod := 20 * time.Second
	manager, err := ctrl.NewManager(
		ctrl.GetConfigOrDie(),
		ctrl.Options{
			LeaseDuration: &leaseDuration,
			RenewDeadline: &renewDeadline,
			RetryPeriod:   &retryPeriod,
		})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	err = ctrl.
		NewControllerManagedBy(manager). // Create the Controller
		For(&appsv1.ReplicaSet{}).       // ReplicaSet is the Application API
		Owns(&corev1.Pod{}).             // ReplicaSet owns Pods created by it
		Complete(&ReplicaSetReconciler{Client: manager.GetClient()})
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}

// ReplicaSetReconciler is a simple Controller example implementation.
type ReplicaSetReconciler struct {
	client.Client
}

// Implement the business logic:
// This function will be called when there is a change to a ReplicaSet or a Pod with an OwnerReference
// to a ReplicaSet.
//
// * Read the ReplicaSet
// * Read the Pods
// * Set a Label on the ReplicaSet with the Pod count.
func (a *ReplicaSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Read the ReplicaSet
	rs := &appsv1.ReplicaSet{}
	err := a.Get(ctx, req.NamespacedName, rs)
	if err != nil {
		return ctrl.Result{}, err
	}

	// List the Pods matching the PodTemplate Labels
	pods := &corev1.PodList{}
	err = a.List(ctx, pods, client.InNamespace(req.Namespace), client.MatchingLabels(rs.Spec.Template.Labels))
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the ReplicaSet
	rs.Labels["pod-count"] = fmt.Sprintf("%v", len(pods.Items))
	err = a.Update(ctx, rs)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
