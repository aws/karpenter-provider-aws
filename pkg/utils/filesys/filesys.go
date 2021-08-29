/*
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

package filesys

import (
	"context"
	"io/fs"

	"github.com/spf13/afero"
)

type contextKey string

const (
	KarpenterFS contextKey = "karpenter.sh/fs"
)

// Inject adds an fs.ReadFileFS that should be used by tests (only)
// to mock out the filesystem.
func Inject(ctx context.Context, override fs.ReadFileFS) context.Context {
	return context.WithValue(ctx, KarpenterFS, override)
}

// For returns an fs.ReadFileFS, normally a pass-through to the host
// filesystem, except in the context of a test.
func For(ctx context.Context) fs.ReadFileFS {
	retval := ctx.Value(KarpenterFS)
	if retval == nil {
		// Use afero because os.DirFS() doesn't resolve aboslute paths
		// anchored at '/' idiomatically.
		return afero.NewIOFS(afero.NewOsFs())
	}
	return retval.(fs.ReadFileFS)
}
