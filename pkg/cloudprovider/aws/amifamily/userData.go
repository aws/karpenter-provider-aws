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

package amifamily

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type UserDataProvider struct {
	clientSet *kubernetes.Clientset
}

// New constructs a new launch template Resolver
func NewUserDataProvider(clientSet *kubernetes.Clientset) *UserDataProvider {
	return &UserDataProvider{
		clientSet: clientSet,
	}
}

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, accelerator, etc
func (u *UserDataProvider) Get(ctx context.Context, provider *v1alpha1.AWS) (string, error) {
	if provider.UserData == nil {
		return "", nil
	}
	userDataConfigMap := provider.UserData.ConfigMap
	configMap, err := u.clientSet.CoreV1().ConfigMaps(*userDataConfigMap.Namespace).Get(ctx, *userDataConfigMap.Name, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failure retrieving config map %s/%s", *userDataConfigMap.Namespace, *userDataConfigMap.Name)
	}
	if len(configMap.Data) > 1 {
		return "", fmt.Errorf("multiple keys specified in user data config map")
	}
	userDataString := ""
	for _, v := range configMap.Data {
		userDataString = v
	}
	return userDataString, nil
}
