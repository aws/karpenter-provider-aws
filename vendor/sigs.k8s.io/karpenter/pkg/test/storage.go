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

	"github.com/imdario/mergo"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PersistentVolumeOptions struct {
	metav1.ObjectMeta
	Zones              []string
	StorageClassName   string
	Driver             string
	UseAWSInTreeDriver bool
	UseLocal           bool
	UseHostPath        bool
}

func PersistentVolume(overrides ...PersistentVolumeOptions) *v1.PersistentVolume {
	options := PersistentVolumeOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge options: %s", err))
		}
	}
	// Determine the PersistentVolumeSource based on the options
	var source v1.PersistentVolumeSource
	switch {
	case options.UseLocal:
		source = v1.PersistentVolumeSource{
			Local: &v1.LocalVolumeSource{
				Path: "/mnt/local-disks",
			},
		}
	case options.UseHostPath:
		source = v1.PersistentVolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/mnt/local-disks",
			},
		}
	case options.UseAWSInTreeDriver:
		source = v1.PersistentVolumeSource{
			AWSElasticBlockStore: &v1.AWSElasticBlockStoreVolumeSource{
				FSType:   "ext4",
				VolumeID: RandomProviderID(),
			},
		}
	default:
		source = v1.PersistentVolumeSource{
			CSI: &v1.CSIPersistentVolumeSource{
				Driver:       lo.Ternary(options.Driver != "", options.Driver, "test.driver"),
				VolumeHandle: "test-handle",
			},
		}
	}
	var nodeAffinity *v1.VolumeNodeAffinity
	if len(options.Zones) != 0 {
		nodeAffinity = &v1.VolumeNodeAffinity{Required: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{{MatchExpressions: []v1.NodeSelectorRequirement{
			{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: options.Zones},
		}}}}}
	}
	return &v1.PersistentVolume{
		ObjectMeta: NamespacedObjectMeta(options.ObjectMeta),
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: source,
			StorageClassName:       options.StorageClassName,
			AccessModes:            []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Capacity:               v1.ResourceList{v1.ResourceStorage: resource.MustParse("100Gi")},
			NodeAffinity:           nodeAffinity,
		},
	}
}

type PersistentVolumeClaimOptions struct {
	metav1.ObjectMeta
	StorageClassName *string
	VolumeName       string
	Resources        v1.VolumeResourceRequirements
}

func PersistentVolumeClaim(overrides ...PersistentVolumeClaimOptions) *v1.PersistentVolumeClaim {
	options := PersistentVolumeClaimOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge options: %s", err))
		}
	}
	if len(options.Resources.Requests) == 0 {
		options.Resources = v1.VolumeResourceRequirements{Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("1Gi")}}
	}
	return &v1.PersistentVolumeClaim{
		ObjectMeta: NamespacedObjectMeta(options.ObjectMeta),
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: options.StorageClassName,
			VolumeName:       options.VolumeName,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.VolumeResourceRequirements{
				Limits:   options.Resources.Limits,
				Requests: options.Resources.Requests,
			},
		},
	}
}

type StorageClassOptions struct {
	metav1.ObjectMeta
	Zones             []string
	Provisioner       *string
	VolumeBindingMode *storagev1.VolumeBindingMode
}

func StorageClass(overrides ...StorageClassOptions) *storagev1.StorageClass {
	options := StorageClassOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge options: %s", err))
		}
	}

	var allowedTopologies []v1.TopologySelectorTerm
	if options.Zones != nil {
		allowedTopologies = []v1.TopologySelectorTerm{{MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{{Key: v1.LabelTopologyZone, Values: options.Zones}}}}
	}
	if options.Provisioner == nil {
		options.Provisioner = lo.ToPtr("test-provisioner")
	}

	return &storagev1.StorageClass{
		ObjectMeta:        ObjectMeta(options.ObjectMeta),
		Provisioner:       *options.Provisioner,
		AllowedTopologies: allowedTopologies,
		VolumeBindingMode: options.VolumeBindingMode,
	}
}

type VolumeAttachmentOptions struct {
	metav1.ObjectMeta
	NodeName   string
	VolumeName string
}

func VolumeAttachment(overrides ...VolumeAttachmentOptions) *storagev1.VolumeAttachment {
	options := VolumeAttachmentOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge options: %s", err))
		}
	}
	return &storagev1.VolumeAttachment{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: options.NodeName,
			Attacher: "fake-csi",
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: lo.ToPtr(options.VolumeName),
			},
		},
	}
}
