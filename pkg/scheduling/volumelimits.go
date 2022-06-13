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

package scheduling

import (
	"context"
	"fmt"

	"knative.dev/pkg/logging"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VolumeLimits tracks volume limits on a per node basis.  The number of volumes that can be mounted varies by instance
// type. We need to be aware and track the mounted volume usage to inform our awareness of which pods can schedule to
// which nodes.
type VolumeLimits struct {
	volumes    volumeUsage
	podVolumes map[types.NamespacedName]volumeUsage
	kubeClient client.Client
}

type volumeUsage map[string]sets.String

func (u volumeUsage) Add(provisioner string, pvcID string) {
	existing, ok := u[provisioner]
	if !ok {
		existing = sets.NewString()
		u[provisioner] = existing
	}
	existing.Insert(pvcID)
}

func (u volumeUsage) union(volumes volumeUsage) volumeUsage {
	cp := volumeUsage{}
	for k, v := range u {
		cp[k] = sets.NewString(v.List()...)
	}
	for k, v := range volumes {
		existing, ok := cp[k]
		if !ok {
			existing = sets.NewString()
			cp[k] = existing
		}
		existing.Insert(v.List()...)
	}
	return cp
}

func (u volumeUsage) insert(volumes volumeUsage) {
	for k, v := range volumes {
		existing, ok := u[k]
		if !ok {
			existing = sets.NewString()
			u[k] = existing
		}
		existing.Insert(v.List()...)
	}
}

func (u volumeUsage) copy() volumeUsage {
	cp := volumeUsage{}
	for k, v := range u {
		cp[k] = sets.NewString(v.List()...)
	}
	return cp
}

func NewVolumeLimits(kubeClient client.Client) *VolumeLimits {
	return &VolumeLimits{
		kubeClient: kubeClient,
		volumes:    volumeUsage{},
		podVolumes: map[types.NamespacedName]volumeUsage{},
	}
}

func (v *VolumeLimits) Add(ctx context.Context, pod *v1.Pod) {
	podVolumes, err := v.validate(ctx, pod)
	if err != nil {
		logging.FromContext(ctx).Errorf("inconsistent state error adding volume, %s, please file an issue", err)
	}
	v.podVolumes[client.ObjectKeyFromObject(pod)] = podVolumes
	v.volumes = v.volumes.union(podVolumes)
}

type VolumeCount map[string]int

// Exceeds returns true if the volume count exceeds the limits provided.  If there is no value for a storage provider, it
// is treated as unlimited.
func (c VolumeCount) Exceeds(limits VolumeCount) bool {
	for k, v := range c {
		limit, hasLimit := limits[k]
		if !hasLimit {
			continue
		}
		if v > limit {
			return true
		}
	}
	return false
}

// Fits returns true if the rhs 'fits' within the volume count.
func (c VolumeCount) Fits(rhs VolumeCount) bool {
	for k, v := range rhs {
		limit, hasLimit := c[k]
		if !hasLimit {
			continue
		}
		if v > limit {
			return false
		}
	}
	return true
}

func (v *VolumeLimits) Validate(ctx context.Context, pod *v1.Pod) (VolumeCount, error) {
	podVolumes, err := v.validate(ctx, pod)
	if err != nil {
		return nil, err
	}
	result := VolumeCount{}
	for k, v := range v.volumes.union(podVolumes) {
		result[k] += len(v)
	}
	return result, nil
}

func (v *VolumeLimits) validate(ctx context.Context, pod *v1.Pod) (volumeUsage, error) {
	podPVCs := volumeUsage{}

	for _, volume := range pod.Spec.Volumes {
		var pvcID string
		var storageClassName *string
		if volume.PersistentVolumeClaim != nil {
			var pvc v1.PersistentVolumeClaim
			if err := v.kubeClient.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: volume.PersistentVolumeClaim.ClaimName}, &pvc); err != nil {
				return nil, err
			}

			pvcID = fmt.Sprintf("%s/%s", pod.Namespace, volume.PersistentVolumeClaim.ClaimName)
			storageClassName = pvc.Spec.StorageClassName
		} else if volume.Ephemeral != nil {
			// generated name per https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#persistentvolumeclaim-naming
			pvcID = fmt.Sprintf("%s/%s-%s", pod.Namespace, pod.Name, volume.Name)
			storageClassName = volume.Ephemeral.VolumeClaimTemplate.Spec.StorageClassName
		} else {
			continue
		}

		provisioner := "unspecified"
		if storageClassName != nil {
			var sc storagev1.StorageClass
			if err := v.kubeClient.Get(ctx, client.ObjectKey{Name: *storageClassName}, &sc); err != nil {
				return nil, err
			}
			provisioner = sc.Provisioner
		}
		podPVCs.Add(provisioner, pvcID)
	}
	return podPVCs, nil
}

func (v *VolumeLimits) DeletePod(key types.NamespacedName) {
	delete(v.podVolumes, key)
	// volume names could be duplicated, so we re-create our volumes
	v.volumes = volumeUsage{}
	for _, c := range v.podVolumes {
		v.volumes.insert(c)
	}
}

func (v *VolumeLimits) Copy() *VolumeLimits {
	cp := &VolumeLimits{
		kubeClient: v.kubeClient,
		volumes:    v.volumes.copy(),
		podVolumes: map[types.NamespacedName]volumeUsage{},
	}
	for k, v := range v.podVolumes {
		cp.podVolumes[k] = v.copy()
	}
	return cp
}
