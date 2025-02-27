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

package test

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// CacheSyncingClient exists for tests that need to use custom fieldSelectors (thus, they need a client cache)
// and also need consistency in their testing by waiting for caches to sync after performing WRITE operations
// NOTE: This cache sync doesn't sync with third-party operations on the api-server
type CacheSyncingClient struct {
	client.Client
}

// If we timeout on polling, the assumption is that the cache updated to a newer version
// and we missed the current WRITE operation that we just performed
var pollingOptions = []retry.Option{
	retry.Attempts(100), // This whole poll should take ~1s
	retry.Delay(time.Millisecond * 10),
	retry.DelayType(retry.FixedDelay),
}

func (c *CacheSyncingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if err := c.Client.Create(ctx, obj, opts...); err != nil {
		return err
	}
	_ = retry.Do(func() error {
		if err := c.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return fmt.Errorf("getting object, %w", err)
		}
		return nil
	}, pollingOptions...)
	return nil
}

func (c *CacheSyncingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if err := c.Client.Delete(ctx, obj, opts...); err != nil {
		return err
	}
	_ = retry.Do(func() error {
		if err := c.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("getting object, %w", err)
		}
		return fmt.Errorf("object still exists")
	}, pollingOptions...)
	return nil
}

func (c *CacheSyncingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if err := c.Client.Update(ctx, obj, opts...); err != nil {
		return err
	}
	_ = retry.Do(func() error {
		return objectSynced(ctx, c.Client, obj)
	}, pollingOptions...)
	return nil
}

func (c *CacheSyncingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if err := c.Client.Patch(ctx, obj, patch, opts...); err != nil {
		return err
	}
	_ = retry.Do(func() error {
		return objectSynced(ctx, c.Client, obj)
	}, pollingOptions...)
	return nil
}

func (c *CacheSyncingClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	options := &client.DeleteAllOfOptions{}
	for _, o := range opts {
		o.ApplyToDeleteAllOf(options)
	}
	if err := c.Client.DeleteAllOf(ctx, obj, opts...); err != nil {
		return err
	}
	metaList := &metav1.PartialObjectMetadataList{}
	metaList.SetGroupVersionKind(lo.Must(apiutil.GVKForObject(obj, c.Scheme())))

	_ = retry.Do(func() error {
		listOptions := []client.ListOption{client.Limit(1)}
		if options.ListOptions.Namespace != "" {
			listOptions = append(listOptions, client.InNamespace(options.ListOptions.Namespace))
		}
		if err := c.Client.List(ctx, metaList, listOptions...); err != nil {
			return fmt.Errorf("listing objects, %w", err)
		}
		if len(metaList.Items) != 0 {
			return fmt.Errorf("objects still exist")
		}
		return nil
	}, pollingOptions...)
	return nil
}

func (c *CacheSyncingClient) Status() client.StatusWriter {
	return &cacheSyncingStatusWriter{
		client: c.Client,
	}
}

type cacheSyncingStatusWriter struct {
	client client.Client
}

func (c *cacheSyncingStatusWriter) Create(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
	panic("create on cacheSyncingStatusWriter isn't supported")
}

func (c *cacheSyncingStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if err := c.client.Status().Update(ctx, obj, opts...); err != nil {
		return err
	}
	_ = retry.Do(func() error {
		return objectSynced(ctx, c.client, obj)
	}, pollingOptions...)
	return nil
}

func (c *cacheSyncingStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if err := c.client.Status().Patch(ctx, obj, patch, opts...); err != nil {
		return err
	}
	_ = retry.Do(func() error {
		return objectSynced(ctx, c.client, obj)
	}, pollingOptions...)
	return nil
}

func objectSynced(ctx context.Context, c client.Client, obj client.Object) error {
	temp := obj.DeepCopyObject().(client.Object)
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), temp); err != nil {
		// If the object isn't found, we assume that the cache was synced since the Update operation must have caused
		// the object to get completely removed (like a finalizer update)
		return client.IgnoreNotFound(fmt.Errorf("getting object, %w", err))
	}
	if obj.GetResourceVersion() != temp.GetResourceVersion() {
		return fmt.Errorf("object hasn't updated")
	}
	return nil
}
