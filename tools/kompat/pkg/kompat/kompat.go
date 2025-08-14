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

package kompat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/mitchellh/go-homedir"
	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

var (
	DefaultFileName     = "k8s-compatibility.yaml"
	DefaultGithubBranch = "main"
)

type List []Kompat

type Kompat struct {
	Name          string          `yaml:"name" json:"name"`
	Compatibility []Compatibility `yaml:"compatibility" json:"compatibility"`
}

type Compatibility struct {
	AppVersion    string `yaml:"appVersion" json:"appVersion"`
	MinK8sVersion string `yaml:"minK8sVersion" json:"minK8sVersion"`
	MaxK8sVersion string `yaml:"maxK8sVersion" json:"maxK8sVersion"`
}

type Options struct {
	LastN   int
	Version string
}

func IsCompatible(filePath string, appVersion string, k8sVersion string) error {
	contents, err := readFromFile(filePath)
	if err != nil {
		return err
	}
	kompats, err := toKompats(contents)
	if err != nil {
		return err
	}
	for _, k := range kompats {
		k8sToAppVersions := k.expand()
		appVersions, ok := k8sToAppVersions[k8sVersion]
		if !ok {
			return fmt.Errorf("%s version %s is not compatible with K8s version %s", k.Name, appVersion, k8sVersion)
		}
		// check if there is any exact matches across any k8s version buckets since that signifies an override
		allAppVersions := lo.Uniq(lo.Flatten(lo.Values(k8sToAppVersions)))
		if exactMatch := lo.Contains(allAppVersions, appVersion); exactMatch {
			if ok := lo.Contains(appVersions, appVersion); !ok {
				return fmt.Errorf("%s version %s is not compatible with K8s version %s", k.Name, appVersion, k8sVersion)
			}
		} else {
			if ok := lo.ContainsBy(appVersions, func(version string) bool {
				return strings.HasPrefix(appVersion, strings.ReplaceAll(version, ".x", ""))
			}); !ok {
				return fmt.Errorf("%s version %s is not compatible with K8s version %s", k.Name, appVersion, k8sVersion)
			}
		}
	}
	return nil
}

func Parse(filePaths ...string) (List, error) {
	var kompats []Kompat
	if len(filePaths) == 0 {
		filePaths = append(filePaths, DefaultFileName)
	}
	for _, f := range filePaths {
		var contents []byte
		var err error
		url, ok := toURL(f)
		if ok {
			contents, err = readFromURL(url)
			if err != nil {
				return nil, err
			}
		} else {
			contents, err = readFromFile(f)
			if err != nil {
				return nil, err
			}
		}
		list, err := toKompats(contents)
		if err != nil {
			return nil, err
		}
		kompats = append(kompats, list...)
	}
	return kompats, nil
}

func toKompats(contents []byte) (List, error) {
	var kompats []Kompat
	decoder := yaml.NewDecoder(bytes.NewBuffer(contents))
	for {
		var kompat Kompat
		err := decoder.Decode(&kompat)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := kompat.Validate(); err != nil {
			return nil, err
		}
		kompats = append(kompats, kompat)
	}
	return kompats, nil
}

func (k Kompat) Validate() error {
	for _, c := range k.Compatibility {
		appVersion := strings.ReplaceAll(c.AppVersion, ".x", "")
		minK8sVersion := strings.ReplaceAll(c.MinK8sVersion, ".x", "")
		maxK8sVersion := strings.ReplaceAll(c.MaxK8sVersion, ".x", "")
		if _, err := semver.NewVersion(appVersion); err != nil {
			return fmt.Errorf("unable to parse compatibility for \"%s\": appVersion \"%s\" is invalid: %w", k.Name, c.AppVersion, err)
		}
		if _, err := semver.NewVersion(minK8sVersion); err != nil {
			return fmt.Errorf("unable to parse compatibility for \"%s\": minK8sVersion \"%s\" is invalid: %w", k.Name, c.MinK8sVersion, err)
		}
		if maxK8sVersion != "" {
			if _, err := semver.NewVersion(maxK8sVersion); err != nil {
				return fmt.Errorf("unable to parse compatibility for \"%s\": maxK8sVersion \"%s\" is invalid: %w", k.Name, c.MaxK8sVersion, err)
			}
		}
	}
	return nil
}

func (k Kompat) JSON() string {
	return List{k}.JSON()
}

func (k List) JSON() string {
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	enc.SetIndent("", "    ")
	if err := enc.Encode(k); err != nil {
		panic(err)
	}
	return buffer.String()
}

func (k Kompat) YAML() string {
	return List{k}.YAML()
}

func (k List) YAML() string {
	var buffer bytes.Buffer
	enc := yaml.NewEncoder(&buffer)
	if err := enc.Encode(k); err != nil {
		panic(err)
	}
	return buffer.String()
}

func (k Kompat) Markdown(_ ...Options) string {
	// options := mergeOptions(opts...)
	out := bytes.Buffer{}
	table := tablewriter.NewWriter(&out)
	headers := []string{"Kubernetes"}
	data := []string{k.Name}
	for _, c := range k.Compatibility {
		if c.MaxK8sVersion == "" || c.MinK8sVersion == c.MaxK8sVersion {
			headers = append(headers, fmt.Sprintf("\\>= `%s`", c.MinK8sVersion))
		} else {
			headers = append(headers, fmt.Sprintf("\\>= `%s` \\<= `%s`", c.MinK8sVersion, c.MaxK8sVersion))
		}
		data = append(data, c.AppVersion)
	}
	table.SetHeader(headers)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk([][]string{data})
	table.Render()
	return out.String()
}

