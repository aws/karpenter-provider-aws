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

package drametadata

type DRADeviceAttribute struct {
	String  *string
	Int     *int64
	Bool    *bool
	Version *string
}

type DRADevice struct {
	Attributes map[string]DRADeviceAttribute
	Capacity   map[string]string
}

type DeviceMetadata struct {
	Count   int
	Devices []DRADevice
}

func strPtr(s string) *string { return &s }
func intPtr(i int64) *int64   { return &i }
func boolPtr(b bool) *bool    { return &b }
