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
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/imdario/mergo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

const DiscoveryLabel = "testing/cluster"

var (
	sequentialNumber     = 0
	randomizer           = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint
	sequentialNumberLock = new(sync.Mutex)
)

func RandomName() string {
	sequentialNumberLock.Lock()
	defer sequentialNumberLock.Unlock()
	sequentialNumber++
	return strings.ToLower(fmt.Sprintf("%s-%d-%s", randomdata.SillyName(), sequentialNumber, randomdata.Alphanumeric(10)))
}

func NamespacedObjectMeta(overrides ...metav1.ObjectMeta) metav1.ObjectMeta {
	return MustMerge(ObjectMeta(metav1.ObjectMeta{
		Namespace: "default",
	}), overrides...)
}

func ObjectMeta(overrides ...metav1.ObjectMeta) metav1.ObjectMeta {
	om := MustMerge(metav1.ObjectMeta{
		Name:   RandomName(),
		Labels: map[string]string{DiscoveryLabel: "unspecified"}, // For cleanup discovery
	}, overrides...)
	om.CreationTimestamp = metav1.Now()
	om.Generation = 1
	return om
}

func TemplateObjectMeta(overrides ...v1.ObjectMeta) v1.ObjectMeta {
	return MustMerge(v1.ObjectMeta{
		Labels: map[string]string{DiscoveryLabel: "unspecified"}, // For cleanup discovery
	}, overrides...)
}

func MustMerge[T interface{}](dest T, srcs ...T) T {
	for _, src := range srcs {
		if err := mergo.Merge(&dest, src, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge object: %s", err))
		}
	}
	return dest
}

func RandomProviderID() string {
	return ProviderID(randomdata.Alphanumeric(17))
}

func ProviderID(base string) string {
	return fmt.Sprintf("fake:///%s", base)
}
