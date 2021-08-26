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
