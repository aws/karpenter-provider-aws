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

package filesystem

import (
	"context"

	"github.com/spf13/afero"
)

type contextKey string

const (
	fsKey contextKey = "karpenter.sh/fs"
)

// Inject adds an fs.ReadFileFS that should be used by tests (only)
// to mock out the filesystem.
func Inject(ctx context.Context, override afero.Fs) context.Context {
	return context.WithValue(ctx, fsKey, override)
}

// For returns an fs.ReadFileFS, normally a pass-through to the host
// filesystem, except in the context of a test.
func For(ctx context.Context) afero.Fs {
	retval := ctx.Value(fsKey)
	if retval == nil {
		// Use afero because io/fs does not allow absolute paths like
		// `/var/something`.
		return afero.NewOsFs()
	}
	return retval.(afero.Fs)
}
