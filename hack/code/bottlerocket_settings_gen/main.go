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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
)

const (
	repo       = "bottlerocket-os/bottlerocket-settings-sdk"
	tagPrefix  = "bottlerocket-settings-models-v"
	sourcePath = "bottlerocket-settings-models/settings-extensions/kubernetes/src/lib.rs"
	structName = "KubernetesSettingsV1"
)

const fileFormat = `%s
package bootstrap

// GENERATED FILE. DO NOT EDIT DIRECTLY.
// Update hack/code/bottlerocket_settings_gen/main.go and re-generate to edit.
// source: https://github.com/%s/blob/%s/%s

// BottlerocketKubernetes is k8s specific configuration for bottlerocket api
type BottlerocketKubernetes struct {
%s}
`

var scalarTypes = map[string]string{
	"bool":                              "bool",
	"i32":                               "int",
	"i64":                               "int",
	"u32":                               "int",
	"NonNegativeInteger":                "int",
	"KubernetesIdsPerPodValue":          "int",
	"KubernetesMemoryThrottlingFactor":  "float64",
	"CpuManagerPolicy":                  "string",
	"DNSDomain":                         "string",
	"Identifier":                        "string",
	"IntegerPercent":                    "string",
	"IpAddr":                            "string",
	"KernelCpuSetValue":                 "string",
	"KubernetesAuthenticationMode":      "string",
	"KubernetesBootstrapToken":          "string",
	"KubernetesCPUManagerPolicyOption":  "string",
	"KubernetesCloudProvider":           "string",
	"KubernetesClusterDnsIp":            "string",
	"KubernetesClusterName":             "string",
	"KubernetesDurationValue":           "string",
	"KubernetesEvictionKey":             "string",
	"KubernetesHostnameOverrideSource":  "string",
	"KubernetesLabelKey":                "string",
	"KubernetesLabelValue":              "string",
	"KubernetesMemoryManagerPolicy":     "string",
	"KubernetesMemoryReservationPolicy": "string",
	"KubernetesMemorySwapBehavior":      "string",
	"KubernetesQuantityValue":           "string",
	"KubernetesReservedResourceKey":     "string",
	"KubernetesTaintValue":              "string",
	"KubernetesThresholdValue":          "string",
	"SingleLineString":                  "string",
	"TopologyManagerPolicy":             "string",
	"TopologyManagerScope":              "string",
	"Url":                               "string",
	"ValidBase64":                       "string",
	"ValidLinuxHostname":                "string",
}

var structTypes = map[string]string{
	"CredentialProvider":                     "BottlerocketCredentialProvider",
	"KubernetesMemoryReservation":            "BottlerocketMemoryManagerReservedMemory",
	"KubernetesTopologyManagerPolicyOptions": "BottlerocketTopologyManagerPolicyOptions",
	"StaticPod":                              "BottlerocketStaticPod",
}

type override struct {
	name      string
	goType    string
	omitEmpty *bool
}

var overrides = map[string]override{
	"api-server":             {omitEmpty: lo.ToPtr(false)},
	"cloud-provider":         {omitEmpty: lo.ToPtr(false)},
	"cluster-certificate":    {omitEmpty: lo.ToPtr(false)},
	"cluster-name":           {omitEmpty: lo.ToPtr(false)},
	"cpu-cfs-quota-enforced": {name: "CPUCFSQuota"},
	"log-level":              {name: "VerbosityLevel", goType: "*uint32"},
}

var initialisms = map[string]string{
	"api": "API", "cfs": "CFS", "cpu": "CPU", "cpus": "CPUs", "dns": "DNS", "gc": "GC",
	"id": "ID", "ip": "IP", "oom": "OOM", "qps": "QPS", "tls": "TLS",
}

type field struct {
	key, name, goType string
	omitEmpty         bool
}

func main() {
	ref := flag.String("ref", "", fmt.Sprintf("git ref of %s to read; defaults to the latest %s* tag", repo, tagPrefix))
	output := flag.String("output", "pkg/providers/amifamily/bootstrap/zz_generated.bottlerocket_kubernetes.go", "output location for the generated go source file")
	flag.Parse()

	if *ref == "" {
		*ref = lo.Must(latestTag())
	}
	src := string(lo.Must(get(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, *ref, sourcePath))))
	fields, err := parse(src)
	if err != nil {
		log.Fatalf("parsing %s@%s: %v", sourcePath, *ref, err)
	}
	if existing, err := os.ReadFile(*output); err == nil {
		fields = keepOrder(fields, string(existing))
	}

	var body strings.Builder
	for _, f := range fields {
		tag := f.key
		if f.omitEmpty {
			tag += ",omitempty"
		}
		fmt.Fprintf(&body, "\t%s %s `toml:\"%s\"`\n", f.name, f.goType, tag)
	}
	license := lo.Must(os.ReadFile("hack/boilerplate.go.txt"))
	formatted := lo.Must(format.Source(fmt.Appendf(nil, fileFormat, license, repo, *ref, sourcePath, body.String())))
	lo.Must0(os.WriteFile(*output, formatted, 0o644))
	fmt.Printf("Generated %d Bottlerocket kubernetes settings from %s@%s to %s\n", len(fields), repo, *ref, *output)
}

