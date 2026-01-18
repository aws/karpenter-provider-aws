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

package client_test

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	c           client.Client
	someIndexer client.FieldIndexer
)

func ExampleNew() {
	cl, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	podList := &corev1.PodList{}

	err = cl.List(context.Background(), podList, client.InNamespace("default"))
	if err != nil {
		fmt.Printf("failed to list pods in namespace default: %v\n", err)
		os.Exit(1)
	}
}

func ExampleNew_suppress_warnings() {
	cfg := config.GetConfigOrDie()
	// Use a rest.WarningHandlerWithContext that discards warning messages.
	cfg.WarningHandlerWithContext = rest.NoWarnings{}

	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	podList := &corev1.PodList{}

	err = cl.List(context.Background(), podList, client.InNamespace("default"))
	if err != nil {
		fmt.Printf("failed to list pods in namespace default: %v\n", err)
		os.Exit(1)
	}
}

// This example shows how to use the client with typed and unstructured objects to retrieve an object.
func ExampleClient_get() {
	// Using a typed object.
	pod := &corev1.Pod{}
	// c is a created client.
	_ = c.Get(context.Background(), client.ObjectKey{
		Namespace: "namespace",
		Name:      "name",
	}, pod)

	// Using a unstructured object.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	})
	_ = c.Get(context.Background(), client.ObjectKey{
		Namespace: "namespace",
		Name:      "name",
	}, u)
}

// This example shows how to use the client with typed and unstructured objects to create objects.
func ExampleClient_create() {
	// Using a typed object.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "name",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "nginx",
					Name:  "nginx",
				},
			},
		},
	}
	// c is a created client.
	_ = c.Create(context.Background(), pod)

	// Using a unstructured object.
	u := &unstructured.Unstructured{}
	u.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "name",
			"namespace": "namespace",
		},
		"spec": map[string]interface{}{
			"replicas": 2,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"foo": "bar",
				},
			},
			"template": map[string]interface{}{
				"labels": map[string]interface{}{
					"foo": "bar",
				},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":  "nginx",
							"image": "nginx",
						},
					},
				},
			},
		},
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	})
	_ = c.Create(context.Background(), u)
}

// This example shows how to use the client with typed and unstructured objects to list objects.
func ExampleClient_list() {
	// Using a typed object.
	pod := &corev1.PodList{}
	// c is a created client.
	_ = c.List(context.Background(), pod)

	// Using a unstructured object.
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "DeploymentList",
		Version: "v1",
	})
	_ = c.List(context.Background(), u)
}

// This example shows how to use the client with typed and unstructured objects to update objects.
func ExampleClient_update() {
	// Using a typed object.
	pod := &corev1.Pod{}
	// c is a created client.
	_ = c.Get(context.Background(), client.ObjectKey{
		Namespace: "namespace",
		Name:      "name",
	}, pod)
	controllerutil.AddFinalizer(pod, "new-finalizer")
	_ = c.Update(context.Background(), pod)

	// Using a unstructured object.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	})
	_ = c.Get(context.Background(), client.ObjectKey{
		Namespace: "namespace",
		Name:      "name",
	}, u)
	controllerutil.AddFinalizer(u, "new-finalizer")
	_ = c.Update(context.Background(), u)
}

// This example shows how to use the client with typed and unstructured objects to patch objects.
func ExampleClient_patch() {
	patch := []byte(`{"metadata":{"annotations":{"version": "v2"}}}`)
	_ = c.Patch(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "name",
		},
	}, client.RawPatch(types.StrategicMergePatchType, patch))
}

// This example shows how to use the client with unstructured objects to create/patch objects using Server Side Apply,
// "k8s.io/apimachinery/pkg/runtime".DefaultUnstructuredConverter.ToUnstructured is used to convert an object into map[string]any representation,
// which is then set as an "Object" field in *unstructured.Unstructured struct, which implements client.Object.
func ExampleClient_apply() {
	// Using a typed object.
	configMap := corev1ac.ConfigMap("name", "namespace").WithData(map[string]string{"key": "value"})
	// c is a created client.
	u := &unstructured.Unstructured{}
	u.Object, _ = runtime.DefaultUnstructuredConverter.ToUnstructured(configMap)
	_ = c.Patch(context.Background(), u, client.Apply, client.ForceOwnership, client.FieldOwner("field-owner"))
}

// This example shows how to use the client with typed and unstructured objects to patch objects' status.
func ExampleClient_patchStatus() {
	u := &unstructured.Unstructured{}
	u.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "foo",
			"namespace": "namespace",
		},
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "batch",
		Version: "v1beta1",
		Kind:    "CronJob",
	})
	patch := []byte(fmt.Sprintf(`{"status":{"lastScheduleTime":"%s"}}`, time.Now().Format(time.RFC3339)))
	_ = c.Status().Patch(context.Background(), u, client.RawPatch(types.MergePatchType, patch))
}

// This example shows how to use the client with typed and unstructured objects to delete objects.
func ExampleClient_delete() {
	// Using a typed object.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "name",
		},
	}
	// c is a created client.
	_ = c.Delete(context.Background(), pod)

	// Using a unstructured object.
	u := &unstructured.Unstructured{}
	u.SetName("name")
	u.SetNamespace("namespace")
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	})
	_ = c.Delete(context.Background(), u)
}

// This example shows how to use the client with typed and unstructured objects to delete collections of objects.
func ExampleClient_deleteAllOf() {
	// Using a typed object.
	// c is a created client.
	_ = c.DeleteAllOf(context.Background(), &corev1.Pod{}, client.InNamespace("foo"), client.MatchingLabels{"app": "foo"})

	// Using an unstructured Object
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	})
	_ = c.DeleteAllOf(context.Background(), u, client.InNamespace("foo"), client.MatchingLabels{"app": "foo"})
}

// This example shows how to set up and consume a field selector over a pod's volumes' secretName field.
func ExampleFieldIndexer_secretNameNode() {
	// someIndexer is a FieldIndexer over a Cache
	_ = someIndexer.IndexField(context.TODO(), &corev1.Pod{}, "spec.volumes.secret.secretName", func(o client.Object) []string {
		var res []string
		for _, vol := range o.(*corev1.Pod).Spec.Volumes {
			if vol.Secret == nil {
				continue
			}
			// just return the raw field value -- the indexer will take care of dealing with namespaces for us
			res = append(res, vol.Secret.SecretName)
		}
		return res
	})

	_ = someIndexer.IndexField(context.TODO(), &corev1.Pod{}, "spec.NodeName", func(o client.Object) []string {
		nodeName := o.(*corev1.Pod).Spec.NodeName
		if nodeName != "" {
			return []string{nodeName}
		}
		return nil
	})

	// elsewhere (e.g. in your reconciler)
	mySecretName := "someSecret" // derived from the reconcile.Request, for instance
	myNode := "master-0"
	var podsWithSecrets corev1.PodList
	_ = c.List(context.Background(), &podsWithSecrets, client.MatchingFields{
		"spec.volumes.secret.secretName": mySecretName,
		"spec.NodeName":                  myNode,
	})
}
