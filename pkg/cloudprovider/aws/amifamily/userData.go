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
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type UserDataProvider struct {
	clientSet       *kubernetes.Clientset
	configMapLister v1.ConfigMapLister
}

// New constructs a new UserData provider
func NewUserDataProvider(clientSet *kubernetes.Clientset) *UserDataProvider {
	factory := informers.NewSharedInformerFactory(clientSet, 1*time.Minute)
	configMapLister := factory.Core().V1().ConfigMaps().Lister()
	factory.Start(wait.NeverStop)
	factory.WaitForCacheSync(wait.NeverStop)
	return &UserDataProvider{
		clientSet:       clientSet,
		configMapLister: configMapLister,
	}
}

// Get returns the UserData from the ConfigMap specified in the provider
func (u *UserDataProvider) Get(ctx context.Context, provider *v1alpha1.AWS) (string, error) {
	if provider.UserData == nil {
		return "", nil
	}
	userDataConfigMap := provider.UserData.ConfigMap
	configMap, err := u.configMapLister.ConfigMaps(*userDataConfigMap.Namespace).Get(*userDataConfigMap.Name)
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
