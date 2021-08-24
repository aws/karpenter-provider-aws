package filesys

import (
	"context"
	"io/fs"
	"os"
)

const (
	KarpenterFS = "karpenter.sh/fs"
)

func Inject(ctx context.Context) context.Context {
	return context.WithValue(ctx, KarpenterFS, os.DirFS("/"))
}

func For(ctx context.Context) fs.FS {
	return ctx.Value(KarpenterFS).(fs.FS)
}
