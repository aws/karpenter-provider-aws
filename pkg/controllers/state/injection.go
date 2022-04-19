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

package state

import "context"

// These methods can't exist in the injection package as it's imported into the webhook indirectly.   If they're imported
// into the webhook, we run into the duplicate kubeflag command line argument issue.
type clusterKeyType struct{}

var clusterKey clusterKeyType

func WithClusterState(ctx context.Context, cs *Cluster) context.Context {
	return context.WithValue(ctx, clusterKey, cs)
}

func GetClusterState(ctx context.Context) *Cluster {
	cs := ctx.Value(clusterKey)
	if cs == nil {
		return nil
	}
	return cs.(*Cluster)
}
