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
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Options struct {
	sourceOutput string
	urlInput     string
}

func main() {
	opts := Options{}
	flag.StringVar(&opts.urlInput, "url", "https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/master/pkg/aws/vpc/limits.go",
		"url of the raw vpc/limits.go file in the github.com/aws/amazon-vpc-resource-controller-k8s repo")
	flag.StringVar(&opts.sourceOutput, "output", "pkg/providers/instancetype/zz_generated.vpclimits.go", "output location for the generated go source file")
	flag.Parse()

	limitsURL, err := url.Parse(opts.urlInput)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create(opts.sourceOutput)
	if err != nil {
		log.Fatal(err)
	}

	client := http.Client{Timeout: time.Second * 10}
	resp, err := client.Get(limitsURL.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	newRespData := strings.Replace(string(respData), "package vpc", "package instancetype", 1)
	out.WriteString(newRespData)
	defer out.Close()

	fmt.Printf("Downloaded vpc/limits.go from \"%s\" to file \"%s\"\n", limitsURL.String(), out.Name())
}
