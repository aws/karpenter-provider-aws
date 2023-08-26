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
	"encoding/base64"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"text/template"
)

type Custom struct {
	Options
}

type TemplateData struct {
	Taints []v1.Taint        `hash:"set"`
	Labels map[string]string `hash:"set"`
}

func (e Custom) Script() (string, error) {
	userData, err := e.templateUserData(lo.FromPtr(e.Options.CustomUserData))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString([]byte(userData)), nil
}

func (e Custom) templateUserData(rawUserData string) (string, error) {
	tmpl, err := template.New("custom").Parse(rawUserData)
	if err != nil {
		return "", err
	}

	data := TemplateData{
		Taints: e.Options.Taints,
		Labels: e.Options.Labels,
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
