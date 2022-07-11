package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type options struct {
	sourceOutput string
	urlInput     string
}

func main() {
	opts := options{}
	flag.StringVar(&opts.urlInput, "url", "https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/master/pkg/aws/vpc/limits.go",
		"url of the raw vpc/limits.go file in the github.com/aws/amazon-vpc-resource-controller-k8s repo")
	flag.StringVar(&opts.sourceOutput, "output", "pkg/cloudprovider/aws/zz_generated.vpclimits.go", "output location for the generated go source file")
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
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	newRespData := strings.Replace(string(respData), "package vpc", "package aws", 1)
	out.WriteString(newRespData)
	defer out.Close()

	fmt.Printf("Downloaded vpc/limits.go from \"%s\" to file \"%s\"\n", limitsURL.String(), out.Name())
}