func (k List) Markdown(opts ...Options) string {
	options := mergeOptions(opts...)
	// if len(k) == 1 {
	// 	return k[0].Markdown()
	// }
	out := bytes.Buffer{}
	table := tablewriter.NewWriter(&out)
	headers := []string{"Kubernetes"}
	var data [][]string
	// Get all k8s versions for the first row
	k8sVersions := k.k8sVersions()
	if options.Version != "" {
		version, ok := lo.Find(k8sVersions, func(version string) bool { return version == options.Version })
		if !ok {
			return ""
		}
		headers = append(headers, version)
	} else if options.LastN != 0 {
		lastN := lo.Min([]int{options.LastN, len(k8sVersions)})
		headers = append(headers, k8sVersions[len(k8sVersions)-lastN:]...)
	} else {
		headers = append(headers, k8sVersions...)
	}

	// Fill in App version rows
	for i, app := range k {
		data = append(data, []string{})
		k8sVersionToAppVersions := app.expand()
		for j, k8sVersion := range headers {
			// skip the first column since it's the text header
			if j == 0 {
				data[i] = append(data[i], app.Name)
				continue
			}

			data[i] = append(data[i], semverRange(k8sVersionToAppVersions[k8sVersion]))
		}
	}
	table.SetHeader(headers)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	return out.String()
}

func mergeOptions(opts ...Options) Options {
	if len(opts) == 0 {
		return Options{}
	}
	return opts[0]
}

func (k List) k8sVersions() []string {
	var k8sVersions []string
	for _, app := range k {
		k8sVersions = append(k8sVersions, lo.Keys(app.expand())...)
	}
	k8sVersions = lo.Uniq(k8sVersions)
	sort.Slice(k8sVersions, func(i, j int) bool {
		return lo.Must(strconv.Atoi(strings.ReplaceAll(k8sVersions[i], ".", ""))) <
			lo.Must(strconv.Atoi(strings.ReplaceAll(k8sVersions[j], ".", "")))
	})
	return k8sVersions
}

// expand returns a map of K8s version to app version, expanding out ranges to single versions
func (k Kompat) expand() map[string][]string {
	k8sToApp := map[string][]string{}
	for _, e := range k.Compatibility {
		for _, kv := range k8sVersions(e.MinK8sVersion, e.MaxK8sVersion) {
			k8sToApp[kv] = append(k8sToApp[kv], e.AppVersion)
		}
	}
	return k8sToApp
}

// Helper functions

func k8sVersions(min string, max string) []string {
	var versions []string
	major := strings.Split(min, ".")[0]
	minMinor := lo.Must(strconv.Atoi(strings.Split(min, ".")[1]))
	maxMinor := lo.Must(strconv.Atoi(strings.Split(max, ".")[1]))
	for i := minMinor; i <= maxMinor; i++ {
		versions = append(versions, fmt.Sprintf("%s.%d", major, i))
	}
	return versions
}

func toURL(str string) (string, bool) {
	isURL := false
	for _, t := range []string{".com", ".net", "http"} {
		if strings.Contains(str, t) {
			isURL = true
			break
		}
	}
	if !isURL {
		return "", false
	}
	if !strings.HasPrefix(str, "http") {
		str = fmt.Sprintf("%s%s", "https://", str)
	}
	url, err := url.Parse(str)
	if err != nil {
		return "", false
	}
	return url.String(), true
}

func readFromURL(url string) ([]byte, error) {
	if !strings.HasSuffix(url, ".yaml") {
		if strings.Contains(url, "github.com") {
			url = fmt.Sprintf("%s/%s/%s", url, DefaultGithubBranch, DefaultFileName)
			url = strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		}
	}
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func readFromFile(file string) ([]byte, error) {
	var err error
	file, err = homedir.Expand(file)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

// semverRange will sort the versions and output in a pretty range string in the format of "\>= minVersion \<= maxVersion"
// Sample output:
//
// | KUBERNETES |       1.30       |       1.31        |      1.32       | ... |
// |------------|------------------|-------------------|-----------------| ... |
// | karpenter  | \>= 0.37 \<= 1.4 | \>= 1.0.5 \<= 1.4 | \>= 1.2 \<= 1.4 | ... |
func semverRange(semvers []string) string {
	if len(semvers) == 0 {
		return ""
	}
	if len(semvers) == 1 {
		return semvers[0]
	}
	sortSemvers(semvers)
	return fmt.Sprintf("\\>= %s \\<= %s", strings.ReplaceAll(semvers[0], ".x", ""), strings.ReplaceAll(semvers[len(semvers)-1], ".x", ""))
}

func sortSemvers(semvers []string) {
	sort.Slice(semvers, func(i, j int) bool {
		return semver.MustParse(strings.ReplaceAll(semvers[i], ".x", "")).LessThan(semver.MustParse(strings.ReplaceAll(semvers[j], ".x", "")))
	})
}
