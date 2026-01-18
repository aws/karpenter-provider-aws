package interceptor

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("NewClient", func() {
	wrappedClient := dummyClient{}
	It("should call the provided Get function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				called = true
				return nil
			},
		})
		_ = client.Get(ctx, types.NamespacedName{}, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Get function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.Get(ctx, types.NamespacedName{}, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided List function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				called = true
				return nil
			},
		})
		_ = client.List(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided List function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.List(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Apply function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			Apply: func(ctx context.Context, client client.WithWatch, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
				called = true
				return nil
			},
		})
		_ = client.Apply(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Apply function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			Apply: func(ctx context.Context, client client.WithWatch, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.Apply(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Create function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				called = true
				return nil
			},
		})
		_ = client.Create(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Create function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.Create(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Delete function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				called = true
				return nil
			},
		})
		_ = client.Delete(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Delete function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.Delete(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided DeleteAllOf function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			DeleteAllOf: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteAllOfOption) error {
				called = true
				return nil
			},
		})
		_ = client.DeleteAllOf(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided DeleteAllOf function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			DeleteAllOf: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteAllOfOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.DeleteAllOf(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Update function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				called = true
				return nil
			},
		})
		_ = client.Update(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Update function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.Update(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Patch function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				called = true
				return nil
			},
		})
		_ = client.Patch(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Patch function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				called = true
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.Patch(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Watch function", func(ctx SpecContext) {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			Watch: func(ctx context.Context, client client.WithWatch, obj client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
				called = true
				return nil, nil
			},
		})
		_, _ = client.Watch(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Watch function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(wrappedClient, Funcs{
			Watch: func(ctx context.Context, client client.WithWatch, obj client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
				called = true
				return nil, nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_, _ = client2.Watch(ctx, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided SubResource function", func() {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			SubResource: func(client client.WithWatch, subResource string) client.SubResourceClient {
				called = true
				return nil
			},
		})
		_ = client.SubResource("")
		Expect(called).To(BeTrue())
	})
	It("should call the provided SubResource function with 'status' when calling Status()", func() {
		var called bool
		client := NewClient(wrappedClient, Funcs{
			SubResource: func(client client.WithWatch, subResource string) client.SubResourceClient {
				if subResource == "status" {
					called = true
				}
				return nil
			},
		})
		_ = client.Status()
		Expect(called).To(BeTrue())
	})
})

var _ = Describe("NewSubResourceClient", func() {
	c := dummyClient{}
	It("should call the provided Get function", func(ctx SpecContext) {
		var called bool
		c := NewClient(c, Funcs{
			SubResourceGet: func(_ context.Context, client client.Client, subResourceName string, obj, subResource client.Object, opts ...client.SubResourceGetOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		_ = c.SubResource("foo").Get(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Get function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(c, Funcs{
			SubResourceGet: func(_ context.Context, client client.Client, subResourceName string, obj, subResource client.Object, opts ...client.SubResourceGetOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.SubResource("foo").Get(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Update function", func(ctx SpecContext) {
		var called bool
		client := NewClient(c, Funcs{
			SubResourceUpdate: func(_ context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		_ = client.SubResource("foo").Update(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Update function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(c, Funcs{
			SubResourceUpdate: func(_ context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.SubResource("foo").Update(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Patch function", func(ctx SpecContext) {
		var called bool
		client := NewClient(c, Funcs{
			SubResourcePatch: func(_ context.Context, client client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		_ = client.SubResource("foo").Patch(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Patch function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(c, Funcs{
			SubResourcePatch: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.SubResource("foo").Patch(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the provided Create function", func(ctx SpecContext) {
		var called bool
		client := NewClient(c, Funcs{
			SubResourceCreate: func(_ context.Context, client client.Client, subResourceName string, obj, subResource client.Object, opts ...client.SubResourceCreateOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		_ = client.SubResource("foo").Create(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
	It("should call the underlying client if the provided Create function is nil", func(ctx SpecContext) {
		var called bool
		client1 := NewClient(c, Funcs{
			SubResourceCreate: func(_ context.Context, client client.Client, subResourceName string, obj, subResource client.Object, opts ...client.SubResourceCreateOption) error {
				called = true
				Expect(subResourceName).To(BeEquivalentTo("foo"))
				return nil
			},
		})
		client2 := NewClient(client1, Funcs{})
		_ = client2.SubResource("foo").Create(ctx, nil, nil)
		Expect(called).To(BeTrue())
	})
})

type dummyClient struct{}

var _ client.WithWatch = &dummyClient{}

func (d dummyClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

func (d dummyClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func (d dummyClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}

func (d dummyClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (d dummyClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (d dummyClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (d dummyClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return nil
}

func (d dummyClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (d dummyClient) Status() client.SubResourceWriter {
	return d.SubResource("status")
}

func (d dummyClient) SubResource(subResource string) client.SubResourceClient {
	return nil
}

func (d dummyClient) Scheme() *runtime.Scheme {
	return nil
}

func (d dummyClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (d dummyClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (d dummyClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, nil
}

func (d dummyClient) Watch(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
	return nil, nil
}
