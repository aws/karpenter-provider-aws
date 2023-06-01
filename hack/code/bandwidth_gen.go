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
	"go/format"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
)

var uriSelectors = map[string]string{
	"https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/general-purpose-instances.html":       "#general-purpose-network-performance",
	"https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/compute-optimized-instances.html":     "#compute-network-performance",
	"https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/memory-optimized-instances.html":      "#memory-network-perf",
	"https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/storage-optimized-instances.html":     "#storage-network-performance",
	"https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/accelerated-computing-instances.html": "#gpu-network-performance",
}

const fileFormat = `
%s
package instancetype

// GENERATED FILE. DO NOT EDIT DIRECTLY.
// Update hack/code/bandwidth_gen.go and re-generate to edit
// You can add instance types by adding to the --instance-types CLI flag

var (
	InstanceTypeBandwidthMegabits = map[string]int64{
		%s
	}
)
`

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalf("Usage: `bandwidth_gen.go pkg/providers/instancetype/zz_generated.bandwidth.go`")
	}

	bandwidth := map[string]int64{}

	for uri, selector := range uriSelectors {
		func() {
			response := lo.Must(http.Get(uri))
			defer response.Body.Close()

			doc := lo.Must(goquery.NewDocumentFromReader(response.Body))

			// grab two tables that contain the network performance values
			// first table will contain all the instance type and bandwidth data
			// some rows will will have vague describe such as "Very Low", "Low", "Low to Moderate", etc.
			// These instance types will can be found on the second table with absolute values in Gbps
			// If the instance type is skipped on the first table it will be grabbed on the second table
			for _, row := range doc.Find(selector).NextAllFiltered(".table-container").Eq(0).Find("tbody").Find("tr").Nodes {
				instanceTypeData := row.FirstChild.NextSibling.FirstChild.FirstChild.Data
				bandwidthData := row.FirstChild.NextSibling.NextSibling.NextSibling.FirstChild.Data
				// exclude all rows that contain any of the following strings
				if containsAny(bandwidthData, "Low", "Moderate", "High", "Up to") {
					continue
				}
				bandwidthSlice := strings.Split(bandwidthData, " ")
				// if the first value contains a multiplier i.e. (4x 100 Gigabit)
				if strings.HasSuffix(bandwidthSlice[0], "x") {
					multiplier := lo.Must(strconv.ParseFloat(bandwidthSlice[0][:len(bandwidthSlice[0])-1], 64))
					bandwidth[instanceTypeData] = int64(lo.Must(strconv.ParseFloat(bandwidthSlice[1], 64)) * 1000 * multiplier)
					// Check row for instancetype for described network performance value i.e (2 Gigabit)
				} else {
					bandwidth[instanceTypeData] = int64(lo.Must(strconv.ParseFloat(bandwidthSlice[0], 64)) * 1000)
				}
			}

			// collect any remaining instancetypes
			for _, row := range doc.Find(selector).NextAllFiltered(".table-container").Eq(1).Find("tbody").Find("tr").Nodes {
				instanceTypeData := row.FirstChild.NextSibling.FirstChild.FirstChild.Data
				bandwidthData := row.FirstChild.NextSibling.NextSibling.NextSibling.FirstChild.Data
				bandwidth[instanceTypeData] = int64(lo.Must(strconv.ParseFloat(bandwidthData, 64)) * 1000)
			}
		}()
	}
	if err := os.Setenv("AWS_SDK_LOAD_CONFIG", "true"); err != nil {
		log.Fatalf("setting AWS_SDK_LOAD_CONFIG, %s", err)
	}
	if err := os.Setenv("AWS_REGION", "us-east-1"); err != nil {
		log.Fatalf("setting AWS_REGION, %s", err)
	}
	sess := session.Must(session.NewSession())
	ec2api := ec2.New(sess)
	instanceTypesOutput := lo.Must(ec2api.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{}))
	allInstanceTypes := lo.Map(instanceTypesOutput.InstanceTypes, func(info *ec2.InstanceTypeInfo, _ int) string { return *info.InstanceType })

	instanceTypes := lo.Keys(bandwidth)
	// 2d sort for readability
	sort.Strings(allInstanceTypes)
	sort.Strings(instanceTypes)
	sort.SliceStable(instanceTypes, func(i, j int) bool {
		return bandwidth[instanceTypes[i]] < bandwidth[instanceTypes[j]]
	})

	// Generate body
	var body string
	for _, instanceType := range lo.Without(allInstanceTypes, instanceTypes...) {
		body += fmt.Sprintf("// %s is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html\n", instanceType)
	}
	for _, instanceType := range instanceTypes {
		body += fmt.Sprintf("\t\"%s\": %d,\n", instanceType, bandwidth[instanceType])
	}

	license := lo.Must(os.ReadFile("hack/boilerplate.go.txt"))

	// Format and print to the file
	formatted := lo.Must(format.Source([]byte(fmt.Sprintf(fileFormat, license, body))))
	file := lo.Must(os.Create(flag.Args()[0]))
	lo.Must(file.Write(formatted))
	file.Close()
}

func containsAny(value string, excludedSubstrings ...string) bool {
	for _, str := range excludedSubstrings {
		if strings.Contains(value, str) {
			return true
		}
	}
	return false
}
