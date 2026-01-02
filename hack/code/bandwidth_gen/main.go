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
	"context"
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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
)

var uriSelectors = map[string]string{
	"https://docs.aws.amazon.com/ec2/latest/instancetypes/gp.html":  "#gp_network",
	"https://docs.aws.amazon.com/ec2/latest/instancetypes/co.html":  "#co_network",
	"https://docs.aws.amazon.com/ec2/latest/instancetypes/mo.html":  "#mo_network",
	"https://docs.aws.amazon.com/ec2/latest/instancetypes/so.html":  "#so_network",
	"https://docs.aws.amazon.com/ec2/latest/instancetypes/ac.html":  "#ac_network",
	"https://docs.aws.amazon.com/ec2/latest/instancetypes/hpc.html": "#hpc_network",
	"https://docs.aws.amazon.com/ec2/latest/instancetypes/pg.html":  "#pg_network",
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

	bandwidth := map[ec2types.InstanceType]int64{}
	vagueBandwidth := map[ec2types.InstanceType]string{}

	for uri, selector := range uriSelectors {
		func() {
			response := lo.Must(http.Get(uri))
			defer response.Body.Close()

			doc := lo.Must(goquery.NewDocumentFromReader(response.Body))

			// grab the table that contains the network performance values. Some instance types will have vague
			// description for bandwidth such as "Very Low", "Low", "Low to Moderate", etc. These instance types
			// will be ignored since we don't know the exact bandwidth for these instance types
			for _, row := range doc.Find(selector).NextAllFiltered(".table-container").Eq(0).Find("tbody").Find("tr").Nodes {
				instanceTypeName := strings.TrimSpace(row.FirstChild.NextSibling.FirstChild.Data)
				if !strings.ContainsAny(instanceTypeName, ".") {
					continue
				}
				bandwidthData := row.FirstChild.NextSibling.NextSibling.NextSibling.FirstChild.Data
				// exclude all rows that contain any of the following strings
				if containsAny(bandwidthData, "Low", "Moderate", "High", "Up to") {
					vagueBandwidth[ec2types.InstanceType(instanceTypeName)] = bandwidthData
					continue
				}
				bandwidthSlice := strings.Split(bandwidthData, " ")
				// if the first value contains a multiplier i.e. (4x 100 Gigabit)
				if strings.HasSuffix(bandwidthSlice[0], "x") {
					multiplier := lo.Must(strconv.ParseFloat(bandwidthSlice[0][:len(bandwidthSlice[0])-1], 64))
					bandwidth[ec2types.InstanceType(instanceTypeName)] = int64(lo.Must(strconv.ParseFloat(bandwidthSlice[1], 64)) * 1000 * multiplier)
					// Check row for instancetype for described network performance value i.e (2 Gigabit)
				} else {
					bandwidth[ec2types.InstanceType(instanceTypeName)] = int64(lo.Must(strconv.ParseFloat(bandwidthSlice[0], 64)) * 1000)
				}
			}
		}()
	}
	allInstanceTypes := getAllInstanceTypes()
	instanceTypes := lo.Keys(bandwidth)
	// 2d sort for readability
	sort.SliceStable(allInstanceTypes, func(i, j int) bool {
		return allInstanceTypes[i] < allInstanceTypes[j]
	})
	sort.SliceStable(instanceTypes, func(i, j int) bool {
		return instanceTypes[i] < instanceTypes[j]
	})
	sort.SliceStable(instanceTypes, func(i, j int) bool {
		return bandwidth[instanceTypes[i]] < bandwidth[instanceTypes[j]]
	})

	// Generate body
	var body string
	for _, instanceType := range lo.Without(allInstanceTypes, instanceTypes...) {
		if _, ok := vagueBandwidth[instanceType]; ok {
			body += fmt.Sprintf("// %s has vague bandwidth information, bandwidth is %s\n", instanceType, vagueBandwidth[instanceType])
			continue
		}
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

func getAllInstanceTypes() []ec2types.InstanceType {
	if err := os.Setenv("AWS_SDK_LOAD_CONFIG", "true"); err != nil {
		log.Fatalf("setting AWS_SDK_LOAD_CONFIG, %s", err)
	}
	if err := os.Setenv("AWS_REGION", "us-east-1"); err != nil {
		log.Fatalf("setting AWS_REGION, %s", err)
	}
	ctx := context.Background()
	cfg := lo.Must(config.LoadDefaultConfig(ctx))
	ec2api := ec2.NewFromConfig(cfg)
	var allInstanceTypes []ec2types.InstanceType

	params := &ec2.DescribeInstanceTypesInput{}
	// Retrieve the instance types in a loop using NextToken
	for {
		result := lo.Must(ec2api.DescribeInstanceTypes(ctx, params))
		allInstanceTypes = append(allInstanceTypes, lo.Map(result.InstanceTypes, func(info ec2types.InstanceTypeInfo, _ int) ec2types.InstanceType { return info.InstanceType })...)
		// Check if they are any instances left
		if result.NextToken != nil {
			params.NextToken = result.NextToken
		} else {
			break
		}
	}
	return allInstanceTypes
}
