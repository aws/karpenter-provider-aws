# Labels Validation

function injectDomainLabelRestrictions() {
    domain=$1
	rule="self.all(x, x in [\"${domain}/instance-tenancy\", \"${domain}/capacity-reservation-type\", \"${domain}/capacity-reservation-id\", \"${domain}/ec2nodeclass\", \"${domain}/instance-encryption-in-transit-supported\", \"${domain}/instance-category\", \"${domain}/instance-hypervisor\", \"${domain}/instance-family\", \"${domain}/instance-generation\", \"${domain}/instance-local-nvme\", \"${domain}/instance-size\", \"${domain}/instance-cpu\", \"${domain}/instance-cpu-manufacturer\", \"${domain}/instance-cpu-sustained-clock-speed-mhz\", \"${domain}/instance-memory\", \"${domain}/instance-ebs-bandwidth\", \"${domain}/instance-network-bandwidth\", \"${domain}/instance-gpu-name\", \"${domain}/instance-gpu-manufacturer\", \"${domain}/instance-gpu-count\", \"${domain}/instance-gpu-memory\", \"${domain}/instance-accelerator-name\", \"${domain}/instance-accelerator-manufacturer\", \"${domain}/instance-accelerator-count\", \"${domain}/instance-capability-flex\"] || !x.find(\"^([^/]+)\").endsWith(\"${domain}\"))"
    message="label domain \"${domain}\" is restricted"
    MSG="${message}" RULE="${rule}" go tool -modfile=go.tools.mod yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.metadata.properties.labels.x-kubernetes-validations += [{"message": strenv(MSG), "rule": strenv(RULE)}]' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
}
