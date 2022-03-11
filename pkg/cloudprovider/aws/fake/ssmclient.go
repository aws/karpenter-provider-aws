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

package fake

import (
	"context"
	"fmt"
)

type SSMClient struct {
	Parameters map[string]string
}

func (c *SSMClient) GetParameterWithContext(ctx context.Context, query string) (string, error) {
	if value, ok := c.Parameters[query]; ok {
		return value, nil
	}
	return "", fmt.Errorf("parameter %s not found", query)
}

func DefaultSSMClient() *SSMClient {
	return &SSMClient{
		Parameters: map[string]string{
			"/aws/service/eks/optimized-ami/1.21/amazon-linux-2-arm64/recommended/image_id":        "ami-002a052abdc5fff1c",
			"/aws/service/eks/optimized-ami/1.21/amazon-linux-2/recommended/image_id":              "ami-015c52b52fe1c5990",
			"/aws/service/canonical/ubuntu/eks/20.04/1.21/stable/current/amd64/hvm/ebs-gp2/ami-id": "ami-03a9a7e59a2817979",
			"/aws/service/canonical/ubuntu/eks/20.04/1.21/stable/current/arm64/hvm/ebs-gp2/ami-id": "ami-07f241bb8b6c4db85",
			"/aws/service/bottlerocket/aws-k8s-1.21/arm64/latest/image_id":                         "ami-07095e4c08d56c3ec",
			"/aws/service/bottlerocket/aws-k8s-1.21/x86_64/latest/image_id":                        "ami-08ae42adf8bb7b8b0",
		},
	}
}
