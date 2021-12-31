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

package test

import (
	"fmt"

	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PersistentVolumeOptions struct {
	metav1.ObjectMeta
}

func PersistentVolume(overrides ...PersistentVolumeOptions) *v1.PersistentVolume {
	options := PersistentVolumeClaimOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge options: %s", err.Error()))
		}
	}
	return &v1.PersistentVolume{
		ObjectMeta: ObjectMeta(metav1.ObjectMeta{}),
		Spec: v1.PersistentVolumeSpec{},
	}
}

type PersistentVolumeClaimOptions struct {
	metav1.ObjectMeta
}

func PersistentVolumeClaim(overrides ...PersistentVolumeClaimOptions) *v1.PersistentVolumeClaim {
	options := PersistentVolumeClaimOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge options: %s", err.Error()))
		}
	}
	return &v1.PersistentVolumeClaim{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources:   v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("1Gi")}},
		},
	}
}

type StorageClassOptions struct {
	metav1.ObjectMeta
}

func StorageClass(overrides ...StorageClassOptions) *storagev1.StorageClass {
	options := PersistentVolumeClaimOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge options: %s", err.Error()))
		}
	}
	return &storagev1.StorageClass{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
	}
}
