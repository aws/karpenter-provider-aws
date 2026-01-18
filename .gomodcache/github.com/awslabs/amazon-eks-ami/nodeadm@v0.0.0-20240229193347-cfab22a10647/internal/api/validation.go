package api

import "fmt"

func ValidateNodeConfig(cfg *NodeConfig) error {
	if cfg.Spec.Cluster.Name == "" {
		return fmt.Errorf("Name is missing in cluster configuration")
	}
	if cfg.Spec.Cluster.APIServerEndpoint == "" {
		return fmt.Errorf("Apiserver endpoint is missing in cluster configuration")
	}
	if cfg.Spec.Cluster.CertificateAuthority == nil {
		return fmt.Errorf("Certificate authority is missing in cluster configuration")
	}
	if cfg.Spec.Cluster.CIDR == "" {
		return fmt.Errorf("CIDR is missing in cluster configuration")
	}
	if enabled := cfg.Spec.Cluster.EnableOutpost; enabled != nil && *enabled {
		if cfg.Spec.Cluster.ID == "" {
			return fmt.Errorf("CIDR is missing in cluster configuration")
		}
	}
	return nil
}
