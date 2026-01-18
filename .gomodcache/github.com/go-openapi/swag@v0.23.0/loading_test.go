// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package swag

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validUsername     = "fake-user"
	validPassword     = "correct-password"
	invalidPassword   = "incorrect-password"
	sharedHeaderKey   = "X-Myapp"
	sharedHeaderValue = "MySecretKey"
)

func TestLoadFromHTTP(t *testing.T) {
	_, err := LoadFromFileOrHTTP("httx://12394:abd")
	require.Error(t, err)

	serv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	}))
	defer serv.Close()

	_, err = LoadFromFileOrHTTP(serv.URL)
	require.Error(t, err)

	ts2 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("the content"))
	}))
	defer ts2.Close()

	d, err := LoadFromFileOrHTTP(ts2.URL)
	require.NoError(t, err)
	assert.Equal(t, []byte("the content"), d)

	ts3 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if ok && u == validUsername && p == validPassword {
			rw.WriteHeader(http.StatusOK)
		} else {
			rw.WriteHeader(http.StatusForbidden)
		}
	}))
	defer ts3.Close()

	// no auth
	_, err = LoadFromFileOrHTTP(ts3.URL)
	require.Error(t, err)

	// basic auth, invalide credentials
	LoadHTTPBasicAuthUsername = validUsername
	LoadHTTPBasicAuthPassword = invalidPassword

	_, err = LoadFromFileOrHTTP(ts3.URL)
	require.Error(t, err)

	// basic auth, valid credentials
	LoadHTTPBasicAuthUsername = validUsername
	LoadHTTPBasicAuthPassword = validPassword

	_, err = LoadFromFileOrHTTP(ts3.URL)
	require.NoError(t, err)

	ts4 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		myHeaders := r.Header[sharedHeaderKey]
		ok := false
		for _, v := range myHeaders {
			if v == sharedHeaderValue {
				ok = true
				break
			}
		}
		if ok {
			rw.WriteHeader(http.StatusOK)
		} else {
			rw.WriteHeader(http.StatusForbidden)
		}
	}))
	defer ts4.Close()

	_, err = LoadFromFileOrHTTP(ts4.URL)
	require.Error(t, err)

	LoadHTTPCustomHeaders[sharedHeaderKey] = sharedHeaderValue

	_, err = LoadFromFileOrHTTP(ts4.URL)
	require.NoError(t, err)

	// clean up for future tests
	LoadHTTPBasicAuthUsername = ""
	LoadHTTPBasicAuthPassword = ""
	LoadHTTPCustomHeaders = map[string]string{}
}

func TestLoadHTTPBytes(t *testing.T) {
	_, err := LoadFromFileOrHTTP("httx://12394:abd")
	require.Error(t, err)

	serv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	}))
	defer serv.Close()

	_, err = LoadFromFileOrHTTP(serv.URL)
	require.Error(t, err)

	ts2 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("the content"))
	}))
	defer ts2.Close()

	d, err := LoadFromFileOrHTTP(ts2.URL)
	require.NoError(t, err)
	assert.Equal(t, []byte("the content"), d)
}

func TestLoadStrategy(t *testing.T) {
	loader := func(_ string) ([]byte, error) {
		return []byte(yamlPetStore), nil
	}
	remLoader := func(_ string) ([]byte, error) {
		return []byte("not it"), nil
	}

	ld := LoadStrategy("blah", loader, remLoader)
	b, _ := ld("")
	assert.Equal(t, []byte(yamlPetStore), b)

	serv := httptest.NewServer(http.HandlerFunc(yamlPestoreServer))
	defer serv.Close()

	s, err := YAMLDoc(serv.URL)
	require.NoError(t, err)
	require.NotNil(t, s)

	ts2 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
		_, _ = rw.Write([]byte("\n"))
	}))
	defer ts2.Close()
	_, err = YAMLDoc(ts2.URL)
	require.Error(t, err)
}

