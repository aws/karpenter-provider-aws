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

package rand

import (
	"encoding/base32"
	"math"
	"math/rand"
)

func String(length int) string {
	bufferSize := math.Ceil(float64((5*length - 4)) / float64(8))
	label := make([]byte, int(bufferSize))
	_, err := rand.Read(label) //nolint
	if err != nil {
		panic(err)
	}
	return base32.StdEncoding.EncodeToString(label)[:length]
}
