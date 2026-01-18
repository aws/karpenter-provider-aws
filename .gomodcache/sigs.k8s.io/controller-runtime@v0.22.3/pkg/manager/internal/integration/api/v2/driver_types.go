/*
Copyright 2023 The Kubernetes Authors.

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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/conversion"
	v1 "sigs.k8s.io/controller-runtime/pkg/manager/internal/integration/api/v1"
)

// Driver is a test type.
type Driver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// DriverList is a list of Drivers.
type DriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Driver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Driver{}, &DriverList{})
}

// DeepCopyInto deep copies into the given Driver.
func (d *Driver) DeepCopyInto(out *Driver) {
	*out = *d
	out.TypeMeta = d.TypeMeta
	d.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy returns a copy of Driver.
func (d *Driver) DeepCopy() *Driver {
	if d == nil {
		return nil
	}
	out := new(Driver)
	d.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a copy of Driver as runtime.Object.
func (d *Driver) DeepCopyObject() runtime.Object {
	return d.DeepCopy()
}

// DeepCopyInto deep copies into the given DriverList.
func (in *DriverList) DeepCopyInto(out *DriverList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Driver, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy returns a copy of DriverList.
func (in *DriverList) DeepCopy() *DriverList {
	if in == nil {
		return nil
	}
	out := new(DriverList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a copy of DriverList as runtime.Object.
func (in *DriverList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// ConvertTo converts Driver to the Hub version of driver.
func (d *Driver) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.Driver)
	dst.Name = d.Name
	dst.Namespace = d.Namespace
	dst.UID = d.UID
	return nil
}

// ConvertFrom converts Driver from the Hub version of driver.
func (d *Driver) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.Driver)
	d.Name = src.Name
	d.Namespace = src.Namespace
	d.UID = src.UID
	return nil
}