func TestLoadStrategyFile(t *testing.T) {
	const (
		thisIsIt    = "thisIsIt"
		thisIsNotIt = "not it"
	)

	type strategyTest struct {
		Title           string
		Path            string
		Expected        string
		ExpectedWindows string
		ExpectError     bool
	}

	t.Run("with local file strategy", func(t *testing.T) {
		loader := func(called *bool, pth *string) func(string) ([]byte, error) {
			return func(p string) ([]byte, error) {
				*called = true
				*pth = p
				return []byte(thisIsIt), nil
			}
		}

		remLoader := func(_ string) ([]byte, error) {
			return []byte(thisIsNotIt), nil
		}

		for _, toPin := range []strategyTest{
			{
				Title:           "valid fully qualified local URI, with rooted path",
				Path:            "file:///a/c/myfile.yaml",
				Expected:        "/a/c/myfile.yaml",
				ExpectedWindows: `\a\c\myfile.yaml`,
			},
			{
				Title:           "local URI with scheme, with host segment before path",
				Path:            "file://a/c/myfile.yaml",
				Expected:        "a/c/myfile.yaml",
				ExpectedWindows: `\\a\c\myfile.yaml`, // UNC host
			},
			{
				Title:           "local URI with scheme, with escaped characters",
				Path:            "file://a/c/myfile%20%28x86%29.yaml",
				Expected:        "a/c/myfile (x86).yaml",
				ExpectedWindows: `\\a\c\myfile (x86).yaml`,
			},
			{
				Title:           "local URI with scheme, rooted, with escaped characters",
				Path:            "file:///a/c/myfile%20%28x86%29.yaml",
				Expected:        "/a/c/myfile (x86).yaml",
				ExpectedWindows: `\a\c\myfile (x86).yaml`,
			},
			{
				Title:           "local URI with scheme, unescaped, with host",
				Path:            "file://a/c/myfile (x86).yaml",
				Expected:        "a/c/myfile (x86).yaml",
				ExpectedWindows: `\\a\c\myfile (x86).yaml`,
			},
			{
				Title:           "local URI with scheme, rooted, unescaped",
				Path:            "file:///a/c/myfile (x86).yaml",
				Expected:        "/a/c/myfile (x86).yaml",
				ExpectedWindows: `\a\c\myfile (x86).yaml`,
			},
			{
				Title:    "file URI with drive letter and backslashes, as a relative Windows path",
				Path:     `file://C:\a\c\myfile.yaml`,
				Expected: `C:\a\c\myfile.yaml`, // outcome on all platforms, not only windows
			},
			{
				Title:           "file URI with drive letter and backslashes, as a rooted Windows path",
				Path:            `file:///C:\a\c\myfile.yaml`,
				Expected:        `/C:\a\c\myfile.yaml`, // on non-windows, this results most likely in a wrong path
				ExpectedWindows: `C:\a\c\myfile.yaml`,  // on windows, we know that C: is a drive letter, so /C: becomes C:
			},
			{
				Title:    "file URI with escaped backslashes",
				Path:     `file://C%3A%5Ca%5Cc%5Cmyfile.yaml`,
				Expected: `C:\a\c\myfile.yaml`, // outcome on all platforms, not only windows
			},
			{
				Title:           "file URI with escaped backslashes, rooted",
				Path:            `file:///C%3A%5Ca%5Cc%5Cmyfile.yaml`,
				Expected:        `/C:\a\c\myfile.yaml`, // outcome on non-windows (most likely not a desired path)
				ExpectedWindows: `C:\a\c\myfile.yaml`,  // outcome on windows
			},
			{
				Title:           "URI with the file scheme, host omitted: relative path with extra dots",
				Path:            `file://./a/c/d/../myfile.yaml`,
				Expected:        `./a/c/d/../myfile.yaml`,
				ExpectedWindows: `a\c\myfile.yaml`, // on windows, extra processing cleans the path
			},
			{
				Title:           "relative URI without the file scheme, rooted path",
				Path:            `/a/c/myfile.yaml`,
				Expected:        `/a/c/myfile.yaml`,
				ExpectedWindows: `\a\c\myfile.yaml`, // there is no drive letter, this would probably result in a wrong path on Windows
			},
			{
				Title:           "relative URI without the file scheme, relative path",
				Path:            `a/c/myfile.yaml`,
				Expected:        `a/c/myfile.yaml`,
				ExpectedWindows: `a\c\myfile.yaml`,
			},
			{
				Title:           "relative URI without the file scheme, relative path with dots",
				Path:            `./a/c/myfile.yaml`,
				Expected:        `./a/c/myfile.yaml`,
				ExpectedWindows: `.\a\c\myfile.yaml`,
			},
			{
				Title:           "relative URI without the file scheme, relative path with extra dots",
				Path:            `./a/c/../myfile.yaml`,
				Expected:        `./a/c/../myfile.yaml`,
				ExpectedWindows: `.\a\c\..\myfile.yaml`,
			},
			{
				Title:           "relative URI without the file scheme, windows slashed-path with drive letter",
				Path:            `A:/a/c/myfile.yaml`,
				Expected:        `A:/a/c/myfile.yaml`, // on non-windows, this results most likely in a wrong path
				ExpectedWindows: `A:\a\c\myfile.yaml`, // on windows, slashes are converted
			},
			{
				Title:           "relative URI without the file scheme, windows backslashed-path with drive letter",
				Path:            `A:\a\c\myfile.yaml`,
				Expected:        `A:\a\c\myfile.yaml`, // on non-windows, this results most likely in a wrong path
				ExpectedWindows: `A:\a\c\myfile.yaml`,
			},
			{
				Title:           "URI with file scheme, host as Windows UNC name",
				Path:            `file://host/share/folder/myfile.yaml`,
				Expected:        `host/share/folder/myfile.yaml`,   // there is no host component accounted for
				ExpectedWindows: `\\host\share\folder\myfile.yaml`, // on windows, the host is interpreted as an UNC host for a file share
			},
			// TODO: invalid URI (cannot unescape/parse path)
		} {
			tc := toPin
			t.Run(tc.Title, func(t *testing.T) {
				var (
					called bool
					pth    string
				)

				ld := LoadStrategy("local", loader(&called, &pth), remLoader)
				b, err := ld(tc.Path)
				if tc.ExpectError {
					require.Error(t, err)
					assert.True(t, called)

					return
				}

				require.NoError(t, err)
				assert.True(t, called)
				assert.Equal(t, []byte(thisIsIt), b)

				if tc.ExpectedWindows != "" && runtime.GOOS == "windows" {
					assert.Equalf(t, tc.ExpectedWindows, pth,
						"expected local LoadStrategy(%q) to open: %q (windows)",
						tc.Path, tc.ExpectedWindows,
					)

					return
				}

				assert.Equalf(t, tc.Expected, pth,
					"expected local LoadStrategy(%q) to open: %q (any OS)",
					tc.Path, tc.Expected,
				)
			})
		}
	})
}
