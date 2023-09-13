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

package fake

import (
	"net/http"
	"net/http/httptest"
)

type HTTPClient struct {
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{}
}

func (f *HTTPClient) Get(_ string) (*http.Response, error) {
	json := `{"gitVersion": "v0.1.2", "message": "success"}`
	recorder := httptest.NewRecorder()
	recorder.Header().Add("Content-Type", "application/json")
	_, _ = recorder.WriteString(json)
	return recorder.Result(), nil
}
