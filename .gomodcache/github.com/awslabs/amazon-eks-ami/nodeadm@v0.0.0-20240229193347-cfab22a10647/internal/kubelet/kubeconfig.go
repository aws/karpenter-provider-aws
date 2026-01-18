package kubelet

import (
	"bytes"
	_ "embed"
	"path"
	"text/template"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/util"
)

const (
	kubeconfigRoot          = "/var/lib/kubelet"
	kubeconfigFile          = "kubeconfig"
	kubeconfigBootstrapFile = "bootstrap-kubeconfig"
	kubeconfigPerm          = 0644
)

var (
	//go:embed kubeconfig.template.yaml
	kubeconfigTemplateData  string
	kubeconfigTemplate      = template.Must(template.New(kubeconfigFile).Parse(kubeconfigTemplateData))
	kubeconfigPath          = path.Join(kubeconfigRoot, kubeconfigFile)
	kubeconfigBootstrapPath = path.Join(kubeconfigRoot, kubeconfigBootstrapFile)
)

func (k *kubelet) writeKubeconfig(cfg *api.NodeConfig) error {
	kubeconfig, err := generateKubeconfig(cfg)
	if err != nil {
		return err
	}
	if enabled := cfg.Spec.Cluster.EnableOutpost; enabled != nil && *enabled {
		// kubelet bootstrap kubeconfig uses aws-iam-authenticator with cluster id to authenticate to cluster
		//   - if "aws eks describe-cluster" is bypassed, for local outpost, the value of CLUSTER_NAME parameter will be cluster id.
		//   - otherwise, the cluster id will use the id returned by "aws eks describe-cluster".
		k.flags["bootstrap-kubeconfig"] = kubeconfigBootstrapPath
		return util.WriteFileWithDir(kubeconfigBootstrapPath, kubeconfig, kubeconfigPerm)
	} else {
		k.flags["kubeconfig"] = kubeconfigPath
		return util.WriteFileWithDir(kubeconfigPath, kubeconfig, kubeconfigPerm)
	}
}

type kubeconfigTemplateVars struct {
	Cluster           string
	Region            string
	APIServerEndpoint string
	CaCertPath        string
}

func generateKubeconfig(cfg *api.NodeConfig) ([]byte, error) {
	cluster := cfg.Spec.Cluster.Name
	if enabled := cfg.Spec.Cluster.EnableOutpost; enabled != nil && *enabled {
		cluster = cfg.Spec.Cluster.ID
	}

	config := kubeconfigTemplateVars{
		Cluster:           cluster,
		Region:            cfg.Status.Instance.Region,
		APIServerEndpoint: cfg.Spec.Cluster.APIServerEndpoint,
		CaCertPath:        caCertificatePath,
	}

	var buf bytes.Buffer
	if err := kubeconfigTemplate.Execute(&buf, config); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