func get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func latestTag() (string, error) {
	data, err := get(fmt.Sprintf("https://api.github.com/repos/%s/git/matching-refs/tags/%s", repo, tagPrefix))
	if err != nil {
		return "", err
	}
	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(data, &refs); err != nil {
		return "", err
	}
	var latest []int
	for _, r := range refs {
		parts := strings.Split(strings.TrimPrefix(r.Ref, "refs/tags/"+tagPrefix), ".")
		version := lo.FilterMap(parts, func(p string, _ int) (int, bool) {
			n, err := strconv.Atoi(p)
			return n, err == nil
		})
		if len(version) == 3 && len(parts) == 3 && slices.Compare(version, latest) > 0 {
			latest = version
		}
	}
	if latest == nil {
		return "", fmt.Errorf("no %s* tags found in %s", tagPrefix, repo)
	}
	return fmt.Sprintf("%s%d.%d.%d", tagPrefix, latest[0], latest[1], latest[2]), nil
}

var tagRegex = regexp.MustCompile("`toml:\"([a-z0-9-]+)")

func keepOrder(fields []field, existing string) []field {
	order := map[string]int{}
	for i, m := range tagRegex.FindAllStringSubmatch(existing, -1) {
		order[m[1]] = i
	}
	sort.SliceStable(fields, func(i, j int) bool {
		oi, iok := order[fields[i].key]
		oj, jok := order[fields[j].key]
		if iok && jok {
			return oi < oj
		}
		return iok && !jok
	})
	return fields
}

var fieldRegex = regexp.MustCompile(`^(?:pub\s+)?([a-z0-9_]+)\s*:\s*(.+?),?$`)

func parse(src string) ([]field, error) {
	start := strings.Index(src, "pub struct "+structName+" {")
	if start == -1 {
		return nil, fmt.Errorf("struct %s not found", structName)
	}
	lines := strings.Split(src[start:], "\n")[1:]
	var fields []field
	attrDepth := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if attrDepth > 0 || strings.HasPrefix(line, "#[") {
			attrDepth += strings.Count(line, "[") - strings.Count(line, "]")
			continue
		}
		if line == "}" {
			return fields, nil
		}
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		m := fieldRegex.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("unrecognized line %q", line)
		}
		f, err := newField(m[1], m[2])
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return nil, fmt.Errorf("end of struct %s not found", structName)
}

func newField(rustName, rustType string) (field, error) {
	key := strings.ReplaceAll(rustName, "_", "-")
	o := overrides[key]
	f := field{key: key, name: lo.Ternary(o.name != "", o.name, goName(key)), goType: o.goType, omitEmpty: lo.FromPtrOr(o.omitEmpty, true)}
	if f.goType != "" {
		return f, nil
	}
	goType, err := toGoType(rustType)
	if err != nil {
		return field{}, fmt.Errorf("%s: %w; add a mapping in hack/code/bottlerocket_settings_gen/main.go", key, err)
	}
	f.goType = lo.Ternary(strings.HasPrefix(goType, "map[") || strings.HasPrefix(goType, "[]"), goType, "*"+goType)
	return f, nil
}

func toGoType(rustType string) (string, error) {
	rustType = strings.TrimSpace(rustType)
	if inner, ok := strings.CutPrefix(rustType, "Vec<"); ok {
		elem, err := toGoType(strings.TrimSuffix(inner, ">"))
		return "[]" + elem, err
	}
	if inner, ok := strings.CutPrefix(rustType, "HashMap<"); ok {
		k, v, found := strings.Cut(strings.TrimSuffix(inner, ">"), ",")
		if !found {
			return "", fmt.Errorf("unsupported map type %q", rustType)
		}
		if kt, err := toGoType(k); err != nil || kt != "string" {
			return "", fmt.Errorf("unsupported map key type %q", k)
		}
		elem, err := toGoType(v)
		return "map[string]" + elem, err
	}
	if t, ok := scalarTypes[rustType]; ok {
		return t, nil
	}
	if t, ok := structTypes[rustType]; ok {
		return t, nil
	}
	return "", fmt.Errorf("unknown Bottlerocket type %q", rustType)
}

func goName(key string) string {
	return strings.Join(lo.Map(strings.Split(key, "-"), func(part string, _ int) string {
		if v, ok := initialisms[part]; ok {
			return v
		}
		return strings.ToUpper(part[:1]) + part[1:]
	}), "")
}
