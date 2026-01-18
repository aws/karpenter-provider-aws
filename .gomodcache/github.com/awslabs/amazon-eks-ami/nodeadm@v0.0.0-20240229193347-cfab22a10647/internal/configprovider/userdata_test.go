package configprovider

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/mail"
	"reflect"
	"strings"
	"testing"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"k8s.io/apimachinery/pkg/runtime"
)

const boundary = "#"
const completeNodeConfig = `---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: autofill
    apiServerEndpoint: autofill
    certificateAuthority: ''
    cidr: 10.100.0.0/16
  kubelet:
    config:
      port: 1010
      maxPods: 120
    flags:
      - --v=2
      - --node-labels=foo=bar,nodegroup=test
`

const partialNodeConfig = `---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  kubelet:
    config:
      maxPods: 150
      podsPerCore: 20
    flags:
      - --v=5
      - --node-labels=foo=baz
`

var completeMergedWithPartial = api.NodeConfig{
	Spec: api.NodeConfigSpec{
		Cluster: api.ClusterDetails{
			Name:                 "autofill",
			APIServerEndpoint:    "autofill",
			CertificateAuthority: []byte{},
			CIDR:                 "10.100.0.0/16",
		},
		Kubelet: api.KubeletOptions{
			Config: api.InlineDocument{
				"maxPods":     runtime.RawExtension{Raw: []byte("150")},
				"podsPerCore": runtime.RawExtension{Raw: []byte("20")},
				"port":        runtime.RawExtension{Raw: []byte("1010")},
			},
			Flags: []string{
				"--v=2",
				"--node-labels=foo=bar,nodegroup=test",
				"--v=5",
				"--node-labels=foo=baz",
			},
		},
	},
}

func indent(in string) string {
	var mid interface{}
	err := json.Unmarshal([]byte(in), &mid)
	if err != nil {
		panic(err)
	}
	out, err := json.MarshalIndent(&mid, "", "    ")
	if err != nil {
		panic(err)
	}
	return string(out)
}

func mimeifyNodeConfigs(configs ...string) string {
	var mimeDocLines = []string{
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="#"`,
	}
	for _, config := range configs {
		mimeDocLines = append(mimeDocLines, fmt.Sprintf("\n--#\nContent-Type: %s\n\n%s", nodeConfigMediaType, config))
	}
	mimeDocLines = append(mimeDocLines, "\n--#--")
	return strings.Join(mimeDocLines, "\n")
}

func TestParseMIMENodeConfig(t *testing.T) {
	mimeMessage, err := mail.ReadMessage(strings.NewReader(mimeifyNodeConfigs(completeNodeConfig)))
	if err != nil {
		t.Fatal(err)
	}
	userDataReader := multipart.NewReader(mimeMessage.Body, boundary)
	if _, err := parseMultipart(userDataReader); err != nil {
		t.Fatal(err)
	}
}

func TestGetMIMEReader(t *testing.T) {
	if _, err := getMIMEMultipartReader([]byte(mimeifyNodeConfigs(completeNodeConfig))); err != nil {
		t.Fatal(err)
	}
	if _, err := getMIMEMultipartReader([]byte(completeNodeConfig)); err == nil {
		t.Fatalf("expected err for bad multipart data")
	}
}

func TestMergeNodeConfig(t *testing.T) {
	mimeNodeConfig := mimeifyNodeConfigs(completeNodeConfig, partialNodeConfig)
	mimeMessage, err := mail.ReadMessage(strings.NewReader(mimeNodeConfig))
	if err != nil {
		t.Fatal(err)
	}
	userDataReader := multipart.NewReader(mimeMessage.Body, boundary)
	config, err := parseMultipart(userDataReader)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config, &completeMergedWithPartial) {
		t.Errorf("\nexpected: %+v\n\ngot:      %+v", &completeMergedWithPartial, config)
	}
}
