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

package bootstrap

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/samber/lo"
)

type compressionAlgorithm int

const (
	gzipCompression compressionAlgorithm = iota
)

// userDataMaxBytes is the maximum size in bytes allowed for EC2 user data BEFORE it is base64-encoded.
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/user-data.html
const userDataMaxBytes = 16 * 1024

// compressIfNeeded will compress the user data using the specified algorithm if it exceeds the size limit.
// An error is returned if the size limit is still breached.
func compressIfNeeded(userData []byte, alg compressionAlgorithm) ([]byte, error) {
	if len(userData) <= userDataMaxBytes {
		return userData, nil
	}
	var buf bytes.Buffer
	var w io.WriteCloser
	switch alg {
	case gzipCompression:
		w = lo.Must(gzip.NewWriterLevel(&buf, gzip.BestCompression))
	default:
		panic(fmt.Errorf("unimplemented compression algorithm, position: %d", alg))
	}
	n := lo.Must(w.Write(userData))
	if n != len(userData) {
		// TODO: panic? this should never happen
		return nil, fmt.Errorf("data written to compression writer doesn't match input (%d): %d", len(userData), n)
	}
	lo.Must0(w.Close())
	compressed := buf.Bytes()
	if len(compressed) > userDataMaxBytes {
		return nil, fmt.Errorf("compressed user data exceeds size limit (%d): %d", userDataMaxBytes, len(compressed))
	}
	return compressed, nil
}
